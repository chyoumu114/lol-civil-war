package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// AddAllowedGroup 添加数据库白名单群；重复添加只更新备注。
func (s *Store) AddAllowedGroup(ctx context.Context, number, note string) error {
	now := millis(time.Now().UTC())
	var noteValue any
	if note != "" {
		noteValue = note
	}
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO allowed_groups (qq_group_number, note, created_at)
		VALUES (?, ?, ?) AS new
		ON DUPLICATE KEY UPDATE note = new.note`,
		number, noteValue, now)
	return err
}

// RemoveAllowedGroup 删除数据库白名单群；返回是否真的删除了记录。
func (s *Store) RemoveAllowedGroup(ctx context.Context, number string) (bool, error) {
	res, err := s.conn().ExecContext(ctx,
		`DELETE FROM allowed_groups WHERE qq_group_number = ?`, number)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// AllowedGroupNumbers 返回数据库白名单（按添加时间升序）。
func (s *Store) AllowedGroupNumbers(ctx context.Context) ([]string, error) {
	rows, err := s.conn().QueryContext(ctx,
		`SELECT qq_group_number FROM allowed_groups ORDER BY created_at ASC, qq_group_number ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0, 8)
	for rows.Next() {
		var number string
		if err := rows.Scan(&number); err != nil {
			return nil, err
		}
		out = append(out, number)
	}
	return out, rows.Err()
}

// IsGroupAllowed 报告群号是否在数据库白名单中。
func (s *Store) IsGroupAllowed(ctx context.Context, number string) (bool, error) {
	var one int
	err := s.conn().QueryRowContext(ctx,
		`SELECT 1 FROM allowed_groups WHERE qq_group_number = ? LIMIT 1`, number).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return one == 1, nil
}
