package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const matchSelect = `
	SELECT id, event_id, game_no, team1_side, blue_win, duration_ms, status, submitted_at,
	       created_by_member_id, created_at, updated_at
	FROM matches
`

// CreateMatch 新建一局。
func (s *Store) CreateMatch(ctx context.Context, m Match) (Match, error) {
	now := time.Now().UTC()
	res, err := s.conn().ExecContext(ctx, `
		INSERT INTO matches (event_id, game_no, team1_side, blue_win, duration_ms, status,
			submitted_at, created_by_member_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.EventID, m.GameNo, m.Team1Side, nullBool(m.BlueWin), nullInt64(m.DurationMS), m.Status,
		nullTime(m.SubmittedAt), nullInt64(m.CreatedByMemberID), millis(now), millis(now))
	if err != nil {
		return Match{}, fmt.Errorf("创建对局失败: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Match{}, err
	}
	return s.MatchByID(ctx, id)
}

// MatchByID 按主键查询对局。
func (s *Store) MatchByID(ctx context.Context, id int64) (Match, error) {
	row := s.conn().QueryRowContext(ctx, matchSelect+` WHERE id = ?`, id)
	return scanMatch(row)
}

// MatchByEventGameNo 按组织 + 局号查询。
func (s *Store) MatchByEventGameNo(ctx context.Context, eventID int64, gameNo int) (Match, error) {
	row := s.conn().QueryRowContext(ctx, matchSelect+` WHERE event_id = ? AND game_no = ?`, eventID, gameNo)
	return scanMatch(row)
}

// MatchesByEvent 查询组织的全部对局（局号升序）。
func (s *Store) MatchesByEvent(ctx context.Context, eventID int64) ([]Match, error) {
	rows, err := s.conn().QueryContext(ctx, matchSelect+` WHERE event_id = ? ORDER BY game_no ASC`, eventID)
	if err != nil {
		return nil, fmt.Errorf("查询对局失败: %w", err)
	}
	defer rows.Close()
	out := make([]Match, 0, 4)
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpdateMatch 保存对局结果与 1 队所在方。
func (s *Store) UpdateMatch(ctx context.Context, m Match) error {
	res, err := s.conn().ExecContext(ctx, `
		UPDATE matches SET team1_side = ?, blue_win = ?, duration_ms = ?, updated_at = ?
		WHERE id = ?`,
		m.Team1Side, nullBool(m.BlueWin), nullInt64(m.DurationMS), millis(time.Now().UTC()), m.ID)
	if err != nil {
		return fmt.Errorf("保存对局失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetMatchDefaults 只在字段为空时回填（用于录像解析结果）。
func (s *Store) SetMatchDefaults(ctx context.Context, matchID int64, blueWin *bool, durationMS int64) error {
	_, err := s.conn().ExecContext(ctx, `
		UPDATE matches SET
			blue_win = COALESCE(blue_win, ?),
			duration_ms = CASE WHEN duration_ms IS NULL AND ? > 0 THEN ? ELSE duration_ms END,
			updated_at = ?
		WHERE id = ?`,
		nullBool(blueWin), durationMS, durationMS, millis(time.Now().UTC()), matchID)
	return err
}

// DeleteMatch 删除一局及其所有子记录，并把剩余局号重排为 1..N。
func (s *Store) DeleteMatch(ctx context.Context, eventID, matchID int64) error {
	return s.Tx(ctx, func(tx *Store) error {
		if _, err := tx.conn().ExecContext(ctx, `DELETE FROM match_player_ratings WHERE match_id = ?`, matchID); err != nil {
			return err
		}
		if _, err := tx.conn().ExecContext(ctx, `DELETE FROM match_bans WHERE match_id = ?`, matchID); err != nil {
			return err
		}
		if _, err := tx.conn().ExecContext(ctx, `DELETE FROM match_players WHERE match_id = ?`, matchID); err != nil {
			return err
		}
		if _, err := tx.conn().ExecContext(ctx, `DELETE FROM rofl_uploads WHERE match_id = ?`, matchID); err != nil {
			return err
		}
		if _, err := tx.conn().ExecContext(ctx, `DELETE FROM matches WHERE id = ?`, matchID); err != nil {
			return err
		}
		rows, err := tx.conn().QueryContext(ctx,
			`SELECT id FROM matches WHERE event_id = ? ORDER BY game_no ASC`, eventID)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, 4)
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		for index, id := range ids {
			if _, err := tx.conn().ExecContext(ctx,
				`UPDATE matches SET game_no = ? WHERE id = ?`, index+1, id); err != nil {
				return err
			}
		}
		return nil
	})
}

// PlayersByMatch 查询一局内的全部玩家（蓝方在前，槽位升序）。
func (s *Store) PlayersByMatch(ctx context.Context, matchID int64) ([]MatchPlayer, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT p.id, p.match_id, p.side, p.position, p.slot, p.member_id, p.player_name, p.champion,
		       p.kills, p.deaths, p.assists, p.cs, p.gold, p.damage_to_champions, p.damage_taken,
		       p.vision_score, p.win, p.raw, p.created_at, p.updated_at, gm.nickname, u.avatar
		FROM match_players p
		LEFT JOIN group_members gm ON gm.id = p.member_id
		LEFT JOIN users u ON u.id = gm.user_id
		WHERE p.match_id = ?
		ORDER BY p.side ASC, p.slot ASC`, matchID)
	if err != nil {
		return nil, fmt.Errorf("查询对局玩家失败: %w", err)
	}
	defer rows.Close()

	out := make([]MatchPlayer, 0, 10)
	for rows.Next() {
		var (
			p        MatchPlayer
			memberID sql.NullInt64
			raw      sql.NullString
			win      int
			created  int64
			updated  int64
			nickname sql.NullString
			avatar   sql.NullString
		)
		err := rows.Scan(&p.ID, &p.MatchID, &p.Side, &p.Position, &p.Slot, &memberID, &p.PlayerName,
			&p.Champion, &p.Kills, &p.Deaths, &p.Assists, &p.CS, &p.Gold, &p.DamageToChampions,
			&p.DamageTaken, &p.VisionScore, &win, &raw, &created, &updated, &nickname, &avatar)
		if err != nil {
			return nil, fmt.Errorf("读取对局玩家失败: %w", err)
		}
		if memberID.Valid {
			id := memberID.Int64
			p.MemberID = &id
		}
		p.Win = win == 1
		if raw.Valid && raw.String != "" {
			p.Raw = json.RawMessage(raw.String)
		}
		p.CreatedAt = fromMillis(created)
		p.UpdatedAt = fromMillis(updated)
		p.MemberNickname = stringPtr(nickname)
		p.MemberAvatar = stringPtr(avatar)
		out = append(out, p)
	}
	return out, rows.Err()
}

