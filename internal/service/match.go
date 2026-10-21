package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chyoumu114/lol-civil-war/internal/domain/match"
	"github.com/chyoumu114/lol-civil-war/internal/domain/rank"
	"github.com/chyoumu114/lol-civil-war/internal/domain/rofl"
	"github.com/chyoumu114/lol-civil-war/internal/store"
)

// UploadedFile 是上传文件的抽象（web 层负责落临时文件）。
type UploadedFile struct {
	Path string
	Name string
	Size int64
}

// MatchSavePayload 是保存对局草稿的请求体。
type MatchSavePayload struct {
	Team1Side  string            `json:"team1Side"`
	BlueWin    json.RawMessage   `json:"blueWin"`
	DurationMS *json.Number      `json:"durationMs"`
	Players    []json.RawMessage `json:"players"`
	Bans       []json.RawMessage `json:"bans"`
}

// MatchCreatedView 是新增对局的返回。
type MatchCreatedView struct {
	ID     int64  `json:"id"`
	GameNo int    `json:"gameNo"`
	Status string `json:"status"`
}

// CreateMatch 新增下一局（从已套用阵容生成 10 名玩家）。
func (a *App) CreateMatch(ctx context.Context, event store.Event, actor store.Member) (MatchCreatedView, error) {
	if event.CreatorMemberID != actor.ID {
		return MatchCreatedView{}, ErrForbidden("只有组织者可以记录对局")
	}
	if event.EndedAt != nil {
		return MatchCreatedView{}, ErrConflict("比赛已结束，不能再新增对局")
	}
	roster, err := a.RosterPayloadFromStore(ctx, event.ID)
	if err != nil {
		return MatchCreatedView{}, err
	}
	if roster == nil || !rosterPayloadIsComplete(mustJSON(roster)) {
		return MatchCreatedView{}, ErrConflict("队伍信息还未满员，请先完成编排")
	}

	matches, err := a.St.MatchesByEvent(ctx, event.ID)
	if err != nil {
		return MatchCreatedView{}, err
	}
	team1Wins, team2Wins := teamSeries(matches)
	if match.IsOver(event.BestOf, team1Wins, team2Wins) {
		return MatchCreatedView{}, ErrConflict("该系列赛已分出胜负，不能再新增对局")
	}

	maxGameNo := 0
	for _, m := range matches {
		if m.GameNo > maxGameNo {
			maxGameNo = m.GameNo
		}
	}
	if maxGameNo >= event.BestOf {
		return MatchCreatedView{}, ErrConflict(fmt.Sprintf("%d 局已达到赛制上限", event.BestOf))
	}

	created, err := a.St.CreateMatch(ctx, store.Match{
		EventID:           event.ID,
		GameNo:            maxGameNo + 1,
		Team1Side:         "blue",
		Status:            "draft",
		CreatedByMemberID: &actor.ID,
	})
	if err != nil {
		return MatchCreatedView{}, err
	}
	if err := a.fillPlayersFromRoster(ctx, event, created, roster); err != nil {
		return MatchCreatedView{}, err
	}
	return MatchCreatedView{ID: created.ID, GameNo: created.GameNo, Status: created.Status}, nil
}

