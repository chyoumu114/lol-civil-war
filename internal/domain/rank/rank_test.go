package rank

import "testing"

func TestParseTier(t *testing.T) {
	cases := map[string]Tier{
		"DIAMOND":    TierDiamond,
		"diamond":    TierDiamond,
		"钻石":         TierDiamond,
		"王者":         TierChallenger,
		"CHALLENGER": TierChallenger,
	}
	for input, want := range cases {
		got, ok := ParseTier(input)
		if !ok || got != want {
			t.Fatalf("ParseTier(%q) = %q, %v; want %q", input, got, ok, want)
		}
	}
	if _, ok := ParseTier(""); ok {
		t.Fatal("空字符串不应解析成功")
	}
	if _, ok := ParseTier("UNKNOWN"); ok {
		t.Fatal("未知段位不应解析成功")
	}
}

func TestParsePosition(t *testing.T) {
	cases := map[string]Position{
		"top": PositionTop, "上单": PositionTop, "上路": PositionTop,
		"jg": PositionJungle, "打野": PositionJungle,
		"mid": PositionMiddle, "中单": PositionMiddle,
		"adc": PositionBottom, "射手": PositionBottom,
		"sup": PositionUtility, "辅助": PositionUtility,
	}
	for input, want := range cases {
		got, ok := ParsePosition(input)
		if !ok || got != want {
			t.Fatalf("ParsePosition(%q) = %q, %v; want %q", input, got, ok, want)
		}
	}
	if _, ok := ParsePosition("JG"); ok {
		t.Fatal("别名应区分大小写（与旧实现一致）")
	}
	if _, ok := ParsePosition(""); ok {
		t.Fatal("空字符串不应解析成功")
	}
}

func TestProfileBaseScoreAndLabel(t *testing.T) {
	d := 3
	profile, err := NewProfile("艾欧尼亚", TierDiamond, &d, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := profile.BaseScore(), 3500; got != want {
		t.Fatalf("BaseScore = %d, want %d", got, want)
	}
	if got, want := profile.Label(), "艾欧尼亚 钻石3"; got != want {
		t.Fatalf("Label = %q, want %q", got, want)
	}

	master, err := NewProfile("韩服", TierMaster, nil, 300, 100)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := master.BaseScore(), 4300; got != want {
		t.Fatalf("大师 BaseScore = %d, want %d", got, want)
	}
	if got, want := master.Label(), "韩服 大师300分"; got != want {
		t.Fatalf("大师 Label = %q, want %q", got, want)
	}
}

func TestNewProfileValidation(t *testing.T) {
	zero := 0
	if _, err := NewProfile("艾欧尼亚", TierDiamond, &zero, 0, 100); err == nil {
		t.Fatal("小段 0 应校验失败")
	}
	if _, err := NewProfile("艾欧尼亚", TierDiamond, nil, 0, 100); err == nil {
		t.Fatal("缺少小段应校验失败")
	}
	if _, err := NewProfile("韩服", TierMaster, nil, -1, 100); err == nil {
		t.Fatal("负 LP 应校验失败")
	}
}

func TestPower(t *testing.T) {
	calc := NewPowerCalculator(100)
	d := 3
	profile, _ := NewProfile("艾欧尼亚", TierDiamond, &d, 0, 100)

	// 3500 × 1.1 × 1.17 = 4504.5 → 4505
	if got, want := calc.Power(profile, PositionTop), 4505; got != want {
		t.Fatalf("战力 = %d, want %d", got, want)
	}
	// 未配置的大区按 1.0
	other, _ := NewProfile("未知大区", TierDiamond, &d, 0, 100)
	if got, want := calc.Power(other, PositionTop), 4095; got != want {
		t.Fatalf("未知大区战力 = %d, want %d", got, want)
	}
	if got := Influence(PositionJungle); got != 0.23 {
		t.Fatalf("打野影响力 = %v", got)
	}
}
