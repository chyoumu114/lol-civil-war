// Package rofl 解析《英雄联盟》.rofl 录像文件的元数据（Metadata）。
//
// 只读取元数据区（明文 JSON），不解析游戏数据包，因此不需要解密密钥。
// 支持 ROFL2（14.11+，签名 "RIOT\x02\x00"）与 ROFL1（旧格式，尽力兼容）；
// 14.9–14.10 的录像不含元数据，会返回明确错误。
package rofl

// Player 是录像元数据中的一名玩家（赛后统计）。
type Player struct {
	Team              string         `json:"team"`
	Slot              int            `json:"slot"`
	Champion          string         `json:"champion"`
	PlayerName        string         `json:"playerName"`
	PUUID             string         `json:"puuid"`
	Position          string         `json:"position"`
	Kills             int            `json:"kills"`
	Deaths            int            `json:"deaths"`
	Assists           int            `json:"assists"`
	CS                int            `json:"cs"`
	Gold              int            `json:"gold"`
	DamageToChampions int            `json:"damageToChampions"`
	DamageTaken       int            `json:"damageTaken"`
	VisionScore       int            `json:"visionScore"`
	Win               bool           `json:"win"`
	Raw               map[string]any `json:"raw"`
}

// Metadata 是录像解析结果（只读）。
type Metadata struct {
	Type           string
	GameVersion    string
	DDragonVersion string
	GameLengthMS   int
	BlueWin        *bool
	Players        []Player
	Bans           map[string]any
}

// ToMap 转成写入数据库与返回给前端的结构（不含原始 metadata）。
func (m Metadata) ToMap() map[string]any {
	players := make([]Player, 0, len(m.Players))
	players = append(players, m.Players...)

	var blueWin any
	if m.BlueWin != nil {
		blueWin = *m.BlueWin
	}
	return map[string]any{
		"type":           m.Type,
		"gameVersion":    m.GameVersion,
		"ddragonVersion": m.DDragonVersion,
		"gameLengthMs":   m.GameLengthMS,
		"blueWin":        blueWin,
		"players":        players,
		"bans":           m.Bans,
	}
}

// ParseError 表示录像无法解析（格式、版本、损坏）。
type ParseError struct{ Message string }

func (e *ParseError) Error() string { return e.Message }

func parseErr(msg string) error { return &ParseError{Message: msg} }
