package store

import "database/sql"

func (s *Store) ActionCounts(userID int64, from string) (done, skipped int, err error) {
	rows, err := s.DB.Query(`
		SELECT action, COUNT(*)::int as n FROM action_log
		WHERE user_id = $1 AND action_date >= $2 GROUP BY action
	`, userID, from)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var action string
		var n int
		if err := rows.Scan(&action, &n); err != nil {
			return 0, 0, err
		}
		if action == "done" {
			done = n
		} else if action == "skip" {
			skipped = n
		}
	}
	return done, skipped, rows.Err()
}

func (s *Store) TopDoneCategory(userID int64, from string) (string, error) {
	var category sql.NullString
	err := s.DB.QueryRow(`
		SELECT category FROM action_log
		WHERE user_id = $1 AND action_date >= $2 AND action = 'done'
		GROUP BY category ORDER BY COUNT(*) DESC LIMIT 1
	`, userID, from).Scan(&category)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return category.String, nil
}

func (s *Store) TopWeightedRemainingCategory(userID int64) (string, error) {
	var category sql.NullString
	err := s.DB.QueryRow(`
		SELECT cw.category FROM category_weights cw
		WHERE cw.user_id = $1 AND EXISTS (
			SELECT 1 FROM user_schedule us
			WHERE us.user_id = cw.user_id AND us.category = cw.category AND us.status = 'pending'
		)
		ORDER BY cw.weight DESC LIMIT 1
	`, userID).Scan(&category)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return category.String, nil
}

type CategoryCount struct {
	Category string
	N        int
}

func (s *Store) CategoryBreakdown(userID int64, from string) ([]CategoryCount, error) {
	rows, err := s.DB.Query(`
		SELECT category, COUNT(*)::int as n FROM action_log
		WHERE user_id = $1 AND action_date >= $2 AND action = 'done'
		GROUP BY category ORDER BY n DESC
	`, userID, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CategoryCount
	for rows.Next() {
		var c CategoryCount
		if err := rows.Scan(&c.Category, &c.N); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type MoodRow struct {
	EntryDate string
	Emoji     string
}

func (s *Store) MoodRows(userID int64, from string) ([]MoodRow, error) {
	rows, err := s.DB.Query(`
		SELECT entry_date, emoji FROM diary_entries
		WHERE user_id = $1 AND entry_date >= $2 ORDER BY entry_date ASC
	`, userID, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MoodRow
	for rows.Next() {
		var m MoodRow
		if err := rows.Scan(&m.EntryDate, &m.Emoji); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

type PendingUser struct {
	ID       int64
	TgID     string
	DayIndex int
	RemindAt string // "HH:MM"; пусто — время по умолчанию
	Streak   int
}

func (s *Store) UsersPendingToday(today string) ([]PendingUser, error) {
	rows, err := s.DB.Query(`
		SELECT id, tg_id, day_index, COALESCE(remind_at, ''), streak_current FROM users
		WHERE gender IS NOT NULL AND day_index < 365
		  AND (last_action_date IS NULL OR last_action_date != $1)
	`, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PendingUser
	for rows.Next() {
		var u PendingUser
		if err := rows.Scan(&u.ID, &u.TgID, &u.DayIndex, &u.RemindAt, &u.Streak); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) AddXP(userID int64, amount int) error {
	_, err := s.DB.Exec("UPDATE users SET xp = xp + $1 WHERE id = $2", amount, userID)
	return err
}

func (s *Store) AllQuizzedUsers() ([]struct {
	ID   int64
	TgID string
}, error) {
	rows, err := s.DB.Query("SELECT id, tg_id FROM users WHERE gender IS NOT NULL")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []struct {
		ID   int64
		TgID string
	}
	for rows.Next() {
		var r struct {
			ID   int64
			TgID string
		}
		if err := rows.Scan(&r.ID, &r.TgID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type RecentUser struct {
	ID        int64
	TgID      string
	CreatedAt string
	Tier      string
	ExpiresAt sql.NullString
}

// UsersRegisteredSince — пользователи, зарегистрированные не раньше since
// (RFC3339 UTC; created_at хранится в том же формате, сравнение строковое).
func (s *Store) UsersRegisteredSince(since string) ([]RecentUser, error) {
	rows, err := s.DB.Query(`
		SELECT id, tg_id, created_at, subscription_tier, subscription_expires_at
		FROM users WHERE created_at >= $1
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RecentUser
	for rows.Next() {
		var u RecentUser
		if err := rows.Scan(&u.ID, &u.TgID, &u.CreatedAt, &u.Tier, &u.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ActionsSince — действия пользователя (done/skip) по датам начиная с since (YYYY-MM-DD).
func (s *Store) ActionsSince(userID int64, since string) (map[string]string, error) {
	rows, err := s.DB.Query(`SELECT action_date, action FROM action_log WHERE user_id = $1 AND action_date >= $2`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var date, action string
		if err := rows.Scan(&date, &action); err != nil {
			return nil, err
		}
		out[date] = action
	}
	return out, rows.Err()
}

// WeekDoneCounts — сколько заданий каждый пользователь выполнил начиная с since.
func (s *Store) WeekDoneCounts(since string) (map[int64]int, error) {
	rows, err := s.DB.Query(`SELECT user_id, COUNT(*)::int FROM action_log WHERE action = 'done' AND action_date >= $1 GROUP BY user_id`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}