// ShowMatch 返回一局详情；组织者打开空草稿时会自动从阵容补齐玩家。
func (a *App) ShowMatch(ctx context.Context, event store.Event, gameNo int, viewer store.Member) (MatchDetailView, error) {
	found, err := a.St.MatchByEventGameNo(ctx, event.ID, gameNo)
	if err != nil {
		if IsNotFound(err) {
			return MatchDetailView{}, ErrNotFound("找不到该局对局记录")
		}
		return MatchDetailView{}, err
	}
	editable := event.CreatorMemberID == viewer.ID && event.EndedAt == nil
	if editable && found.Status == "draft" {
		count, err := a.St.CountPlayers(ctx, found.ID)
		if err != nil {
			return MatchDetailView{}, err
		}
		if count == 0 {
			roster, err := a.RosterPayloadFromStore(ctx, event.ID)
			if err != nil {
				return MatchDetailView{}, err
			}
			if roster != nil {
				if err := a.fillPlayersFromRoster(ctx, event, found, roster); err != nil {
					return MatchDetailView{}, err
				}
			}
		}
	}

	players, err := a.St.PlayersByMatch(ctx, found.ID)
	if err != nil {
		return MatchDetailView{}, err
	}
	bans, err := a.St.BansByMatch(ctx, found.ID)
	if err != nil {
		return MatchDetailView{}, err
	}
	roflUpload, err := a.St.RoflByMatch(ctx, found.ID)
	if err != nil {
		return MatchDetailView{}, err
	}
	matches, err := a.St.MatchesByEvent(ctx, event.ID)
	if err != nil {
		return MatchDetailView{}, err
	}
	blueWins, redWins := blueRedWins(matches)

	registrations, err := a.St.RegistrationsByEvent(ctx, event.ID)
	if err != nil {
		return MatchDetailView{}, err
	}

	view := MatchDetailView{
		Match: MatchView{
			ID:          found.ID,
			GameNo:      found.GameNo,
			BlueWin:     found.BlueWin,
			DurationMS:  found.DurationMS,
			Status:      found.Status,
			Team1Side:   defaultSide(found.Team1Side),
			SubmittedAt: a.isoPtr(found.SubmittedAt),
			Editable:    editable,
		},
		Players:       make([]PlayerView, 0, len(players)),
		Bans:          make([]BanView, 0, len(bans)),
		Event:         a.eventOfMatchView(event, blueWins, redWins),
		Registrations: a.registrationOptions(registrations),
		RatingLabels:  ratingLabels(),
	}

	for _, player := range players {
		view.Players = append(view.Players, playerView(player))
	}
	for _, ban := range bans {
		view.Bans = append(view.Bans, BanView{Side: ban.Side, Slot: ban.Slot, Champion: ban.Champion})
	}
	if roflUpload != nil {
		view.Rofl = a.roflView(roflUpload)
	}

	canRate, err := a.canRate(ctx, event, viewer)
	if err != nil {
		return MatchDetailView{}, err
	}
	view.CanRate = canRate
	ratings, err := a.ratingSummary(ctx, found.ID, viewer.ID)
	if err != nil {
		return MatchDetailView{}, err
	}
	view.Ratings = ratings
	return view, nil
}

// SaveMatch 保存草稿（全量覆盖 players/bans）。
func (a *App) SaveMatch(ctx context.Context, event store.Event, gameNo int, actor store.Member, payload MatchSavePayload) (MatchDetailView, error) {
	if event.CreatorMemberID != actor.ID {
		return MatchDetailView{}, ErrForbidden("只有组织者可以记录对局")
	}
	if event.EndedAt != nil {
		return MatchDetailView{}, ErrConflict("比赛已结束，对局记录不可修改")
	}
	found, err := a.St.MatchByEventGameNo(ctx, event.ID, gameNo)
	if err != nil {
		if IsNotFound(err) {
			return MatchDetailView{}, ErrNotFound("找不到该局对局记录")
		}
		return MatchDetailView{}, err
	}

	team1Side := payload.Team1Side
	if team1Side == "" {
		team1Side = "blue"
	}
	if team1Side != "blue" && team1Side != "red" {
		return MatchDetailView{}, ErrInvalid("1队所在方必须是蓝方或红方")
	}

	blueWin, err := parseNullableBool(payload.BlueWin, "胜负字段必须是布尔值")
	if err != nil {
		return MatchDetailView{}, err
	}

	var duration *int64
	if payload.DurationMS != nil {
		value, err := payload.DurationMS.Int64()
		if err != nil {
			return MatchDetailView{}, ErrInvalid("对局时长不合法")
		}
		if value < 0 || value > 24*3600*1000 {
			return MatchDetailView{}, ErrInvalid("对局时长不合法")
		}
		duration = &value
	}

	// 录像已解析出胜方/时长时，客户端未显式提供则沿用录像值。
	roflUpload, err := a.St.RoflByMatch(ctx, found.ID)
	if err != nil {
		return MatchDetailView{}, err
	}
	if blueWin == nil && roflUpload != nil && roflUpload.BlueWin != nil {
		value := *roflUpload.BlueWin
		blueWin = &value
	}
	if duration == nil && roflUpload != nil && roflUpload.GameLengthMS > 0 {
		value := roflUpload.GameLengthMS
		duration = &value
	}

	players, err := a.normalizePlayers(ctx, event, payload.Players, blueWin)
	if err != nil {
		return MatchDetailView{}, err
	}
	bans, err := a.normalizeBans(event, payload.Bans)
	if err != nil {
		return MatchDetailView{}, err
	}

	found.Team1Side = team1Side
	found.BlueWin = blueWin
	found.DurationMS = duration
	err = a.St.Tx(ctx, func(tx *store.Store) error {
		if err := tx.UpdateMatch(ctx, found); err != nil {
			return err
		}
		if err := tx.ReplacePlayers(ctx, found.ID, players); err != nil {
			return err
		}
		return tx.ReplaceBans(ctx, found.ID, bans)
	})
	if err != nil {
		return MatchDetailView{}, err
	}
	return a.ShowMatch(ctx, event, gameNo, actor)
}

