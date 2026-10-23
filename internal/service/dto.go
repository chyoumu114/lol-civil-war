package service

import "encoding/json"

// RankView 是段位 JSON（不含基础分）。
type RankView struct {
	Region    string `json:"region"`
	Tier      string `json:"tier"`
	TierLabel string `json:"tierLabel"`
	Division  *int   `json:"division"`
	LP        int    `json:"lp"`
	Label     string `json:"label"`
}

// MemberRankView 是成员资料里的段位（含基础分）。
type MemberRankView struct {
	RankView
	BaseScore int `json:"baseScore"`
}

// MemberView 是群内成员资料。
type MemberView struct {
	ID              int64           `json:"id"`
	QQNumber        *string         `json:"qqNumber"`
	Nickname        string          `json:"nickname"`
	Avatar          *string         `json:"avatar"`
	Rank            *MemberRankView `json:"rank"`
	Positions       []string        `json:"positions"`
	ProfileComplete bool            `json:"profileComplete"`
	JoinedAt        *string         `json:"joinedAt"`
}

// UserView 是全局账号信息。userNo 是系统生成的唯一标识。
type UserView struct {
	ID          int64   `json:"id"`
	UserNo      string  `json:"userNo"`
	Name        string  `json:"name"`
	Avatar      *string `json:"avatar"`
	Email       *string `json:"email"`
	IsDev       bool    `json:"isDev"`
	QQBound     bool    `json:"qqBound"`
	HasPassword bool    `json:"hasPassword"`
}

// MeView 是 /api/me 响应。
type MeView struct {
	Authenticated bool      `json:"authenticated"`
	User          *UserView `json:"user"`
	DevLogin      bool      `json:"devLogin"`
}

// CreatorView 是组织者摘要。
type CreatorView struct {
	MemberID int64   `json:"memberId"`
	Nickname string  `json:"nickname"`
	Avatar   *string `json:"avatar"`
	QQNumber *string `json:"qqNumber"`
}

// EventView 是组织摘要。
type EventView struct {
	ID             int64        `json:"id"`
	Title          *string      `json:"title"`
	Region         string       `json:"region"`
	BestOf         int          `json:"bestOf"`
	HasBP          bool         `json:"hasBp"`
	SignupDeadline *string      `json:"signupDeadline"`
	StartsAt       *string      `json:"startsAt"`
	Note           *string      `json:"note"`
	Status         string       `json:"status"`
	EndedAt        *string      `json:"endedAt"`
	CreatedAt      *string      `json:"createdAt"`
	Creator        *CreatorView `json:"creator"`
}

// EventListItem 是组织列表项（带统计）。
type EventListItem struct {
	EventView
	RegistrationCount int `json:"registrationCount"`
	MatchCount        int `json:"matchCount"`
	Team1Wins         int `json:"team1Wins"`
	Team2Wins         int `json:"team2Wins"`
}

// RegistrationView 是报名列表项。
type RegistrationView struct {
	ID             int64    `json:"id"`
	MemberID       int64    `json:"memberId"`
	Nickname       string   `json:"nickname"`
	Avatar         *string  `json:"avatar"`
	QQNumber       *string  `json:"qqNumber"`
	Rank           RankView `json:"rank"`
	SubRank        RankView `json:"subRank"`
	MainPosition   *string  `json:"mainPosition"`
	SubPositions   []string `json:"subPositions"`
	Note           *string  `json:"note"`
	IsSpectator    bool     `json:"isSpectator"`
	PositionPowers any      `json:"positionPowers"`
	Editable       bool     `json:"editable"`
	IsOrganizer    bool     `json:"isOrganizer"`
	CreatedAt      *string  `json:"createdAt"`
}

// PermissionsView 是组织详情里的权限位。
type PermissionsView struct {
	IsOrganizer            bool `json:"isOrganizer"`
	SignupOpen             bool `json:"signupOpen"`
	IsEnded                bool `json:"isEnded"`
	CanRegister            bool `json:"canRegister"`
	CanManageRegistrations bool `json:"canManageRegistrations"`
	CanArrange             bool `json:"canArrange"`
	CanAutoMatch           bool `json:"canAutoMatch"`
	CanSpectate            bool `json:"canSpectate"`
	CanRecordMatch         bool `json:"canRecordMatch"`
	CanEnd                 bool `json:"canEnd"`
}

