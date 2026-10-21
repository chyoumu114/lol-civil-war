package rank

import (
	"errors"
	"fmt"
)

// Profile 描述一段段位：大区 + 大段 + 小段/LP。值对象，不可变。
type Profile struct {
	Region       string
	Tier         Tier
	Division     *int
	LP           int
	DivisionStep int
}

// NewProfile 构造段位并校验。小段必须 1-4，LP 不能为负。
func NewProfile(region string, tier Tier, division *int, lp, divisionStep int) (Profile, error) {
	if divisionStep <= 0 {
		divisionStep = 100
	}
	p := Profile{Region: region, Tier: tier, Division: division, LP: lp, DivisionStep: divisionStep}
	if tier.HasDivisions() {
		if division == nil || *division < 1 || *division > 4 {
			return Profile{}, fmt.Errorf("%s必须指定 1-4 的小段", tier.Label())
		}
	} else if lp < 0 {
		return Profile{}, errors.New("分数不能为负数")
	}
	return p, nil
}

// BaseScore 小段位 = start + (4 - division) × step；大师及以上 = start + LP。
func (p Profile) BaseScore() int {
	if p.Tier.HasDivisions() {
		d := 1
		if p.Division != nil {
			d = *p.Division
		}
		return p.Tier.StartScore() + (4-d)*p.DivisionStep
	}
	return p.Tier.StartScore() + p.LP
}

// Label 展示文本，如「艾欧尼亚 钻石3」「韩服 大师300分」。
func (p Profile) Label() string {
	text := p.Region + " " + p.Tier.Label()
	if p.Tier.HasDivisions() {
		if p.Division != nil {
			return fmt.Sprintf("%s%d", text, *p.Division)
		}
		return text
	}
	return fmt.Sprintf("%s%d分", text, p.LP)
}