// DeleteMatch 删除一局并重排局号。
func (a *App) DeleteMatch(ctx context.Context, event store.Event, gameNo int, actor store.Member) error {
	if event.CreatorMemberID != actor.ID {
		return ErrForbidden("只有组织者可以记录对局")
	}
	if event.EndedAt != nil {
		return ErrConflict("比赛已结束，不能删除对局记录")
	}
	found, err := a.St.MatchByEventGameNo(ctx, event.ID, gameNo)
	if err != nil {
		if IsNotFound(err) {
			return ErrNotFound("找不到该局对局记录")
		}
		return err
	}
	return a.St.DeleteMatch(ctx, event.ID, found.ID)
}

// ImportRofl 上传并解析录像，返回解析结果与映射建议。
func (a *App) ImportRofl(ctx context.Context, event store.Event, gameNo int, actor store.Member, file UploadedFile) (map[string]any, error) {
	if event.CreatorMemberID != actor.ID {
		return nil, ErrForbidden("只有组织者可以记录对局")
	}
	if event.EndedAt != nil {
		return nil, ErrConflict("比赛已结束，不能再上传录像")
	}
	found, err := a.St.MatchByEventGameNo(ctx, event.ID, gameNo)
	if err != nil {
		if IsNotFound(err) {
			return nil, ErrNotFound("找不到该局对局记录")
		}
		return nil, err
	}

	upload, err := a.importRoflFile(ctx, file, actor)
	if err != nil {
		return nil, err
	}
	if upload.MatchID == nil || *upload.MatchID != found.ID || upload.FileName != file.Name {
		if err := a.St.UpdateRoflOwner(ctx, upload.ID, found.ID, file.Name, file.Size); err != nil {
			return nil, err
		}
		upload.MatchID = &found.ID
		upload.FileName = file.Name
	}

	if err := a.St.SetMatchDefaults(ctx, found.ID, upload.BlueWin, upload.GameLengthMS); err != nil {
		return nil, err
	}
	if err := a.applyRoflPlayers(ctx, found.ID, upload); err != nil {
		return nil, err
	}

	players, _ := roflParsedParts(upload)
	suggestions, err := a.suggestMapping(ctx, found.ID, players)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"rofl":             a.roflView(upload),
		"suggestedMapping": suggestions,
	}, nil
}

