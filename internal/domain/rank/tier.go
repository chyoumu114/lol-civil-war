package rank

import "strings"

// Tier 是段位大段。黑铁四 = 1000 分，每升一小段 +100；
// 大师及以上没有小段，统一用 LP 分数。
type Tier string

const (
	TierIron        Tier = "IRON"
	TierBronze      Tier = "BRONZE"
	TierSilver      Tier = "SILVER"
	TierGold        Tier = "GOLD"
	TierPlatinum    Tier = "PLATINUM"
	TierEmerald     Tier = "EMERALD"
	TierDiamond     Tier = "DIAMOND"
	TierMaster      Tier = "MASTER"
	TierGrandmaster Tier = "GRANDMASTER"
	TierChallenger  Tier = "CHALLENGER"
)

// Tiers 按从低到高排列，用于前端下拉与元数据。
var Tiers = []Tier{
	TierIron, TierBronze, TierSilver, TierGold, TierPlatinum,
	TierEmerald, TierDiamond, TierMaster, TierGrandmaster, TierChallenger,
}

var tierLabels = map[Tier]string{
	TierIron:        "黑铁",
	TierBronze:      "青铜",
	TierSilver:      "白银",
	TierGold:        "黄金",
	TierPlatinum:    "铂金",
	TierEmerald:     "翡翠",
	TierDiamond:     "钻石",
	TierMaster:      "大师",
	TierGrandmaster: "宗师",
	TierChallenger:  "王者",
}

var tierStart = map[Tier]int{
	TierIron: 1000, TierBronze: 1400, TierSilver: 1800, TierGold: 2200,
	TierPlatinum: 2600, TierEmerald: 3000, TierDiamond: 3400,
	TierMaster: 4000, TierGrandmaster: 4000, TierChallenger: 4000,
}

// Label 返回中文名，如「钻石」。
func (t Tier) Label() string { return tierLabels[t] }

// StartScore 返回该大段的基础分。
func (t Tier) StartScore() int { return tierStart[t] }

// HasDivisions 报告是否有 4/3/2/1 小段。
func (t Tier) HasDivisions() bool {
	switch t {
	case TierMaster, TierGrandmaster, TierChallenger:
		return false
	default:
		return true
	}
}

// ParseTier 从 code（大小写不敏感）或中文名解析段位。
func ParseTier(value string) (Tier, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	upper := strings.ToUpper(value)
	for _, tier := range Tiers {
		if string(tier) == upper || tier.Label() == value {
			return tier, true
		}
	}
	return "", false
}
