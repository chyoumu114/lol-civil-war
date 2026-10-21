package rofl

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	sigRofl1 = "RIOT\x00\x00"
	sigRofl2 = "RIOT\x02\x00"

	// maxMetadataBytes 防御异常文件。
	maxMetadataBytes = 32 * 1024 * 1024
)

// 位置代号规范化映射（兼容官方与历史写法）。
var positionAliases = map[string]string{
	"TOP": "TOP", "JUNGLE": "JUNGLE", "MIDDLE": "MIDDLE", "MID": "MIDDLE",
	"BOTTOM": "BOTTOM", "BOT": "BOTTOM", "ADC": "BOTTOM",
	"UTILITY": "UTILITY", "SUPPORT": "UTILITY", "SUP": "UTILITY",
}

var (
	versionRe = regexp.MustCompile(`^(\d{1,3})\.(\d{1,3})`)
	anyVerRe  = regexp.MustCompile(`\d+\.\d+`)
)

// Parser 解析 .rofl 文件。
type Parser struct{}

// ParseFile 读取并解析录像文件。
func (Parser) ParseFile(path string) (Metadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return Metadata{}, parseErr("文件不存在或不可读")
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return Metadata{}, parseErr("无法读取文件信息")
	}
	size := info.Size()

	sig := readAt(f, 0, 6)
	if len(sig) < 6 || string(sig[:4]) != "RIOT" {
		return Metadata{}, parseErr("不是有效的 ROFL 文件（缺少 RIOT 签名）")
	}

	var (
		kind        string
		gameVersion string
		metaJSON    []byte
	)
	switch string(sig) {
	case sigRofl2:
		kind = "ROFL2"
		gameVersion, metaJSON, err = readRofl2(f, size)
	case sigRofl1:
		kind = "ROFL"
		gameVersion, metaJSON, err = readRofl1(f)
	default:
		return Metadata{}, parseErr("未知的 ROFL 文件版本，当前仅支持 14.11+ 的新格式（ROFL2）")
	}
	if err != nil {
		return Metadata{}, err
	}

	var metadata map[string]any
	if err := json.Unmarshal(metaJSON, &metadata); err != nil {
		return Metadata{}, parseErr("录像元数据不是合法的 JSON，文件可能已损坏")
	}

	if gameVersion == "" {
		gameVersion = stringOf(metadata["gameVersion"])
	}

	stats, err := decodeStats(metadata["statsJson"])
	if err != nil {
		return Metadata{}, err
	}

	players := make([]Player, 0, len(stats))
	slotByTeam := map[string]int{"blue": 0, "red": 0, "unknown": 0}
	for _, raw := range stats {
		team := "unknown"
		switch key := stringOf(raw["TEAM"]); key {
		case "100":
			team = "blue"
		case "200":
			team = "red"
		}
		slotByTeam[team]++

		players = append(players, Player{
			Team:              team,
			Slot:              slotByTeam[team],
			Champion:          stringOf(raw["SKIN"]),
			PlayerName:        resolvePlayerName(raw),
			PUUID:             stringOf(raw["PUUID"]),
			Position:          normalizePosition(raw),
			Kills:             toInt(raw["CHAMPIONS_KILLED"]),
			Deaths:            toInt(raw["NUM_DEATHS"]),
			Assists:           toInt(raw["ASSISTS"]),
			CS:                toInt(raw["MINIONS_KILLED"]) + toInt(raw["NEUTRAL_MINIONS_KILLED"]),
			Gold:              toInt(raw["GOLD_EARNED"]),
			DamageToChampions: toInt(raw["TOTAL_DAMAGE_DEALT_TO_CHAMPIONS"]),
			DamageTaken:       toInt(raw["TOTAL_DAMAGE_TAKEN"]),
			VisionScore:       toInt(raw["VISION_SCORE"]),
			Win:               toBool(raw["WIN"]),
			Raw:               raw,
		})
	}

	var blueWin *bool
	for _, player := range players {
		if player.Team == "blue" {
			w := player.Win
			blueWin = &w
			break
		}
	}

	return Metadata{
		Type:           kind,
		GameVersion:    gameVersion,
		DDragonVersion: ddragonVersion(gameVersion),
		GameLengthMS:   toInt(metadata["gameLength"]),
		BlueWin:        blueWin,
		Players:        players,
		Bans:           extractBans(metadata, stats),
	}, nil
}

