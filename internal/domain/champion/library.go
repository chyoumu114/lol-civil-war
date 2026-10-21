// Package champion 提供英雄资料库（本地静态数据）的加载与搜索。
//
// 数据来自 DataDragon 的英雄列表，附带中文名、称号、位置与外号；离线可用。
// 搜索规则：空格分隔多个关键词，需全部命中；命中字段优先级为
// 外号 → 称号 → 中文名 → 英文名 → 位置关键词。
package champion

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/chyoumu114/lol-civil-war/internal/domain/rank"
)

// Champion 是资料库中的一名英雄。
type Champion struct {
	Key       string   `json:"key"`
	EN        string   `json:"en"`
	CN        string   `json:"cn"`
	Title     string   `json:"title"`
	Positions []string `json:"positions"`
	Aliases   []string `json:"aliases"`
}

// Hit 表示某个关键词命中的字段。
type Hit struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

// Item 是搜索结果中的一名英雄（附带命中信息）。
type Item struct {
	Key       string   `json:"key"`
	EN        string   `json:"en"`
	CN        string   `json:"cn"`
	Title     string   `json:"title"`
	Positions []string `json:"positions"`
	Aliases   []string `json:"aliases"`
	Matched   []Hit    `json:"matched"`
}

// Result 是 /api/champions/library 的响应。
type Result struct {
	Version   string            `json:"version"`
	UpdatedAt string            `json:"updatedAt"`
	Positions map[string]string `json:"positions"`
	Total     int               `json:"total"`
	Count     int               `json:"count"`
	Query     string            `json:"query"`
	Position  string            `json:"position"`
	Items     []Item            `json:"items"`
}

type rawLibrary struct {
	Version   string     `json:"version"`
	UpdatedAt string     `json:"updatedAt"`
	Champions []Champion `json:"champions"`
}

var (
	loadOnce sync.Once
	loaded   *Library
	loadErr  error
)

// Library 是已加载的英雄资料库。
type Library struct {
	Version   string
	UpdatedAt string
	Champions []Champion
}

// Load 返回内置英雄资料库（进程内只解析一次）。
func Load() (*Library, error) {
	loadOnce.Do(func() {
		var raw rawLibrary
		if err := json.Unmarshal(embeddedLibrary, &raw); err != nil {
			loadErr = fmt.Errorf("英雄资料库数据损坏: %w", err)
			return
		}
		if len(raw.Champions) == 0 {
			loadErr = fmt.Errorf("英雄资料库数据为空")
			return
		}
		loaded = &Library{Version: raw.Version, UpdatedAt: raw.UpdatedAt, Champions: raw.Champions}
	})
	return loaded, loadErr
}

var positionKeywords = map[rank.Position][]string{
	rank.PositionTop:     {"TOP", "上单", "上路"},
	rank.PositionJungle:  {"JUNGLE", "JG", "打野", "野区"},
	rank.PositionMiddle:  {"MIDDLE", "MID", "中单", "中路"},
	rank.PositionBottom:  {"BOTTOM", "BOT", "ADC", "下路", "射手"},
	rank.PositionUtility: {"UTILITY", "SUPPORT", "SUP", "辅助"},
}

// Search 按关键词与位置筛选英雄。
func Search(query string, position *rank.Position) (Result, error) {
	lib, err := Load()
	if err != nil {
		return Result{}, err
	}

	query = strings.TrimSpace(query)
	tokens := strings.Fields(strings.ToLower(query))

	items := make([]Item, 0, len(lib.Champions))
	for _, champ := range lib.Champions {
		if position != nil && !containsString(champ.Positions, string(*position)) {
			continue
		}
		matched := make([]Hit, 0, len(tokens))
		seen := map[string]struct{}{}
		ok := true
		for _, token := range tokens {
			hit, found := matchToken(champ, token)
			if !found {
				ok = false
				break
			}
			key := hit.Field + ":" + hit.Value
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			matched = append(matched, hit)
		}
		if !ok {
			continue
		}
		items = append(items, Item{
			Key:       champ.Key,
			EN:        champ.EN,
			CN:        champ.CN,
			Title:     champ.Title,
			Positions: nonNilStrings(champ.Positions),
			Aliases:   nonNilStrings(champ.Aliases),
			Matched:   matched,
		})
	}

	positionCode := "ALL"
	if position != nil {
		positionCode = string(*position)
	}

	return Result{
		Version:   lib.Version,
		UpdatedAt: lib.UpdatedAt,
		Positions: positionLabels(),
		Total:     len(lib.Champions),
		Count:     len(items),
		Query:     query,
		Position:  positionCode,
		Items:     items,
	}, nil
}

// Names 返回英雄的「key → 中文名」映射，供 DataDragon 不可用时兜底。
func Names() map[string]string {
	lib, err := Load()
	if err != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(lib.Champions))
	for _, champ := range lib.Champions {
		out[champ.Key] = champ.CN
	}
	return out
}

// Version 返回资料库版本（如 16.20.1）。
func Version() string {
	lib, err := Load()
	if err != nil {
		return ""
	}
	return lib.Version
}

func positionLabels() map[string]string {
	out := make(map[string]string, len(rank.Positions))
	for _, pos := range rank.Positions {
		out[string(pos)] = pos.Label()
	}
	return out
}

func matchToken(champ Champion, token string) (Hit, bool) {
	for _, alias := range champ.Aliases {
		if containsFold(alias, token) {
			return Hit{Field: "alias", Value: alias}, true
		}
	}
	fields := []struct {
		field string
		value string
	}{
		{"title", champ.Title},
		{"cn", champ.CN},
		{"en", champ.EN},
		{"en", champ.Key},
	}
	for _, f := range fields {
		if f.value != "" && containsFold(f.value, token) {
			return Hit{Field: f.field, Value: f.value}, true
		}
	}
	for _, pos := range rank.Positions {
		if !containsString(champ.Positions, string(pos)) {
			continue
		}
		for _, keyword := range positionKeywords[pos] {
			if containsFold(keyword, token) {
				return Hit{Field: "position", Value: pos.Label()}, true
			}
		}
	}
	return Hit{}, false
}

func containsFold(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	return strings.Contains(strings.ToLower(haystack), needle)
}

func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}
