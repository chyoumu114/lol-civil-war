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

const (
	// 版本缓存上限：只保留最近使用的少量版本，避免匿名枚举撑爆内存（M-06）。
	ddragonMaxEntries  = 32
	ddragonPositiveTTL = 24 * time.Hour
	ddragonNegativeTTL = 5 * time.Minute
	ddragonLatestTTL   = time.Hour
)

// DDragon 是 DataDragon 英雄数据客户端（带进程内缓存）。
//
// 安全约定（M-06）：
//   - 只允许“配置版本 / latest / 已缓存版本”触发网络请求，其余版本直接 422；
//   - 缓存容量有上限，并带负缓存，避免反复探测上游；
//   - 同一键的并发请求合并为一次出站请求。
//
// DataDragon 不可达时降级为内置英雄资料库，保证离线部署也能正常显示头像版本。
type DDragon struct {
	client            *http.Client
	configuredVersion string

	mu       sync.Mutex
	versions map[string]*ddragonCache
	latest   *ddragonCache
	calls    map[string]*ddragonCall
}

type ddragonCache struct {
	expires time.Time
	stored  time.Time
	data    *ddragonData
}

type ddragonCall struct {
	mu sync.Mutex
}

type ddragonData struct {
	Version   string             `json:"version"`
	Champions []ChampionNameView `json:"champions"`
}

// NewDDragon 构造客户端。
func NewDDragon(cfg *config.Config) *DDragon {
	configured := normalizeDDragonVersion(cfg.DDragonVersion)
	if configured == "latest" {
		configured = ""
	}
	return &DDragon{
		client:            &http.Client{Timeout: 10 * time.Second},
		configuredVersion: configured,
		versions:          map[string]*ddragonCache{},
		calls:             map[string]*ddragonCall{},
	}
}

// Allowed 报告请求的版本是否可以触发上游请求。
func (d *DDragon) Allowed(ctx context.Context, requested string) bool {
	requested = strings.TrimSpace(requested)
	if requested == "" || strings.EqualFold(requested, "latest") {
		return true
	}
	version := normalizeDDragonVersion(requested)
	if version == "latest" {
		return true
	}
	if d.configuredVersion != "" {
		return version == d.configuredVersion
	}
	if d.hasCached(version) {
		return true
	}
	latest := d.latestRealVersion(ctx)
	if latest == "" {
		return false
	}
	return version == latest || version == ddragonPatchOf(latest)
}

// Names 返回英雄名列表；失败时回退本地资料库。
func (d *DDragon) Names(ctx context.Context, requested string) ChampionNamesView {
	requested = strings.TrimSpace(requested)
	version := normalizeDDragonVersion(requested)
	latest := d.configuredVersion
	if latest == "" {
		latest = d.latestRealVersion(ctx)
	}
	// 配置了固定 DataDragon 版本时，latest 也使用该版本，保证 CDN 与本地缓存一致。
	if d.configuredVersion != "" && (requested == "" || requested == "latest") {
		version = d.configuredVersion
	}

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

func (d *DDragon) hasCached(version string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	cached, ok := d.versions[version]
	return ok && cached.data != nil
}

// peek 读取未过期的缓存；ok=true 且 data=nil 表示命中负缓存。
func (d *DDragon) peek(version string, now time.Time) (*ddragonData, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	cached, ok := d.versions[version]
	if !ok || !now.Before(cached.expires) {
		return nil, false
	}
	return cached.data, true
}

func (d *DDragon) fetchChampions(ctx context.Context, version string) *ddragonData {
	now := time.Now()
	if data, ok := d.peek(version, now); ok {
		return data
	}

	call := d.callFor("champions:" + version)
	call.mu.Lock()
	defer call.mu.Unlock()
	// 双重检查：等待期间可能已有其他请求填充缓存。
	if data, ok := d.peek(version, now); ok {
		return data
	}

	url := fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/%s/data/zh_CN/champion.json", version)
	body, err := d.getWithRetry(ctx, url)
	if err != nil {
		// 客户端主动取消不应污染负缓存（否则取消请求就能让所有人 5 分钟内拿不到数据）。
		if ctx.Err() == nil {
			d.store(version, &ddragonCache{expires: now.Add(ddragonNegativeTTL), stored: now})
		}
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
		d.store(version, &ddragonCache{expires: now.Add(ddragonNegativeTTL), stored: now})
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
	d.store(version, &ddragonCache{expires: now.Add(ddragonPositiveTTL), stored: now, data: data})
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

	call := d.callFor("latest")
	call.mu.Lock()
	defer call.mu.Unlock()
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
		if ctx.Err() == nil {
			d.mu.Lock()
			d.latest = &ddragonCache{expires: now.Add(ddragonNegativeTTL), stored: now}
			d.mu.Unlock()
		}
		return ""
	}
	var versions []string
	if err := json.Unmarshal(body, &versions); err != nil || len(versions) == 0 {
		d.mu.Lock()
		d.latest = &ddragonCache{expires: now.Add(ddragonNegativeTTL), stored: now}
		d.mu.Unlock()
		return ""
	}
	version := strings.TrimSpace(versions[0])
	d.mu.Lock()
	d.latest = &ddragonCache{
		expires: now.Add(ddragonLatestTTL),
		stored:  now,
		data:    &ddragonData{Version: version},
	}
	d.mu.Unlock()
	return version
}

// store 写入缓存并维持容量上限。
func (d *DDragon) store(version string, cached *ddragonCache) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.versions[version] = cached
	if len(d.versions) <= ddragonMaxEntries {
		return
	}
	now := time.Now()
	for key, item := range d.versions {
		if !now.Before(item.expires) {
			delete(d.versions, key)
		}
	}
	for len(d.versions) > ddragonMaxEntries {
		oldestKey := ""
		var oldest time.Time
		for key, item := range d.versions {
			if oldestKey == "" || item.stored.Before(oldest) {
				oldestKey, oldest = key, item.stored
			}
		}
		delete(d.versions, oldestKey)
	}
	// calls 只服务白名单版本；随 versions 一起清理，避免未来放宽闸门后无界增长（N-7.2）。
	for key := range d.calls {
		if key == "latest" {
			continue
		}
		if _, ok := d.versions[strings.TrimPrefix(key, "champions:")]; !ok {
			delete(d.calls, key)
		}
	}
}

// callFor 返回某个键的并发合并锁（singleflight 的简化实现，不引入额外依赖）。
func (d *DDragon) callFor(key string) *ddragonCall {
	d.mu.Lock()
	defer d.mu.Unlock()
	call, ok := d.calls[key]
	if !ok {
		call = &ddragonCall{}
		d.calls[key] = call
	}
	return call
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