// readRofl2 读取 ROFL2：末尾 4 字节为元数据长度，元数据紧挨在其前。
func readRofl2(f *os.File, size int64) (string, []byte, error) {
	if size < 512 {
		return "", nil, parseErr("文件过小，不是完整的 ROFL 录像")
	}
	gameVersion := readVersionString(f, 14)

	lengthBytes := readAt(f, size-4, 4)
	if len(lengthBytes) != 4 {
		return "", nil, parseErr("读取元数据长度失败，文件可能已损坏")
	}
	metaLen := int64(binary.LittleEndian.Uint32(lengthBytes))
	if metaLen <= 0 || metaLen > maxMetadataBytes || metaLen > size-8 {
		return "", nil, parseErr("元数据区长度异常，文件可能已损坏或不是完整录像")
	}
	meta := readAt(f, size-4-metaLen, int(metaLen))
	if int64(len(meta)) != metaLen {
		return "", nil, parseErr("读取元数据失败，文件可能已损坏")
	}
	return gameVersion, meta, nil
}

// readRofl1 读取旧格式：长度字段从偏移 262 开始。
func readRofl1(f *os.File) (string, []byte, error) {
	fields := readAt(f, 262, 26)
	if len(fields) < 26 {
		return "", nil, parseErr("文件头不完整，不是有效的 ROFL 录像")
	}
	metaOffset := int64(binary.LittleEndian.Uint32(fields[6:10]))
	metaLen := int64(binary.LittleEndian.Uint32(fields[10:14]))

	if metaOffset <= 0 {
		metaOffset = 288
	}
	if metaLen <= 0 || metaLen > maxMetadataBytes {
		return "", nil, parseErr("元数据区长度异常，文件可能已损坏")
	}
	meta := readAt(f, metaOffset, int(metaLen))
	if int64(len(meta)) != metaLen {
		return "", nil, parseErr("读取元数据失败，文件可能已损坏")
	}
	return "", meta, nil
}

// decodeStats 解析 statsJson：可能是 JSON 字符串，也可能已经是数组。
func decodeStats(value any) ([]map[string]any, error) {
	var stats []map[string]any
	switch v := value.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &stats); err != nil {
			return nil, parseErr("录像元数据中的玩家数据不是合法 JSON，文件可能已损坏")
		}
	case []any:
		stats = make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				stats = append(stats, m)
			}
		}
	default:
		return nil, parseErr("该录像不包含玩家数据（14.9 – 14.10 版本的录像没有元数据，无法解析）")
	}
	if len(stats) == 0 {
		return nil, parseErr("该录像不包含玩家数据（14.9 – 14.10 版本的录像没有元数据，无法解析）")
	}
	return stats, nil
}

