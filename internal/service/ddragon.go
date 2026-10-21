package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chyoumu114/lol-civil-war/internal/config"
	"github.com/chyoumu114/lol-civil-war/internal/domain/champion"
)

// DDragon 是 DataDragon 英雄数据客户端（带进程内缓存）。
//
// DataDragon 不可达时降级为内置英雄资料库，保证离线部署也能正常显示头像版本。
type DDragon struct {
	client *http.Client

	mu       sync.Mutex
	versions map[string]*ddragonCache
	latest   *ddragonCache
}

type ddragonCache struct {
	expires time.Time
	data    *ddragonData
}

type ddragonData struct {
	Version   string             `json:"version"`
	Champions []ChampionNameView `json:"champions"`
}

// NewDDragon 构造客户端。
func NewDDragon(cfg *config.Config) *DDragon {
	return &DDragon{
		client:   &http.Client{Timeout: 10 * time.Second},
		versions: map[string]*ddragonCache{},
	}
}

// Names 返回英雄名列表；失败时回退本地资料库。
func (d *DDragon) Names(ctx context.Context, requested string) ChampionNamesView {
	version := normalizeDDragonVersion(requested)
	latest := d.latestRealVersion(ctx)

	candidates := make([]string, 0, 3)
	seen := map[string]bool{}
	for _, candidate := range []string{version, ddragonPatchOf(version), latest} {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		candidates = append(candidates, candidate)
	}

	for _, candidate := range candidates {
		if data := d.fetchChampions(ctx, candidate); data != nil {
			latestVersion := latest
			if latestVersion == "" {
				latestVersion = "latest"
			}
			return ChampionNamesView{
				Version:       data.Version,
				Champions:     data.Champions,
				LatestVersion: latestVersion,
			}
		}
	}

	// 离线兜底：用内置资料库生成英雄表。
	if lib, err := champion.Load(); err == nil {
		champions := make([]ChampionNameView, 0, len(lib.Champions))
		for _, item := range lib.Champions {
			champions = append(champions, ChampionNameView{
				ID: item.Key, Key: item.Key, Name: item.CN, Title: item.Title,
			})
		}
		libVersion := lib.Version
		if libVersion == "" {
			libVersion = "latest"
		}
		if latest == "" {
			latest = libVersion
		}
		return ChampionNamesView{Version: libVersion, Champions: champions, LatestVersion: latest}
	}

	return ChampionNamesView{Version: "latest", Champions: []ChampionNameView{}, LatestVersion: "latest"}
}

func (d *DDragon) fetchChampions(ctx context.Context, version string) *ddragonData {
	now := time.Now()
	d.mu.Lock()
	if cached, ok := d.versions[version]; ok && now.Before(cached.expires) {
		data := cached.data
		d.mu.Unlock()
		return data
	}
	d.mu.Unlock()

	url := fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/%s/data/zh_CN/champion.json", version)
	body, err := d.getWithRetry(ctx, url)
	if err != nil {
		return nil
	}
	var payload struct {
		Version string `json:"version"`
		Data    map[string]struct {
			ID    string `json:"id"`
			Key   string `json:"key"`
			Name  string `json:"name"`
			Title string `json:"title"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || len(payload.Data) == 0 {
		return nil
	}
	champions := make([]ChampionNameView, 0, len(payload.Data))
	for _, item := range payload.Data {
		champions = append(champions, ChampionNameView{
			ID: item.ID, Key: item.Key, Name: item.Name, Title: item.Title,
		})
	}
	data := &ddragonData{Version: payload.Version, Champions: champions}
	if data.Version == "" {
		data.Version = version
	}

	d.mu.Lock()
	d.versions[version] = &ddragonCache{expires: now.Add(24 * time.Hour), data: data}
	d.mu.Unlock()
	return data
}

func (d *DDragon) latestRealVersion(ctx context.Context) string {
	now := time.Now()
	d.mu.Lock()
	if d.latest != nil && now.Before(d.latest.expires) {
		data := d.latest.data
		d.mu.Unlock()
		if data != nil {
			return data.Version
		}
		return ""
	}
	d.mu.Unlock()

	body, err := d.getWithRetry(ctx, "https://ddragon.leagueoflegends.com/api/versions.json")
	if err != nil {
		return ""
	}
	var versions []string
	if err := json.Unmarshal(body, &versions); err != nil || len(versions) == 0 {
		return ""
	}
	version := strings.TrimSpace(versions[0])
	d.mu.Lock()
	d.latest = &ddragonCache{expires: now.Add(time.Hour), data: &ddragonData{Version: version}}
	d.mu.Unlock()
	return version
}

func (d *DDragon) getWithRetry(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := d.client.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			_ = resp.Body.Close()
			if readErr == nil && resp.StatusCode == http.StatusOK {
				return body, nil
			}
			if readErr != nil {
				lastErr = readErr
			} else {
				lastErr = fmt.Errorf("DataDragon 返回 %d", resp.StatusCode)
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return nil, lastErr
}

var ddragonVersionRe = regexp.MustCompile(`^(\d{1,3})\.(\d{1,3})`)

// normalizeDDragonVersion 把任意版本串归一化为 x.y.z。
func normalizeDDragonVersion(version string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(version) {
		if (r >= '0' && r <= '9') || r == '.' {
			b.WriteRune(r)
		}
	}
	clean := strings.Trim(b.String(), ".")
	parts := strings.Split(clean, ".")
	if len(parts) >= 2 && isDigits(parts[0]) && isDigits(parts[1]) {
		patch := "1"
		if len(parts) == 3 && isDigits(parts[2]) {
			patch = parts[2]
		}
		return parts[0] + "." + parts[1] + "." + patch
	}
	return "latest"
}

// ddragonPatchOf 从 x.y.z 生成 x.y.1（旧接口的兜底候选）。
func ddragonPatchOf(version string) string {
	m := ddragonVersionRe.FindStringSubmatch(version)
	if m == nil {
		return ""
	}
	if strings.Count(version, ".") == 2 {
		return m[1] + "." + m[2] + ".1"
	}
	return ""
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	_, err := strconv.Atoi(value)
	return err == nil
}
