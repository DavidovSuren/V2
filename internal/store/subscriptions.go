package store

import "database/sql"

// Payment — запись об оплате подписки. ChargeID пустой — платёж без
// идентификатора Telegram (в БД NULL, уникальность не проверяется).
type Payment struct {
	UserID           int64
	Tier             string
	Price            int
	ReferrerID       sql.NullInt64 // кому начислена комиссия
	CommissionPct    int
	CommissionAmount int
	ChargeID         string
	PaidAt           string
}

func (s *Store) InsertSubscriptionPayment(q Queryer, p Payment) error {
	_, err := q.Exec(`
		INSERT INTO subscription_payments
			(user_id, tier, base_price, discount_pct, price_paid, referrer_commission_user_id,
			 commission_pct, commission_amount, charge_id, paid_at)
		VALUES ($1, $2, $3, 0, $3, $4, $5, $6, NULLIF($7, ''), $8)
	`, p.UserID, p.Tier, p.Price, p.ReferrerID, p.CommissionPct, p.CommissionAmount, p.ChargeID, p.PaidAt)
	return err
}

// PaymentExists — оплата с таким charge_id уже проведена.
func (s *Store) PaymentExists(q Queryer, chargeID string) (bool, error) {
	var exists bool
	err := q.QueryRow(`SELECT EXISTS(SELECT 1 FROM subscription_payments WHERE charge_id = $1)`, chargeID).Scan(&exists)
	return exists, err
}

// PriorPayingReferralsCount — сколько разных приглашённых referrerID
// (users.referred_by_user_id) хотя бы раз оплачивали подписку, не считая excludeUserID.
func (s *Store) PriorPayingReferralsCount(q Queryer, referrerID, excludeUserID int64) (int, error) {
	var n int
	err := q.QueryRow(`
		SELECT COUNT(DISTINCT u.id)::int FROM users u
		WHERE u.referred_by_user_id = $1 AND u.id <> $2
		  AND EXISTS (SELECT 1 FROM subscription_payments p WHERE p.user_id = u.id)
	`, referrerID, excludeUserID).Scan(&n)
	return n, err
}

// SubscriptionState — то, что нужно для продления и начисления комиссии;
// строка пользователя блокируется до конца транзакции (FOR UPDATE).
type SubscriptionState struct {
	Name       string
	Tier       string
	ExpiresAt  sql.NullString
	ReferrerID sql.NullInt64
}

func (s *Store) LockSubscriptionState(q Queryer, userID int64) (SubscriptionState, error) {
	var st SubscriptionState
	err := q.QueryRow(`
		SELECT name, subscription_tier, subscription_expires_at, referred_by_user_id
		FROM users WHERE id = $1 FOR UPDATE
	`, userID).Scan(&st.Name, &st.Tier, &st.ExpiresAt, &st.ReferrerID)
	return st, err
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

// PayingReferralsCount — сколько приглашённых пользователя оплачивали подписку.
func (s *Store) PayingReferralsCount(userID int64) (int, error) {
	return s.PriorPayingReferralsCount(s.DB, userID, 0)
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