// Rate 对某名出场玩家评分（1-5 星）。
func (a *App) Rate(ctx context.Context, event store.Event, gameNo int, actor store.Member, targetMemberID int64, score int) (RatingSummaryView, error) {
	found, err := a.St.MatchByEventGameNo(ctx, event.ID, gameNo)
	if err != nil {
		if IsNotFound(err) {
			return RatingSummaryView{}, ErrNotFound("找不到该局对局记录")
		}
		return RatingSummaryView{}, err
	}
	allowed, err := a.canRate(ctx, event, actor)
	if err != nil {
		return RatingSummaryView{}, err
	}
	if !allowed {
		return RatingSummaryView{}, ErrForbidden("只有报名玩家、观战玩家或组织者可以评分")
	}
	if score < 1 || score > 5 {
		return RatingSummaryView{}, ErrInvalid("评分必须是 1-5 星")
	}
	if targetMemberID == actor.ID {
		return RatingSummaryView{}, ErrInvalid("不能给自己评分")
	}

	players, err := a.St.PlayersByMatch(ctx, found.ID)
	if err != nil {
		return RatingSummaryView{}, err
	}
	targetFound := false
	for _, player := range players {
		if player.MemberID != nil && *player.MemberID == targetMemberID {
			targetFound = true
			break
		}
	}
	if !targetFound {
		return RatingSummaryView{}, ErrNotFound("该玩家不在这局比赛中")
	}

	if err := a.St.UpsertRating(ctx, store.Rating{
		MatchID:        found.ID,
		RaterMemberID:  actor.ID,
		TargetMemberID: targetMemberID,
		Score:          score,
	}); err != nil {
		return RatingSummaryView{}, err
	}

	summary, err := a.ratingSummary(ctx, found.ID, actor.ID)
	if err != nil {
		return RatingSummaryView{}, err
	}
	if values, ok := summary.(map[string]RatingSummaryView); ok {
		if item, exists := values[itoa64(targetMemberID)]; exists {
			return item, nil
		}
	}
	return RatingSummaryView{MemberID: targetMemberID, Average: float64(score), Count: 1, MyScore: &score}, nil
}

// fillPlayersFromRoster 从已套用阵容生成 10 名玩家。
func (a *App) fillPlayersFromRoster(ctx context.Context, event store.Event, m store.Match, roster *RosterPayload) error {
	team1Side := defaultSide(m.Team1Side)
	team2Side := "blue"
	if team1Side == "blue" {
		team2Side = "red"
	}
	sideMap := map[string]string{"team1": team1Side, "team2": team2Side}

	players := make([]store.MatchPlayer, 0, 10)
	for _, team := range []string{"team1", "team2"} {
		slots := roster.Team1
		if team == "team2" {
			slots = roster.Team2
		}
		for index, slot := range slots {
			side := sideMap[team]
			win := false
			if m.BlueWin != nil {
				win = (side == "blue") == *m.BlueWin
			}
			var memberID *int64
			if slot.MemberID > 0 {
				id := slot.MemberID
				memberID = &id
			}
			players = append(players, store.MatchPlayer{
				MatchID:    m.ID,
				Side:       side,
				Position:   slot.Position,
				Slot:       index + 1,
				MemberID:   memberID,
				PlayerName: slot.Name,
				Win:        win,
			})
		}
	}
	return a.St.InsertPlayers(ctx, m.ID, players)
}