// SeriesMatchView 是系列赛中的一局摘要。
type SeriesMatchView struct {
	ID          int64   `json:"id"`
	GameNo      int     `json:"gameNo"`
	Status      string  `json:"status"`
	BlueWin     *bool   `json:"blueWin"`
	Team1Side   string  `json:"team1Side"`
	WinnerTeam  *int    `json:"winnerTeam"`
	DurationMS  *int64  `json:"durationMs"`
	SubmittedAt *string `json:"submittedAt"`
}

// SeriesView 是系列赛比分。
type SeriesView struct {
	BestOf       int               `json:"bestOf"`
	WinsRequired int               `json:"winsRequired"`
	Team1Wins    int               `json:"team1Wins"`
	Team2Wins    int               `json:"team2Wins"`
	TeamWinner   *int              `json:"teamWinner"`
	Matches      []SeriesMatchView `json:"matches"`
}

// DraftView 是当前草稿局。
type DraftView struct {
	GameNo int `json:"gameNo"`
}

// EventDetailView 是组织详情。
type EventDetailView struct {
	Event            EventView          `json:"event"`
	Permissions      PermissionsView    `json:"permissions"`
	Registrations    []RegistrationView `json:"registrations"`
	ParticipantCount int                `json:"participantCount"`
	SubstituteCount  int                `json:"substituteCount"`
	SpectatorCount   int                `json:"spectatorCount"`
	MySpectator      bool               `json:"mySpectator"`
	MyRegistration   *RegistrationView  `json:"myRegistration"`
	Series           SeriesView         `json:"series"`
	Roster           json.RawMessage    `json:"roster"`
	Draft            *DraftView         `json:"draft"`
}

// MatchView 是对局摘要。
type MatchView struct {
	ID          int64   `json:"id"`
	GameNo      int     `json:"gameNo"`
	BlueWin     *bool   `json:"blueWin"`
	DurationMS  *int64  `json:"durationMs"`
	Status      string  `json:"status"`
	Team1Side   string  `json:"team1Side"`
	SubmittedAt *string `json:"submittedAt"`
	Editable    bool    `json:"editable"`
}

// PlayerView 是对局内的一名玩家。
type PlayerView struct {
	ID                int64           `json:"id"`
	Side              string          `json:"side"`
	Position          string          `json:"position"`
	Slot              int             `json:"slot"`
	MemberID          *int64          `json:"memberId"`
	MemberNickname    *string         `json:"memberNickname"`
	MemberAvatar      *string         `json:"memberAvatar"`
	PlayerName        string          `json:"playerName"`
	Champion          string          `json:"champion"`
	Kills             int             `json:"kills"`
	Deaths            int             `json:"deaths"`
	Assists           int             `json:"assists"`
	CS                int             `json:"cs"`
	Gold              int             `json:"gold"`
	DamageToChampions int             `json:"damageToChampions"`
	DamageTaken       int             `json:"damageTaken"`
	VisionScore       int             `json:"visionScore"`
	Win               bool            `json:"win"`
	Raw               json.RawMessage `json:"raw"`
}

// BanView 是 BP 禁用。
type BanView struct {
	Side     string `json:"side"`
	Slot     int    `json:"slot"`
	Champion string `json:"champion"`
}

// RoflView 是录像信息。
type RoflView struct {
	ID             int64           `json:"id"`
	FileName       string          `json:"fileName"`
	GameVersion    string          `json:"gameVersion"`
	DDragonVersion string          `json:"ddragonVersion"`
	GameLengthMS   int64           `json:"gameLengthMs"`
	BlueWin        *bool           `json:"blueWin"`
	UploadedAt     *string         `json:"uploadedAt"`
	Players        json.RawMessage `json:"players"`
	Bans           json.RawMessage `json:"bans"`
}

// EventOfMatchView 是对局详情里的组织摘要。
type EventOfMatchView struct {
	ID           int64      `json:"id"`
	Region       string     `json:"region"`
	BestOf       int        `json:"bestOf"`
	HasBP        bool       `json:"hasBp"`
	WinsRequired int        `json:"winsRequired"`
	IsEnded      bool       `json:"isEnded"`
	EndedAt      *string    `json:"endedAt"`
	Series       SeriesMini `json:"series"`
}

