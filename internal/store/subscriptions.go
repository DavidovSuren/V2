package store

import "database/sql"

func (s *Store) InsertSubscriptionPayment(q Queryer, userID int64, tier string, basePrice, discountPct, pricePaid int, referrerCommissionUserID sql.NullInt64, commissionAmount int, paidAt string) error {
	_, err := q.Exec(`
		INSERT INTO subscription_payments
			(user_id, tier, base_price, discount_pct, price_paid, referrer_commission_user_id, commission_amount, paid_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, userID, tier, basePrice, discountPct, pricePaid, referrerCommissionUserID, commissionAmount, paidAt)
	return err
}

func (s *Store) UpdateSubscription(q Queryer, userID int64, tier, expiresAt string) error {
	_, err := q.Exec("UPDATE users SET subscription_tier = $1, subscription_expires_at = $2 WHERE id = $3",
		tier, expiresAt, userID)
	return err
}

func (s *Store) InsertWalletTransaction(q Queryer, userID int64, amount int, sourceUserID int64, note, createdAt string) error {
	_, err := q.Exec(`
		INSERT INTO wallet_transactions (user_id, amount, source_user_id, note, created_at) VALUES ($1, $2, $3, $4, $5)
	`, userID, amount, sourceUserID, note, createdAt)
	return err
}

func (s *Store) WalletBalance(userID int64) (int, error) {
	var total int
	err := s.DB.QueryRow(`SELECT COALESCE(SUM(amount), 0)::int FROM wallet_transactions WHERE user_id = $1`, userID).Scan(&total)
	return total, err
}

func (s *Store) PayingReferralsCount(userID int64) (int, error) {
	var count int
	err := s.DB.QueryRow(`SELECT COUNT(DISTINCT user_id)::int FROM subscription_payments WHERE referrer_commission_user_id = $1`, userID).Scan(&count)
	return count, err
}

type WalletTx struct {
	Amount    int
	Note      sql.NullString
	CreatedAt string
}

func (s *Store) WalletHistory(userID int64) ([]WalletTx, error) {
	rows, err := s.DB.Query(`
		SELECT amount, note, created_at FROM wallet_transactions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 50
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WalletTx
	for rows.Next() {
		var w WalletTx
		if err := rows.Scan(&w.Amount, &w.Note, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
