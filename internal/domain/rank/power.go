package rank

import "math"

// Region 是大区配置：名称 + 战力系数。
type Region struct {
	Name        string
	Coefficient float64
}

// DefaultRegions 是内置大区表。未列出的大区按 1.0 计算。
var DefaultRegions = []Region{
	{"艾欧尼亚", 1.1},
	{"峡谷之巅", 1.3},
	{"韩服", 1.5},
	{"黑色玫瑰", 1.0},
	{"祖安", 1.0},
	{"诺克萨斯", 1.0},
	{"班德尔城", 1.0},
	{"皮尔特沃夫", 1.0},
	{"战争学院", 1.0},
	{"巨神峰", 1.0},
	{"雷瑟守备", 1.0},
	{"裁决之地", 1.0},
	{"暗影岛", 1.0},
	{"钢铁烈阳", 1.0},
	{"水晶之痕", 1.0},
	{"均衡教派", 1.0},
	{"影流", 1.0},
	{"守望之海", 1.0},
	{"征服之海", 1.0},
	{"卡拉曼达", 1.0},
	{"巨龙之巢", 1.0},
	{"教育网专区", 1.0},
	{"男爵领域", 1.0},
	{"欧服", 1.0},
	{"美服", 1.0},
	{"日服", 1.0},
}

// RegionNames 按配置顺序返回大区名，供前端下拉使用。
func RegionNames() []string {
	out := make([]string, 0, len(DefaultRegions))
	for _, r := range DefaultRegions {
		out = append(out, r.Name)
	}
	return out
}

// RegionCoefficient 返回大区系数，未配置按 1.0。
func RegionCoefficient(name string) float64 {
	for _, r := range DefaultRegions {
		if r.Name == name {
			return r.Coefficient
		}
	}
	return 1.0
}

// IsRegion 报告大区是否合法。
func IsRegion(name string) bool {
	for _, r := range DefaultRegions {
		if r.Name == name {
			return true
		}
	}
	return false
}

// /* 位置影响力默认值：上单 0.17、打野 0.23、中单 0.22、下路 0.17、辅助 0.21。 */
var defaultInfluence = map[Position]float64{
	PositionTop:     0.17,
	PositionJungle:  0.23,
	PositionMiddle:  0.22,
	PositionBottom:  0.17,
	PositionUtility: 0.21,
}

// PowerCalculator 计算单名玩家在某个位置上的战力：
//
//	power = round(baseScore × 大区系数 × (1 + 位置影响力))
type PowerCalculator struct {
	DivisionStep int
}

// NewPowerCalculator 构造计算器；divisionStep <= 0 时使用默认 100。
func NewPowerCalculator(divisionStep int) *PowerCalculator {
	if divisionStep <= 0 {
		divisionStep = 100
	}
	return &PowerCalculator{DivisionStep: divisionStep}
}

// Power 返回战力。
func (c *PowerCalculator) Power(p Profile, position Position) int {
	step := p.DivisionStep
	if step <= 0 {
		step = c.DivisionStep
	}
	profile := p
	profile.DivisionStep = step
	return int(math.Round(float64(profile.BaseScore()) * RegionCoefficient(profile.Region) * (1 + Influence(position))))
}

// Breakdown 返回战力拆解，供调试与前端展示。
func (c *PowerCalculator) Breakdown(p Profile, position Position) map[string]any {
	region := RegionCoefficient(p.Region)
	influence := Influence(position)
	return map[string]any{
		"baseScore":         p.BaseScore(),
		"regionCoefficient": region,
		"positionInfluence": influence,
		"power":             int(math.Round(float64(p.BaseScore()) * region * (1 + influence))),
	}
}

// Influence 返回位置影响力。
func Influence(position Position) float64 {
	if v, ok := defaultInfluence[position]; ok {
		return v
	}
	return 0
}
