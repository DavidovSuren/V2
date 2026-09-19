package store

func (s *Store) HasRedeemedPromo(userID int64, code string) (bool, error) {
	var exists bool
	err := s.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM promo_redemptions WHERE user_id = $1 AND code = $2)",
		userID, code,
	).Scan(&exists)
	return exists, err
}

func (s *Store) RedeemPromo(q Queryer, userID int64, code, redeemedAt string) error {
	_, err := q.Exec(
		"INSERT INTO promo_redemptions (user_id, code, redeemed_at) VALUES ($1, $2, $3)",
		userID, code, redeemedAt,
	)
	return err
}

// MarkAllScheduleDone — используется промокодом на 100 уровень: без этого
// экран "Прогресс" показывал бы 0% по категориям при уровне 100.
func (s *Store) MarkAllScheduleDone(q Queryer, userID int64) error {
	_, err := q.Exec("UPDATE user_schedule SET status = 'done' WHERE user_id = $1", userID)
	return err
}