// SeriesMini 是 1队/2队 胜场（不是蓝/红方胜场）。
type SeriesMini struct {
	Team1Wins int `json:"team1Wins"`
	Team2Wins int `json:"team2Wins"`
}

// RegistrationOptionView 是对局里可选的报名者。
type RegistrationOptionView struct {
	RegistrationID int64    `json:"registrationId"`
	MemberID       int64    `json:"memberId"`
	Nickname       string   `json:"nickname"`
	Avatar         *string  `json:"avatar"`
	QQNumber       *string  `json:"qqNumber"`
	MainPosition   *string  `json:"mainPosition"`
	SubPositions   []string `json:"subPositions"`
	RankLabel      string   `json:"rankLabel"`
	Rank           RankView `json:"rank"`
	SubRank        RankView `json:"subRank"`
}

// RatingSummaryView 是某人的评分汇总。
type RatingSummaryView struct {
	MemberID int64   `json:"memberId"`
	Average  float64 `json:"average"`
	Count    int     `json:"count"`
	MyScore  *int    `json:"myScore"`
}

// MatchDetailView 是对局详情。
type MatchDetailView struct {
	Match         MatchView                `json:"match"`
	Players       []PlayerView             `json:"players"`
	Bans          []BanView                `json:"bans"`
	Rofl          *RoflView                `json:"rofl"`
	Event         EventOfMatchView         `json:"event"`
	Registrations []RegistrationOptionView `json:"registrations"`
	CanRate       bool                     `json:"canRate"`
	Ratings       any                      `json:"ratings"`
	RatingLabels  map[string]string        `json:"ratingLabels"`
}

// TierView 是段位元数据。
type TierView struct {
	Code         string `json:"code"`
	Label        string `json:"label"`
	HasDivisions bool   `json:"hasDivisions"`
	StartScore   int    `json:"startScore"`
}

// PositionView 是位置元数据。
type PositionView struct {
	Code      string  `json:"code"`
	Label     string  `json:"label"`
	Short     string  `json:"short"`
	Influence float64 `json:"influence"`
}

// MetaView 是前端静态配置。
type MetaView struct {
	Regions      []string       `json:"regions"`
	RankTiers    []TierView     `json:"rankTiers"`
	Positions    []PositionView `json:"positions"`
	BestOf       []int          `json:"bestOf"`
	DevLogin     bool           `json:"devLogin"`
	QQConfigured bool           `json:"qqConfigured"`
	MaxRoflKB    int64          `json:"maxRoflKb"`
}

// RankingStatItem 是排行榜的统计项。
type RankingStatItem struct {
	Rank           int     `json:"rank"`
	MemberID       int64   `json:"memberId"`
	Nickname       string  `json:"nickname"`
	Avatar         *string `json:"avatar"`
	QQNumber       *string `json:"qqNumber"`
	Games          int     `json:"games"`
	Wins           int     `json:"wins"`
	Losses         int     `json:"losses"`
	WinRate        float64 `json:"winRate"`
	Kills          int     `json:"kills"`
	Deaths         int     `json:"deaths"`
	Assists        int     `json:"assists"`
	CS             int     `json:"cs"`
	Gold           int     `json:"gold"`
	Damage         int     `json:"damage"`
	DamageTaken    int     `json:"damageTaken"`
	VisionScore    int     `json:"visionScore"`
	KDA            float64 `json:"kda"`
	AvgKills       float64 `json:"avgKills"`
	AvgDeaths      float64 `json:"avgDeaths"`
	AvgAssists     float64 `json:"avgAssists"`
	AvgCS          float64 `json:"avgCs"`
	AvgGold        float64 `json:"avgGold"`
	AvgDamage      float64 `json:"avgDamage"`
	AvgDamageTaken float64 `json:"avgDamageTaken"`
	AvgVisionScore float64 `json:"avgVisionScore"`
}

// RankingAllItem 是合并排行项（统计 + 评分）。
type RankingAllItem struct {
	RankingStatItem
	Rating      float64 `json:"rating"`
	RatingCount int     `json:"ratingCount"`
}