// extractBans 扫描元数据中可能存在的 Ban 字段。
// ROFL 录像从游戏加载时开始记录，Ban/Pick 不写入录像，因此通常为空。
func extractBans(metadata map[string]any, stats []map[string]any) map[string]any {
	found := map[string]any{}

	metaFound := map[string]any{}
	for key, value := range metadata {
		if _, ok := value.([]any); ok && strings.Contains(strings.ToLower(key), "ban") {
			metaFound[key] = value
		}
	}
	if len(metaFound) > 0 {
		found["metadata"] = metaFound
	}

	playerFound := map[string]any{}
	for _, raw := range stats {
		for key, value := range raw {
			if !strings.Contains(strings.ToLower(key), "ban") {
				continue
			}
			if isEmptyValue(value) {
				continue
			}
			champion := stringOf(raw["SKIN"])
			if champion == "" {
				champion = "unknown"
			}
			entry, _ := playerFound[champion].(map[string]any)
			if entry == nil {
				entry = map[string]any{}
				playerFound[champion] = entry
			}
			entry[key] = value
		}
	}
	if len(playerFound) > 0 {
		found["players"] = playerFound
	}

	if len(found) > 0 {
		return map[string]any{
			"available": true,
			"blue":      []any{},
			"red":       []any{},
			"raw":       found,
			"note":      "检测到录像元数据中的 Ban 相关字段（非标准字段），已按原始内容返回。",
		}
	}
	return map[string]any{
		"available": false,
		"blue":      []any{},
		"red":       []any{},
		"note":      "ROFL 录像不包含 Ban/Pick 阶段数据：录像从游戏加载时开始记录，Ban 由客户端在选人阶段完成，不写入录像文件。BP 请在本页面手动记录。",
	}
}

func resolvePlayerName(raw map[string]any) string {
	gameName := strings.TrimSpace(stringOf(raw["RIOT_ID_GAME_NAME"]))
	tagLine := strings.TrimSpace(stringOf(raw["RIOT_ID_TAG_LINE"]))
	if gameName != "" {
		if tagLine != "" {
			return gameName + "#" + tagLine
		}
		return gameName
	}
	if name := strings.TrimSpace(stringOf(raw["NAME"])); name != "" {
		return name
	}
	puuid := stringOf(raw["PUUID"])
	if puuid != "" {
		runes := []rune(puuid)
		if len(runes) > 8 {
			runes = runes[:8]
		}
		return string(runes) + "…"
	}
	return "未知玩家"
}

func normalizePosition(raw map[string]any) string {
	for _, key := range []string{"INDIVIDUAL_POSITION", "TEAM_POSITION"} {
		candidate := strings.ToUpper(strings.TrimSpace(stringOf(raw[key])))
		if candidate == "" || candidate == "INVALID" {
			continue
		}
		if mapped, ok := positionAliases[candidate]; ok {
			return mapped
		}
		return candidate
	}
	return ""
}

// readVersionString 读取偏移 14 的版本号：1 字节长度 + ASCII，兼容直接写 15 字节的布局。
func readVersionString(f *os.File, offset int64) string {
	first := readAt(f, offset, 1)
	if len(first) == 1 && (first[0] < '0' || first[0] > '9') {
		length := int(first[0])
		if length > 0 && length <= 64 {
			version := sanitizeVersion(string(readAt(f, offset+1, length)))
			if anyVerRe.MatchString(version) {
				return version
			}
		}
	}
	return sanitizeVersion(string(readAt(f, offset, 15)))
}

func sanitizeVersion(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= 0x20 && r <= 0x7E {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// ddragonVersion 把 16.19.821.7343 转成 DataDragon 目录 16.19.1。
func ddragonVersion(gameVersion string) string {
	clean := sanitizeVersion(gameVersion)
	if m := versionRe.FindStringSubmatch(clean); m != nil {
		return fmt.Sprintf("%s.%s.1", m[1], m[2])
	}
	return "latest"
}

func readAt(f *os.File, offset int64, length int) []byte {
	if offset < 0 || length <= 0 {
		return nil
	}
	buf := make([]byte, length)
	n, err := f.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return nil
	}
	if n <= 0 {
		return nil
	}
	return buf[:n]
}

func stringOf(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		return v.String()
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return 0
		}
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return int(n)
		}
		return 0
	case bool:
		if v {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func toBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case float64:
		return v > 0
	case int:
		return v > 0
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "yes", "win", "1":
			return true
		}
		return false
	default:
		return false
	}
}

func isEmptyValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return v == "" || v == "0"
	case float64:
		return v == 0
	case int:
		return v == 0
	case bool:
		return !v
	default:
		return false
	}
}

// Compile-time assertion that json.RawMessage is usable in this package's helpers.
var _ = bytes.MinRead
