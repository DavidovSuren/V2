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
}

func (s *Store) UsersPendingToday(today string) ([]PendingUser, error) {
	rows, err := s.DB.Query(`
		SELECT id, tg_id, day_index FROM users
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
		if err := rows.Scan(&u.ID, &u.TgID, &u.DayIndex); err != nil {
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
