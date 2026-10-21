package rank

import "strings"

// Position 是五个分路。key 与 ROFL 元数据、前端保持一致。
type Position string

const (
	PositionTop     Position = "TOP"
	PositionJungle  Position = "JUNGLE"
	PositionMiddle  Position = "MIDDLE"
	PositionBottom  Position = "BOTTOM"
	PositionUtility Position = "UTILITY"
)

// Positions 按 上→野→中→下→辅 顺序排列。
var Positions = []Position{
	PositionTop, PositionJungle, PositionMiddle, PositionBottom, PositionUtility,
}

var positionLabels = map[Position]string{
	PositionTop: "上单", PositionJungle: "打野", PositionMiddle: "中单",
	PositionBottom: "下路", PositionUtility: "辅助",
}

var positionShorts = map[Position]string{
	PositionTop: "上", PositionJungle: "野", PositionMiddle: "中",
	PositionBottom: "下", PositionUtility: "辅",
}

// Label 返回中文名，如「上单」。
func (p Position) Label() string { return positionLabels[p] }

// Short 返回单字简称，如「上」。
func (p Position) Short() string { return positionShorts[p] }

// PositionKeys 返回全部位置 code。
func PositionKeys() []string {
	out := make([]string, 0, len(Positions))
	for _, p := range Positions {
		out = append(out, string(p))
	}
	return out
}

// ParsePosition 兼容多种写法：code（大小写不敏感）、中文与常见别名。
// 注意：与旧实现保持一致，别名本身区分大小写。
func ParsePosition(value string) (Position, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	upper := strings.ToUpper(value)
	for _, p := range Positions {
		if string(p) == upper {
			return p, true
		}
	}
	switch value {
	case "上单", "上路":
		return PositionTop, true
	case "打野", "野区", "jg":
		return PositionJungle, true
	case "中单", "中路", "mid":
		return PositionMiddle, true
	case "下路", "射手", "adc", "bot":
		return PositionBottom, true
	case "辅助", "sup", "support":
		return PositionUtility, true
	default:
		return "", false
	}
}
