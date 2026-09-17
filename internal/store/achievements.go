package store

func (s *Store) HasAchievement(userID int64, code string) (bool, error) {
	var exists bool
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM achievements WHERE user_id = $1 AND code = $2)", userID, code).Scan(&exists)
	return exists, err
}

func (s *Store) InsertAchievement(userID int64, code, unlockedAt string) error {
	_, err := s.DB.Exec(`
		INSERT INTO achievements (user_id, code, unlocked_at) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, code) DO NOTHING
	`, userID, code, unlockedAt)
	return err
}

type AchievementRow struct {
	Code       string
	UnlockedAt string
}

func (s *Store) UserAchievements(userID int64) ([]AchievementRow, error) {
	rows, err := s.DB.Query("SELECT code, unlocked_at FROM achievements WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AchievementRow
	for rows.Next() {
		var a AchievementRow
		if err := rows.Scan(&a.Code, &a.UnlockedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
