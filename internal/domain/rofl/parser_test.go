package rofl

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildRofl2 合成一个 ROFL2 文件：头部 + 填充 + 元数据 + 4 字节长度。
func buildRofl2(t *testing.T, metadata map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	version := "16.19.1"
	size := 512
	if len(raw)+4+32 > size {
		size = len(raw) + 4 + 32
	}
	buf := make([]byte, size)
	copy(buf, "RIOT\x02\x00")
	buf[14] = byte(len(version))
	copy(buf[15:], version)

	offset := size - 4 - len(raw)
	copy(buf[offset:], raw)
	binary.LittleEndian.PutUint32(buf[size-4:], uint32(len(raw)))

	path := filepath.Join(t.TempDir(), "test.rofl")
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseRofl2(t *testing.T) {
	stats := []map[string]any{
		{
			"TEAM": "100", "SKIN": "Garen", "RIOT_ID_GAME_NAME": "Faker", "RIOT_ID_TAG_LINE": "KR1",
			"PUUID": "abcdefgh-1234", "INDIVIDUAL_POSITION": "TOP", "TEAM_POSITION": "INVALID",
			"CHAMPIONS_KILLED": 7, "NUM_DEATHS": 2, "ASSISTS": 9,
			"MINIONS_KILLED": 180, "NEUTRAL_MINIONS_KILLED": 20,
			"GOLD_EARNED": 12345, "TOTAL_DAMAGE_DEALT_TO_CHAMPIONS": 21000,
			"TOTAL_DAMAGE_TAKEN": 15000, "VISION_SCORE": 25, "WIN": "1",
		},
		{
			"TEAM": "200", "SKIN": "Ahri", "NAME": "对手", "PUUID": "zzzzzzzz-9999",
			"INDIVIDUAL_POSITION": "", "TEAM_POSITION": "MID",
			"CHAMPIONS_KILLED": "3", "NUM_DEATHS": "4", "ASSISTS": "5",
			"MINIONS_KILLED": "150", "NEUTRAL_MINIONS_KILLED": "0",
			"GOLD_EARNED": "9000", "TOTAL_DAMAGE_DEALT_TO_CHAMPIONS": "12000",
			"TOTAL_DAMAGE_TAKEN": "18000", "VISION_SCORE": "12", "WIN": "0",
		},
	}
	statsJSON, _ := json.Marshal(stats)
	path := buildRofl2(t, map[string]any{
		"gameVersion": "16.19.821.7343",
		"gameLength":  1523724,
		"statsJson":   string(statsJSON),
	})

	parser := Parser{}
	metadata, err := parser.ParseFile(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if metadata.Type != "ROFL2" {
		t.Fatalf("类型 = %q", metadata.Type)
	}
	if metadata.GameVersion != "16.19.1" {
		t.Fatalf("版本 = %q", metadata.GameVersion)
	}
	if metadata.DDragonVersion != "16.19.1" {
		t.Fatalf("DataDragon 版本 = %q", metadata.DDragonVersion)
	}
	if metadata.GameLengthMS != 1523724 {
		t.Fatalf("时长 = %d", metadata.GameLengthMS)
	}
	if metadata.BlueWin == nil || !*metadata.BlueWin {
		t.Fatal("蓝方应获胜")
	}
	if len(metadata.Players) != 2 {
		t.Fatalf("玩家数 = %d", len(metadata.Players))
	}

	blue := metadata.Players[0]
	if blue.Team != "blue" || blue.Slot != 1 || blue.Position != "TOP" {
		t.Fatalf("蓝方玩家字段异常: %+v", blue)
	}
	if blue.PlayerName != "Faker#KR1" {
		t.Fatalf("玩家名 = %q", blue.PlayerName)
	}
	if blue.CS != 200 {
		t.Fatalf("补刀 = %d", blue.CS)
	}
	if blue.Kills != 7 || blue.Deaths != 2 || blue.Assists != 9 {
		t.Fatalf("KDA = %d/%d/%d", blue.Kills, blue.Deaths, blue.Assists)
	}
	if !blue.Win {
		t.Fatal("蓝方玩家应获胜")
	}

	red := metadata.Players[1]
	if red.Team != "red" || red.Position != "MIDDLE" || red.PlayerName != "对手" {
		t.Fatalf("红方玩家字段异常: %+v", red)
	}
	if red.Win {
		t.Fatal("红方玩家不应获胜")
	}

	bans, ok := metadata.Bans["available"].(bool)
	if !ok || bans {
		t.Fatalf("Ban 应为不可用: %+v", metadata.Bans)
	}
}

func TestParseRejectsInvalidSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.rofl")
	if err := os.WriteFile(path, []byte("NOPE\x00\x00"+strings.Repeat("0", 600)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := (Parser{}).ParseFile(path)
	if err == nil || !strings.Contains(err.Error(), "RIOT") {
		t.Fatalf("应报签名错误: %v", err)
	}
}

func TestParseRejectsMissingStats(t *testing.T) {
	path := buildRofl2(t, map[string]any{
		"gameVersion": "16.19.821.7343",
		"gameLength":  1000,
	})
	_, err := (Parser{}).ParseFile(path)
	if err == nil || !strings.Contains(err.Error(), "不包含玩家数据") {
		t.Fatalf("应报缺少玩家数据: %v", err)
	}
}

func TestParseRejectsTinyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tiny.rofl")
	if err := os.WriteFile(path, []byte("RIOT\x02\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := (Parser{}).ParseFile(path)
	if err == nil {
		t.Fatal("过小的文件应解析失败")
	}
}