// RankingRatingItem 是评分榜项（字段集与统计项不同）。
type RankingRatingItem struct {
	Rank        int     `json:"rank"`
	MemberID    int64   `json:"memberId"`
	Nickname    string  `json:"nickname"`
	Avatar      *string `json:"avatar"`
	Rating      float64 `json:"rating"`
	RatingCount int     `json:"ratingCount"`
	Games       int     `json:"games"`
	Wins        int     `json:"wins"`
	Losses      int     `json:"losses"`
	WinRate     float64 `json:"winRate"`
}

// RankingView 是排行榜响应。
type RankingView struct {
	Metric  string   `json:"metric"`
	Metrics []string `json:"metrics"`
	Items   any      `json:"items"`
}

// TeammateView 是队友分析项。
type TeammateView struct {
	MemberID int64   `json:"memberId"`
	Nickname string  `json:"nickname"`
	QQNumber *string `json:"qqNumber"`
	Games    int     `json:"games"`
	Wins     int     `json:"wins"`
	WinRate  float64 `json:"winRate"`
}

// TeammatesView 是最佳/最差队友。
type TeammatesView struct {
	Best  []TeammateView `json:"best"`
	Worst []TeammateView `json:"worst"`
}

// PlayerSummaryView 是个人分析汇总。
type PlayerSummaryView struct {
	Games      int     `json:"games"`
	Wins       int     `json:"wins"`
	Losses     int     `json:"losses"`
	WinRate    float64 `json:"winRate"`
	Kills      int     `json:"kills"`
	Deaths     int     `json:"deaths"`
	Assists    int     `json:"assists"`
	CS         int     `json:"cs"`
	Gold       int     `json:"gold"`
	Damage     int     `json:"damage"`
	BestKills  int     `json:"bestKills"`
	BestDamage int     `json:"bestDamage"`
	AvgKills   float64 `json:"avgKills"`
	AvgDeaths  float64 `json:"avgDeaths"`
	AvgAssists float64 `json:"avgAssists"`
	AvgCS      float64 `json:"avgCs"`
	AvgGold    float64 `json:"avgGold"`
	AvgDamage  float64 `json:"avgDamage"`
}

// RecentGameView 是最近对局。
type RecentGameView struct {
	MatchID  int64  `json:"matchId"`
	EventID  int64  `json:"eventId"`
	GameNo   int    `json:"gameNo"`
	Region   string `json:"region"`
	BestOf   int    `json:"bestOf"`
	Champion string `json:"champion"`
	Position string `json:"position"`
	Win      bool   `json:"win"`
	Kills    int    `json:"kills"`
	Deaths   int    `json:"deaths"`
	Assists  int    `json:"assists"`
	CS       int    `json:"cs"`
	Gold     int    `json:"gold"`
	Damage   int    `json:"damage"`
}

// PlayerAnalysisView 是个人分析响应。
type PlayerAnalysisView struct {
	Member struct {
		ID        int64    `json:"id"`
		Nickname  string   `json:"nickname"`
		QQNumber  *string  `json:"qqNumber"`
		Positions []string `json:"positions"`
	} `json:"member"`
	Summary   PlayerSummaryView `json:"summary"`
	Teammates TeammatesView     `json:"teammates"`
	Recent    []RecentGameView  `json:"recent"`
}

// PowerView 是战力计算器响应。
type PowerView struct {
	Position          string   `json:"position"`
	PositionLabel     string   `json:"positionLabel"`
	PositionShort     string   `json:"positionShort"`
	Region            string   `json:"region"`
	Tier              string   `json:"tier"`
	TierLabel         string   `json:"tierLabel"`
	Division          *int     `json:"division"`
	LP                int      `json:"lp"`
	Rank              RankView `json:"rank"`
	BaseScore         int      `json:"baseScore"`
	RegionCoefficient float64  `json:"regionCoefficient"`
	PositionInfluence float64  `json:"positionInfluence"`
	Power             int      `json:"power"`
}

// ChampionNameView 是 DataDragon 英雄名。
type ChampionNameView struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Name  string `json:"name"`
	Title string `json:"title"`
}

// ChampionNamesView 是 /api/champions 响应。
type ChampionNamesView struct {
	Version       string             `json:"version"`
	Champions     []ChampionNameView `json:"champions"`
	LatestVersion string             `json:"latestVersion"`
}