// CountPlayers 统计一局的玩家数。
func (s *Store) CountPlayers(ctx context.Context, matchID int64) (int, error) {
	var count int
	err := s.conn().QueryRowContext(ctx,
		`SELECT count(*) FROM match_players WHERE match_id = ?`, matchID).Scan(&count)
	return count, err
}

// ReplacePlayers 全量覆盖一局的玩家。
func (s *Store) ReplacePlayers(ctx context.Context, matchID int64, players []MatchPlayer) error {
	return s.Tx(ctx, func(tx *Store) error {
		if _, err := tx.conn().ExecContext(ctx, `DELETE FROM match_players WHERE match_id = ?`, matchID); err != nil {
			return err
		}
		return tx.insertPlayers(ctx, matchID, players)
	})
}

// InsertPlayers 追加玩家记录。
func (s *Store) InsertPlayers(ctx context.Context, matchID int64, players []MatchPlayer) error {
	return s.insertPlayers(ctx, matchID, players)
}

func (s *Store) insertPlayers(ctx context.Context, matchID int64, players []MatchPlayer) error {
	now := millis(time.Now().UTC())
	for _, p := range players {
		if _, err := s.conn().ExecContext(ctx, `
			INSERT INTO match_players (match_id, side, position, slot, member_id, player_name, champion,
				kills, deaths, assists, cs, gold, damage_to_champions, damage_taken, vision_score,
				win, raw, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			matchID, p.Side, p.Position, p.Slot, nullInt64(p.MemberID), p.PlayerName, p.Champion,
			p.Kills, p.Deaths, p.Assists, p.CS, p.Gold, p.DamageToChampions, p.DamageTaken,
			p.VisionScore, boolInt(p.Win), rawOrNil(p.Raw), now, now); err != nil {
			return fmt.Errorf("保存对局玩家失败: %w", err)
		}
	}
	return nil
}

// BansByMatch 查询 BP 记录。
func (s *Store) BansByMatch(ctx context.Context, matchID int64) ([]MatchBan, error) {
	rows, err := s.conn().QueryContext(ctx,
		`SELECT id, match_id, side, slot, champion, created_at, updated_at
		 FROM match_bans WHERE match_id = ? ORDER BY side ASC, slot ASC`, matchID)
	if err != nil {
		return nil, fmt.Errorf("查询 BP 失败: %w", err)
	}
	defer rows.Close()
	out := make([]MatchBan, 0, 10)
	for rows.Next() {
		var (
			b              MatchBan
			created, upded int64
		)
		if err := rows.Scan(&b.ID, &b.MatchID, &b.Side, &b.Slot, &b.Champion, &created, &upded); err != nil {
			return nil, err
		}
		b.CreatedAt = fromMillis(created)
		b.UpdatedAt = fromMillis(upded)
		out = append(out, b)
	}
	return out, rows.Err()
}

// ReplaceBans 全量覆盖一局的 BP。
func (s *Store) ReplaceBans(ctx context.Context, matchID int64, bans []MatchBan) error {
	return s.Tx(ctx, func(tx *Store) error {
		if _, err := tx.conn().ExecContext(ctx, `DELETE FROM match_bans WHERE match_id = ?`, matchID); err != nil {
			return err
		}
		now := millis(time.Now().UTC())
		for _, b := range bans {
			if _, err := tx.conn().ExecContext(ctx, `
				INSERT INTO match_bans (match_id, side, slot, champion, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?)`, matchID, b.Side, b.Slot, b.Champion, now, now); err != nil {
				return fmt.Errorf("保存 BP 失败: %w", err)
			}
		}
		return nil
	})
}

// UpsertRating 写入/覆盖评分。
func (s *Store) UpsertRating(ctx context.Context, r Rating) error {
	now := time.Now().UTC()
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO match_player_ratings (match_id, rater_member_id, target_member_id, score, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(match_id, rater_member_id, target_member_id) DO UPDATE SET score = excluded.score, updated_at = excluded.updated_at`,
		r.MatchID, r.RaterMemberID, r.TargetMemberID, r.Score, millis(now), millis(now))
	if err != nil {
		return fmt.Errorf("保存评分失败: %w", err)
	}
	return nil
}

// RatingsByMatch 查询一局的全部评分。
func (s *Store) RatingsByMatch(ctx context.Context, matchID int64) ([]Rating, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT id, match_id, rater_member_id, target_member_id, score, created_at, updated_at
		FROM match_player_ratings WHERE match_id = ?`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Rating, 0, 20)
	for rows.Next() {
		var (
			r              Rating
			created, upded int64
		)
		if err := rows.Scan(&r.ID, &r.MatchID, &r.RaterMemberID, &r.TargetMemberID, &r.Score, &created, &upded); err != nil {
			return nil, err
		}
		r.CreatedAt = fromMillis(created)
		r.UpdatedAt = fromMillis(upded)
		out = append(out, r)
	}
	return out, rows.Err()
}

// RoflBySHA 按文件摘要查询已解析录像。
func (s *Store) RoflBySHA(ctx context.Context, sha string) (*RoflUpload, error) {
	row := s.conn().QueryRowContext(ctx, `
		SELECT id, match_id, file_name, file_size, sha256, game_version, ddragon_version,
		       game_length_ms, blue_win, parsed, created_by_member_id, created_at, updated_at
		FROM rofl_uploads WHERE sha256 = ?`, sha)
	return scanRofl(row)
}

// RoflByMatch 查询某局关联的录像。
func (s *Store) RoflByMatch(ctx context.Context, matchID int64) (*RoflUpload, error) {
	row := s.conn().QueryRowContext(ctx, `
		SELECT id, match_id, file_name, file_size, sha256, game_version, ddragon_version,
		       game_length_ms, blue_win, parsed, created_by_member_id, created_at, updated_at
		FROM rofl_uploads WHERE match_id = ? ORDER BY id DESC LIMIT 1`, matchID)
	return scanRofl(row)
}

// CreateRofl 保存解析结果。
func (s *Store) CreateRofl(ctx context.Context, r RoflUpload) (int64, error) {
	now := time.Now().UTC()
	res, err := s.conn().ExecContext(ctx, `
		INSERT INTO rofl_uploads (match_id, file_name, file_size, sha256, game_version, ddragon_version,
			game_length_ms, blue_win, parsed, created_by_member_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt64(r.MatchID), r.FileName, r.FileSize, r.SHA256, r.GameVersion, r.DDragonVersion,
		r.GameLengthMS, nullBool(r.BlueWin), rawOrNil(r.Parsed), nullInt64(r.CreatedByMemberID),
		millis(now), millis(now))
	if err != nil {
		return 0, fmt.Errorf("保存录像失败: %w", err)
	}
	return res.LastInsertId()
}

// UpdateRoflOwner 把已有录像归属到当前局并更新文件名/大小。
func (s *Store) UpdateRoflOwner(ctx context.Context, id, matchID int64, fileName string, fileSize int64) error {
	_, err := s.conn().ExecContext(ctx, `
		UPDATE rofl_uploads SET match_id = ?, file_name = ?, file_size = ?, updated_at = ?
		WHERE id = ?`, matchID, fileName, fileSize, millis(time.Now().UTC()), id)
	return err
}

func scanMatch(row interface{ Scan(...any) error }) (Match, error) {
	var (
		m         Match
		blueWin   sql.NullInt64
		duration  sql.NullInt64
		submitted sql.NullInt64
		createdBy sql.NullInt64
		created   int64
		updated   int64
	)
	err := row.Scan(&m.ID, &m.EventID, &m.GameNo, &m.Team1Side, &blueWin, &duration, &m.Status,
		&submitted, &createdBy, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Match{}, ErrNotFound
	}
	if err != nil {
		return Match{}, fmt.Errorf("读取对局失败: %w", err)
	}
	if blueWin.Valid {
		v := blueWin.Int64 == 1
		m.BlueWin = &v
	}
	if duration.Valid {
		v := duration.Int64
		m.DurationMS = &v
	}
	m.SubmittedAt = nullTimePtr(submitted)
	if createdBy.Valid {
		v := createdBy.Int64
		m.CreatedByMemberID = &v
	}
	m.CreatedAt = fromMillis(created)
	m.UpdatedAt = fromMillis(updated)
	return m, nil
}

func scanRofl(row interface{ Scan(...any) error }) (*RoflUpload, error) {
	var (
		r         RoflUpload
		matchID   sql.NullInt64
		blueWin   sql.NullInt64
		parsed    sql.NullString
		createdBy sql.NullInt64
		created   int64
		updated   int64
	)
	err := row.Scan(&r.ID, &matchID, &r.FileName, &r.FileSize, &r.SHA256, &r.GameVersion,
		&r.DDragonVersion, &r.GameLengthMS, &blueWin, &parsed, &createdBy, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取录像失败: %w", err)
	}
	if matchID.Valid {
		v := matchID.Int64
		r.MatchID = &v
	}
	if blueWin.Valid {
		v := blueWin.Int64 == 1
		r.BlueWin = &v
	}
	if createdBy.Valid {
		v := createdBy.Int64
		r.CreatedByMemberID = &v
	}
	if parsed.Valid && parsed.String != "" {
		r.Parsed = json.RawMessage(parsed.String)
	}
	r.CreatedAt = fromMillis(created)
	r.UpdatedAt = fromMillis(updated)
	return &r, nil
}

func nullBool(v *bool) any {
	if v == nil {
		return nil
	}
	return boolInt(*v)
}

func nullInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func rawOrNil(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}