// normalizePlayers 校验并归一化玩家列表。
func (a *App) normalizePlayers(ctx context.Context, event store.Event, input []json.RawMessage, blueWin *bool) ([]store.MatchPlayer, error) {
	out := make([]store.MatchPlayer, 0, len(input))
	seenSlots := map[string]bool{}
	seenMembers := map[int64]bool{}

	for _, raw := range input {
		if len(raw) == 0 {
			continue
		}
		var item struct {
			Side              string          `json:"side"`
			Position          string          `json:"position"`
			Slot              int             `json:"slot"`
			MemberID          *int64          `json:"memberId"`
			PlayerName        string          `json:"playerName"`
			Champion          string          `json:"champion"`
			Kills             int             `json:"kills"`
			Deaths            int             `json:"deaths"`
			Assists           int             `json:"assists"`
			CS                int             `json:"cs"`
			Gold              int64           `json:"gold"`
			DamageToChampions int64           `json:"damageToChampions"`
			DamageTaken       int64           `json:"damageTaken"`
			VisionScore       int             `json:"visionScore"`
			Raw               json.RawMessage `json:"raw"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		if item.Side != "blue" && item.Side != "red" {
			return nil, ErrInvalid("队伍必须是蓝方或红方")
		}
		position, ok := rank.ParsePosition(item.Position)
		if !ok {
			return nil, ErrInvalid("玩家位置不合法")
		}
		if item.Slot < 1 || item.Slot > 5 {
			return nil, ErrInvalid("位置槽位必须在 1-5 之间")
		}
		slotKey := item.Side + "-" + itoa(item.Slot)
		if seenSlots[slotKey] {
			return nil, ErrInvalid("同一队不能有重复的槽位")
		}
		seenSlots[slotKey] = true

		var memberID *int64
		var memberNickname string
		if item.MemberID != nil {
			member, err := a.St.MemberByID(ctx, event.GroupID, *item.MemberID)
			if err != nil {
				if IsNotFound(err) {
					return nil, ErrInvalid("玩家不在当前群成员中")
				}
				return nil, err
			}
			if seenMembers[member.ID] {
				return nil, ErrInvalid("同一名玩家不能出现在多个位置")
			}
			seenMembers[member.ID] = true
			id := member.ID
			memberID = &id
			memberNickname = member.Nickname
		}

		playerName := strings.TrimSpace(item.PlayerName)
		if playerName == "" {
			playerName = memberNickname
		}
		if len([]rune(playerName)) > 32 {
			return nil, ErrInvalid("玩家名称最多 32 个字符")
		}
		champion := strings.TrimSpace(item.Champion)
		if len([]rune(champion)) > 64 {
			return nil, ErrInvalid("英雄名称最多 64 个字符")
		}

		win := false
		if blueWin != nil {
			win = (item.Side == "blue") == *blueWin
		}
		out = append(out, store.MatchPlayer{
			Side:              item.Side,
			Position:          string(position),
			Slot:              item.Slot,
			MemberID:          memberID,
			PlayerName:        playerName,
			Champion:          champion,
			Kills:             maxInt(0, item.Kills),
			Deaths:            maxInt(0, item.Deaths),
			Assists:           maxInt(0, item.Assists),
			CS:                maxInt(0, item.CS),
			Gold:              maxInt64(0, item.Gold),
			DamageToChampions: maxInt64(0, item.DamageToChampions),
			DamageTaken:       maxInt64(0, item.DamageTaken),
			VisionScore:       maxInt(0, item.VisionScore),
			Win:               win,
			Raw:               item.Raw,
		})
	}
	return out, nil
}

// normalizeBans 校验并归一化 BP（未开启 BP 时直接清空）。
func (a *App) normalizeBans(event store.Event, input []json.RawMessage) ([]store.MatchBan, error) {
	if !event.HasBP {
		return []store.MatchBan{}, nil
	}
	out := make([]store.MatchBan, 0, len(input))
	seen := map[string]bool{}
	for _, raw := range input {
		var item struct {
			Side     string `json:"side"`
			Slot     int    `json:"slot"`
			Champion string `json:"champion"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		if item.Side != "blue" && item.Side != "red" {
			continue
		}
		champion := strings.TrimSpace(item.Champion)
		if item.Slot < 1 || item.Slot > 5 || champion == "" {
			continue
		}
		key := item.Side + "-" + itoa(item.Slot)
		if seen[key] {
			return nil, ErrInvalid("同一队的 Ban 位不能重复")
		}
		seen[key] = true
		out = append(out, store.MatchBan{Side: item.Side, Slot: item.Slot, Champion: champion})
	}
	return out, nil
}

// importRoflFile 校验、解析并按 sha256 幂等落库。
func (a *App) importRoflFile(ctx context.Context, file UploadedFile, actor store.Member) (*store.RoflUpload, error) {
	maxKB := a.Cfg.RoflMaxKB
	if file.Size > maxKB*1024 {
		return nil, ErrInvalid(fmt.Sprintf("录像文件过大，最大支持 %d KB", maxKB))
	}
	if strings.ToLower(filepath.Ext(file.Name)) != ".rofl" {
		return nil, ErrInvalid("请上传 .rofl 格式的录像文件")
	}

	sum, err := hashFile(file.Path)
	if err != nil {
		return nil, ErrInvalid("无法读取上传的录像文件")
	}
	existing, err := a.St.RoflBySHA(ctx, sum)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	metadata, err := a.Parser.ParseFile(file.Path)
	if err != nil {
		var parseErr *rofl.ParseError
		if errors.As(err, &parseErr) {
			return nil, ErrInvalid(parseErr.Message)
		}
		return nil, ErrInvalid(err.Error())
	}

	parsed, err := json.Marshal(metadata.ToMap())
	if err != nil {
		return nil, err
	}
	upload := store.RoflUpload{
		FileName:          file.Name,
		FileSize:          file.Size,
		SHA256:            sum,
		GameVersion:       metadata.GameVersion,
		DDragonVersion:    metadata.DDragonVersion,
		GameLengthMS:      int64(metadata.GameLengthMS),
		BlueWin:           metadata.BlueWin,
		Parsed:            parsed,
		CreatedByMemberID: &actor.ID,
	}
	id, err := a.St.CreateRofl(ctx, upload)
	if err != nil {
		return nil, err
	}
	upload.ID = id
	return &upload, nil
}

// applyRoflPlayers 把录像中 10 人的战绩按 side + position 写回当前阵容。
func (a *App) applyRoflPlayers(ctx context.Context, matchID int64, upload *store.RoflUpload) error {
	players, _ := roflParsedParts(upload)
	var parsedPlayers []rofl.Player
	if err := json.Unmarshal(players, &parsedPlayers); err != nil {
		return nil
	}
	if len(parsedPlayers) == 0 {
		return nil
	}

	current, err := a.St.PlayersByMatch(ctx, matchID)
	if err != nil {
		return err
	}
	type key struct{ side, position string }
	index := make(map[key]int, len(current))
	for i, player := range current {
		index[key{player.Side, player.Position}] = i
	}

	for _, roflPlayer := range parsedPlayers {
		side := strings.ToLower(roflPlayer.Team)
		position, ok := rank.ParsePosition(roflPlayer.Position)
		if (side != "blue" && side != "red") || !ok {
			continue
		}
		i, found := index[key{side, string(position)}]
		if !found {
			continue
		}
		current[i].Champion = roflPlayer.Champion
		current[i].Kills = roflPlayer.Kills
		current[i].Deaths = roflPlayer.Deaths
		current[i].Assists = roflPlayer.Assists
		current[i].CS = roflPlayer.CS
		current[i].Gold = int64(roflPlayer.Gold)
		current[i].DamageToChampions = int64(roflPlayer.DamageToChampions)
		current[i].DamageTaken = int64(roflPlayer.DamageTaken)
		current[i].VisionScore = roflPlayer.VisionScore
		current[i].Win = roflPlayer.Win
	}
	return a.St.ReplacePlayers(ctx, matchID, current)
}

// suggestMapping 给出录像玩家与当前记录的映射建议。
func (a *App) suggestMapping(ctx context.Context, matchID int64, playersRaw json.RawMessage) ([]map[string]any, error) {
	var parsedPlayers []rofl.Player
	if len(playersRaw) > 0 {
		if err := json.Unmarshal(playersRaw, &parsedPlayers); err != nil {
			return []map[string]any{}, nil
		}
	}
	current, err := a.St.PlayersByMatch(ctx, matchID)
	if err != nil {
		return nil, err
	}

	out := make([]map[string]any, 0, len(parsedPlayers))
	for _, roflPlayer := range parsedPlayers {
		var matched *store.MatchPlayer
		for i := range current {
			player := &current[i]
			if player.Side != roflPlayer.Team {
				continue
			}
			if roflPlayer.Position != "" && player.Position == roflPlayer.Position {
				matched = player
				break
			}
			if roflPlayer.Champion != "" && strings.EqualFold(player.Champion, roflPlayer.Champion) {
				matched = player
			}
		}
		var matchPlayerID, memberID *int64
		if matched != nil {
			id := matched.ID
			matchPlayerID = &id
			memberID = matched.MemberID
		}
		out = append(out, map[string]any{
			"roflPlayer":    roflPlayer,
			"matchPlayerId": matchPlayerID,
			"memberId":      memberID,
			"side":          roflPlayer.Team,
			"position":      roflPlayer.Position,
			"champion":      roflPlayer.Champion,
		})
	}
	return out, nil
}

func (a *App) eventOfMatchView(event store.Event, blueWins, redWins int) EventOfMatchView {
	return EventOfMatchView{
		ID:           event.ID,
		Region:       event.Region,
		BestOf:       event.BestOf,
		HasBP:        event.HasBP,
		WinsRequired: match.WinsRequired(event.BestOf),
		IsEnded:      event.EndedAt != nil,
		EndedAt:      a.isoPtr(event.EndedAt),
		Series:       SeriesMini{BlueWins: blueWins, RedWins: redWins},
	}
}

func (a *App) registrationOptions(registrations []store.Registration) []RegistrationOptionView {
	out := make([]RegistrationOptionView, 0, len(registrations))
	for _, registration := range registrations {
		rankView := a.registrationRankView(registration, false)
		subRank := a.registrationRankView(registration, true)
		tierLabel := rankView.Region + " " + rankView.TierLabel
		if rankView.Division != nil {
			tierLabel += itoa(*rankView.Division)
		} else if rankView.LP > 0 {
			tierLabel += itoa(rankView.LP) + "分"
		}
		out = append(out, RegistrationOptionView{
			RegistrationID: registration.ID,
			MemberID:       registration.MemberID,
			Nickname:       registration.MemberNickname,
			Avatar:         registration.MemberAvatar,
			QQNumber:       maskQQ(registration.MemberQQNumber),
			MainPosition:   registration.MainPosition,
			SubPositions:   nonNil(registration.SubPositions),
			RankLabel:      strings.TrimSpace(tierLabel),
			Rank:           rankView,
			SubRank:        subRank,
		})
	}
	return out
}

func playerView(player store.MatchPlayer) PlayerView {
	var raw json.RawMessage
	if len(player.Raw) > 0 {
		raw = player.Raw
	}
	return PlayerView{
		ID:                player.ID,
		Side:              player.Side,
		Position:          player.Position,
		Slot:              player.Slot,
		MemberID:          player.MemberID,
		MemberNickname:    player.MemberNickname,
		MemberAvatar:      player.MemberAvatar,
		PlayerName:        player.PlayerName,
		Champion:          player.Champion,
		Kills:             player.Kills,
		Deaths:            player.Deaths,
		Assists:           player.Assists,
		CS:                player.CS,
		Gold:              int(player.Gold),
		DamageToChampions: int(player.DamageToChampions),
		DamageTaken:       int(player.DamageTaken),
		VisionScore:       player.VisionScore,
		Win:               player.Win,
		Raw:               raw,
	}
}

func (a *App) roflView(upload *store.RoflUpload) *RoflView {
	if upload == nil {
		return nil
	}
	players, bans := roflParsedParts(upload)
	return &RoflView{
		ID:             upload.ID,
		FileName:       upload.FileName,
		GameVersion:    upload.GameVersion,
		DDragonVersion: upload.DDragonVersion,
		GameLengthMS:   upload.GameLengthMS,
		BlueWin:        upload.BlueWin,
		UploadedAt:     a.isoPtr(&upload.CreatedAt),
		Players:        players,
		Bans:           bans,
	}
}

func roflParsedParts(upload *store.RoflUpload) (json.RawMessage, json.RawMessage) {
	var parsed struct {
		Players json.RawMessage `json:"players"`
		Bans    json.RawMessage `json:"bans"`
	}
	if len(upload.Parsed) > 0 {
		_ = json.Unmarshal(upload.Parsed, &parsed)
	}
	if len(parsed.Players) == 0 {
		parsed.Players = json.RawMessage("[]")
	}
	return parsed.Players, parsed.Bans
}

func teamSeries(matches []store.Match) (int, int) {
	team1Wins, team2Wins := 0, 0
	for _, m := range matches {
		if m.BlueWin == nil {
			continue
		}
		side := defaultSide(m.Team1Side)
		team1Won := (side == "blue" && *m.BlueWin) || (side == "red" && !*m.BlueWin)
		if team1Won {
			team1Wins++
		} else {
			team2Wins++
		}
	}
	return team1Wins, team2Wins
}

func blueRedWins(matches []store.Match) (int, int) {
	blue, red := 0, 0
	for _, m := range matches {
		if m.BlueWin == nil {
			continue
		}
		if *m.BlueWin {
			blue++
		} else {
			red++
		}
	}
	return blue, red
}

func defaultSide(side string) string {
	if side == "red" {
		return "red"
	}
	return "blue"
}

func parseNullableBool(raw json.RawMessage, message string) (*bool, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, ErrInvalid(message)
	}
	if value == nil {
		return nil, nil
	}
	b, ok := value.(bool)
	if !ok {
		return nil, ErrInvalid(message)
	}
	return &b, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func mustJSON(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func itoa64(v int64) string { return itoa(int(v)) }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
