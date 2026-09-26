package store

import (
	"database/sql"
	"errors"
)

// Виды записей кошелька (wallet_transactions.kind).
const (
	TxIncome     = "income"     // партнёрский доход
	TxWithdrawal = "withdrawal" // вывод на карту (сумма отрицательная)
	TxRefund     = "refund"     // возврат отклонённого вывода
)

// WalletSummary — цифры для экрана кошелька.
type WalletSummary struct {
	Balance   int
	Earned    int // всего заработано (партнёрский доход)
	Withdrawn int // выведено: заявки в обработке и выплаченные
}

func (s *Store) WalletSummary(userID int64) (WalletSummary, error) {
	var w WalletSummary
	err := s.DB.QueryRow(`
		SELECT
		  COALESCE((SELECT SUM(amount) FROM wallet_transactions WHERE user_id = $1), 0)::int,
		  COALESCE((SELECT SUM(amount) FROM wallet_transactions WHERE user_id = $1 AND kind = 'income'), 0)::int,
		  COALESCE((SELECT SUM(gross) FROM withdrawals WHERE user_id = $1 AND status IN ('pending', 'paid')), 0)::int
	`, userID).Scan(&w.Balance, &w.Earned, &w.Withdrawn)
	return w, err
}

// LockWalletBalance — баланс под блокировкой строки пользователя (FOR UPDATE),
// чтобы два одновременных вывода не списали деньги дважды.
func (s *Store) LockWalletBalance(q Queryer, userID int64) (int, error) {
	var id int64
	if err := q.QueryRow(`SELECT id FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&id); err != nil {
		return 0, err
	}
	var balance int
	err := q.QueryRow(`SELECT COALESCE(SUM(amount), 0)::int FROM wallet_transactions WHERE user_id = $1`, userID).Scan(&balance)
	return balance, err
}

// HasWithdrawalInMonth — заявка в этом месяце уже есть (отклонённые не считаются).
func (s *Store) HasWithdrawalInMonth(q Queryer, userID int64, monthKey string) (bool, error) {
	var exists bool
	err := q.QueryRow(`SELECT EXISTS(SELECT 1 FROM withdrawals WHERE user_id = $1 AND month_key = $2 AND status <> 'rejected')`,
		userID, monthKey).Scan(&exists)
	return exists, err
}

type NewWithdrawal struct {
	UserID    int64
	MonthKey  string
	Gross     int
	Tax       int
	Net       int
	TaxPct    int
	CardLast4 string
	CardEnc   string
	CreatedAt string
}

// CreateWithdrawal — заявка и списание -gross из кошелька (в транзакции q).
func (s *Store) CreateWithdrawal(q Queryer, w NewWithdrawal) (int64, error) {
	var id int64
	err := q.QueryRow(`
		INSERT INTO withdrawals (user_id, month_key, gross, tax, net, tax_pct, card_last4, card_enc, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending', $9) RETURNING id
	`, w.UserID, w.MonthKey, w.Gross, w.Tax, w.Net, w.TaxPct, w.CardLast4, w.CardEnc, w.CreatedAt).Scan(&id)
	if err != nil {
		return 0, err
	}
	_, err = q.Exec(`
		INSERT INTO wallet_transactions (user_id, amount, note, created_at, kind, withdrawal_id)
		VALUES ($1, $2, $3, $4, 'withdrawal', $5)
	`, w.UserID, -w.Gross, "Вывод на карту •••• "+w.CardLast4, w.CreatedAt, id)
	return id, err
}

// SavePayoutCard — карта запоминается (зашифрованной) для следующего вывода.
func (s *Store) SavePayoutCard(q Queryer, userID int64, enc, last4 string) error {
	_, err := q.Exec(`UPDATE users SET payout_card_enc = $1, payout_card_last4 = $2 WHERE id = $3`, enc, last4, userID)
	return err
}

func (s *Store) PayoutCard(userID int64) (enc, last4 string, err error) {
	var e, l sql.NullString
	err = s.DB.QueryRow(`SELECT payout_card_enc, payout_card_last4 FROM users WHERE id = $1`, userID).Scan(&e, &l)
	return e.String, l.String, err
}

type Withdrawal struct {
	ID          int64
	UserID      int64
	Name        string
	Username    sql.NullString
	TgID        string
	MonthKey    string
	Gross       int
	Tax         int
	Net         int
	TaxPct      int
	CardLast4   string
	CardEnc     string
	Status      string
	CreatedAt   string
	ProcessedAt sql.NullString
	Reason      sql.NullString
}

const withdrawalColumns = `w.id, w.user_id, u.name, u.username, u.tg_id, w.month_key, w.gross, w.tax, w.net, w.tax_pct,
	w.card_last4, w.card_enc, w.status, w.created_at, w.processed_at, w.reject_reason`

func scanWithdrawals(rows *sql.Rows) ([]Withdrawal, error) {
	defer rows.Close()
	var out []Withdrawal
	for rows.Next() {
		var w Withdrawal
		if err := rows.Scan(&w.ID, &w.UserID, &w.Name, &w.Username, &w.TgID, &w.MonthKey, &w.Gross, &w.Tax, &w.Net, &w.TaxPct,
			&w.CardLast4, &w.CardEnc, &w.Status, &w.CreatedAt, &w.ProcessedAt, &w.Reason); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// Withdrawals — заявки со статусом status (пусто — все) за месяц monthKey (пусто — все).
func (s *Store) Withdrawals(status, monthKey string) ([]Withdrawal, error) {
	rows, err := s.DB.Query(`
		SELECT `+withdrawalColumns+` FROM withdrawals w JOIN users u ON u.id = w.user_id
		WHERE ($1 = '' OR w.status = $1) AND ($2 = '' OR w.month_key = $2)
		ORDER BY w.id
	`, status, monthKey)
	if err != nil {
		return nil, err
	}
	return scanWithdrawals(rows)
}

var ErrWithdrawalProcessed = errors.New("заявка уже обработана")

// FinishWithdrawal — «Выплачено» (paid) или «Отклонить» (rejected: сумма
// возвращается в кошелёк). Обрабатывается только заявка в статусе pending.
func (s *Store) FinishWithdrawal(id int64, status, reason, now string) (Withdrawal, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return Withdrawal{}, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`SELECT `+withdrawalColumns+` FROM withdrawals w JOIN users u ON u.id = w.user_id
		WHERE w.id = $1 FOR UPDATE OF w`, id)
	if err != nil {
		return Withdrawal{}, err
	}
	list, err := scanWithdrawals(rows)
	if err != nil {
		return Withdrawal{}, err
	}
	if len(list) == 0 {
		return Withdrawal{}, sql.ErrNoRows
	}
	w := list[0]
	if w.Status != "pending" {
		return w, ErrWithdrawalProcessed
	}
	if _, err := tx.Exec(`UPDATE withdrawals SET status = $1, processed_at = $2, reject_reason = NULLIF($3, '') WHERE id = $4`,
		status, now, reason, id); err != nil {
		return w, err
	}
	if status == "rejected" {
		if _, err := tx.Exec(`
			INSERT INTO wallet_transactions (user_id, amount, note, created_at, kind, withdrawal_id)
			VALUES ($1, $2, $3, $4, 'refund', $5)
		`, w.UserID, w.Gross, "Возврат: вывод отклонён — "+reason, now, id); err != nil {
			return w, err
		}
	}
	w.Status = status
	return w, tx.Commit()
}

// UsersWithBalanceAtLeast — для напоминания в день вывода.
func (s *Store) UsersWithBalanceAtLeast(min int) ([]struct {
	TgID    string
	Balance int
}, error) {
	rows, err := s.DB.Query(`
		SELECT u.tg_id, SUM(w.amount)::int FROM wallet_transactions w JOIN users u ON u.id = w.user_id
		GROUP BY u.tg_id HAVING SUM(w.amount) >= $1
	`, min)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		TgID    string
		Balance int
	}
	for rows.Next() {
		var r struct {
			TgID    string
			Balance int
		}
		if err := rows.Scan(&r.TgID, &r.Balance); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// InvitedRow — приглашённый для экрана кошелька.
type InvitedRow struct {
	Name      string
	CreatedAt string
	Tier      string
	ExpiresAt sql.NullString
	PaidCount int
	Earned    int // сколько он принёс пригласившему
}

func (s *Store) Invited(referrerID int64) ([]InvitedRow, error) {
	rows, err := s.DB.Query(`
		SELECT u.name, u.created_at, u.subscription_tier, u.subscription_expires_at,
		       (SELECT COUNT(*) FROM subscription_payments p WHERE p.user_id = u.id)::int,
		       COALESCE((SELECT SUM(p.commission_amount) FROM subscription_payments p
		                 WHERE p.user_id = u.id AND p.referrer_commission_user_id = $1), 0)::int
		FROM users u WHERE u.referred_by_user_id = $1
		ORDER BY u.created_at DESC
	`, referrerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InvitedRow
	for rows.Next() {
		var r InvitedRow
		if err := rows.Scan(&r.Name, &r.CreatedAt, &r.Tier, &r.ExpiresAt, &r.PaidCount, &r.Earned); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
