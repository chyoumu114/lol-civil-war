package store

import (
	"encoding/json"
	"time"
)

// User 是全局账号：user_no 是系统生成的唯一标识，
// 邮箱密码与 QQ OpenID 都是可选的登录方式。
type User struct {
	ID           int64
	UserNo       string
	Email        *string
	PasswordHash *string
	QQOpenID     *string
	Name         string
	Avatar       *string
	IsDev        bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Group 是 QQ 群，也是业务数据的隔离边界。
type Group struct {
	ID            int64
	QQGroupNumber string
	PublicKey     string
	Name          *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Member 是群内成员资料；同一用户在不同群有独立记录。
type Member struct {
	ID           int64
	GroupID      int64
	UserID       int64
	QQNumber     *string
	Nickname     string
	RankRegion   *string
	RankTier     *string
	RankDivision *int
	RankLP       int
	Positions    []string
	CreatedAt    time.Time
	UpdatedAt    time.Time

	// 以下字段来自 users 表（只读展示用）。
	UserName string
	Avatar   *string
}

// Event 是一次内战组织。
type Event struct {
	ID              int64
	GroupID         int64
	CreatorMemberID int64
	Title           *string
	Region          string
	BestOf          int
	HasBP           bool
	SignupDeadline  time.Time
	StartsAt        time.Time
	Note            *string
	EndedAt         *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// 以下字段来自 creator 的成员资料（只读展示用）。
	CreatorNickname string
	CreatorAvatar   *string
	CreatorQQNumber *string
}

// Registration 是报名/替补记录。
type Registration struct {
	ID              int64
	EventID         int64
	MemberID        int64
	RankRegion      string
	RankTier        string
	RankDivision    *int
	RankLP          int
	MainPosition    *string
	SubPositions    []string
	SubRankRegion   *string
	SubRankTier     *string
	SubRankDivision *int
	SubRankLP       int
	Note            *string
	IsSpectator     bool
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// 以下字段来自 member 的资料（只读展示用）。
	MemberNickname string
	MemberAvatar   *string
	MemberQQNumber *string
}

// EventRoster 是组织者套用的阵容方案。
type EventRoster struct {
	ID        int64
	EventID   int64
	Payload   json.RawMessage
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Match 是一局对局记录。
type Match struct {
	ID                int64
	EventID           int64
	GameNo            int
	Team1Side         string
	BlueWin           *bool
	DurationMS        *int64
	Status            string
	SubmittedAt       *time.Time
	CreatedByMemberID *int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// MatchPlayer 是一局内的一名玩家。
type MatchPlayer struct {
	ID                int64
	MatchID           int64
	Side              string
	Position          string
	Slot              int
	MemberID          *int64
	PlayerName        string
	Champion          string
	Kills             int
	Deaths            int
	Assists           int
	CS                int
	Gold              int64
	DamageToChampions int64
	DamageTaken       int64
	VisionScore       int
	Win               bool
	Raw               json.RawMessage
	CreatedAt         time.Time
	UpdatedAt         time.Time

	// 以下字段来自 group_members / users（只读展示用）。
	MemberNickname *string
	MemberAvatar   *string
}

// MatchBan 是 BP 禁用记录。
type MatchBan struct {
	ID        int64
	MatchID   int64
	Side      string
	Slot      int
	Champion  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Rating 是赛后评分。
type Rating struct {
	ID             int64
	MatchID        int64
	RaterMemberID  int64
	TargetMemberID int64
	Score          int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// RoflUpload 是录像解析结果（按 sha256 幂等）。
type RoflUpload struct {
	ID                int64
	MatchID           *int64
	FileName          string
	FileSize          int64
	SHA256            string
	GameVersion       string
	DDragonVersion    string
	GameLengthMS      int64
	BlueWin           *bool
	Parsed            json.RawMessage
	CreatedByMemberID *int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Session 是服务端会话。
type Session struct {
	ID            string
	UserID        *int64
	CSRF          string
	OAuthState    string
	OAuthRedirect string
	// QQ 快捷登录尚未完成邮箱注册时的临时状态。
	QQPendingOpenID string
	QQPendingUserID *int64
	QQPendingName   string
	QQPendingAvatar string
	// 已登录用户发起“绑定 QQ”时的目标账号。
	QQBindUserID *int64
	ExpiresAt    time.Time
	CreatedAt    time.Time
}
