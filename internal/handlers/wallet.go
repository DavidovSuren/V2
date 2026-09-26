package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"version20/internal/models"
	"version20/internal/payouts"
	"version20/internal/referrals"
	"version20/internal/reports"
	"version20/internal/store"
	"version20/internal/subscription"
)

type WalletData struct {
	Summary        store.WalletSummary
	RateLabel      string // "Premium · 20%"
	RateHint       string // "ещё 3 оплативших до 50%"
	NoSubscription bool
	InvitedCount   int
	PaidCount      int
	Link           string
	ShareURL       string
	People         []InvitedView
	History        []HistoryView
	PayoutToday    bool
	NextPayout     string // "15 октября"
	Message        string
}

type InvitedView struct {
	Initial string
	Name    string
	Status  string
	Earned  int
	Right   string // «ждём оплату» / «не оплатил», если ничего не принёс
}

type HistoryView struct {
	Amount int
	Note   string
	Date   string
	Kind   string
	Status string // для выводов: «в обработке» / «выплачено» / «отклонено»
}

var withdrawalStatusLabels = map[string]string{"pending": "в обработке", "paid": "выплачено", "rejected": "отклонено"}

var walletMessages = map[string]string{
	"ok": "Заявка на вывод принята. Деньги придут на карту в течение 3 рабочих дней.",
}

func initial(name string) string {
	r, _ := utf8.DecodeRuneInString(strings.TrimSpace(name))
	if r == utf8.RuneError {
		return "?"
	}
	return strings.ToUpper(string(r))
}

// humanDateOf — "24 сентября" из RFC3339 (по Москве).
func humanDateOf(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	return payouts.HumanDate(t.In(reports.MoscowLocation()))
}

// partnerRate — текущий процент партнёра: плашка в кошельке.
func (a *App) partnerRate(user *models.User, paying int) (label, hint string, none bool) {
	active := subscription.ActiveTier(user.SubscriptionTier, user.SubscriptionExpiresAt)
	if active == "" {
		return "", "", true
	}
	pct := referrals.CommissionPct(active, paying)
	label = fmt.Sprintf("%s · %d%%", referrals.TierNames[active], pct)
	if left := referrals.BoostThreshold - paying; left > 0 {
		hint = fmt.Sprintf("ещё %d оплативших до %d%%", left, referrals.Rates[active].Boosted)
	}
	return label, hint, false
}

func (a *App) handleWalletShow(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	now := a.now()
	msk := reports.MoscowLocation()

	summary, err := a.Store.WalletSummary(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	paying, err := a.Store.PayingReferralsCount(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	invited, err := a.Store.Invited(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	history, err := a.Store.WalletHistory(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}

	data := WalletData{
		Summary: summary, InvitedCount: len(invited), PaidCount: paying,
		Link: a.referralLink(user.ReferralCode.String), PayoutToday: payouts.IsPayoutDay(now, msk),
		NextPayout: payouts.HumanDate(payouts.NextPayoutDate(now, msk)),
		Message:    walletMessages[r.URL.Query().Get("withdraw")],
	}
	data.ShareURL = shareURL(data.Link)
	data.RateLabel, data.RateHint, data.NoSubscription = a.partnerRate(user, paying)

	for _, p := range invited {
		v := InvitedView{Initial: initial(p.Name), Name: p.Name, Earned: p.Earned}
		active := subscription.ActiveTier(p.Tier, p.ExpiresAt)
		switch {
		case active != "":
			v.Status = referrals.TierNames[active] + " · " + humanDateOf(p.CreatedAt)
		case subscription.TrialDaysLeft(p.CreatedAt, now) > 0:
			v.Status = fmt.Sprintf("Пробный период · осталось %d дн.", subscription.TrialDaysLeft(p.CreatedAt, now))
			v.Right = "ждём оплату"
		case p.PaidCount > 0:
			v.Status = "Подписка закончилась"
		default:
			v.Status = "Пробный закончился"
			v.Right = "не оплатил"
		}
		data.People = append(data.People, v)
	}
	for _, h := range history {
		data.History = append(data.History, HistoryView{
			Amount: h.Amount, Note: h.Note.String, Date: humanDateOf(h.CreatedAt),
			Kind: h.Kind, Status: withdrawalStatusLabels[h.Status.String],
		})
	}

	a.render(w, "wallet.html", data)
}

// ---- Вывод ----

type WithdrawData struct {
	Balance    int
	Split      payouts.Breakdown
	TaxPct     int
	SavedMask  string
	Card       string // введённое значение при ошибке
	CardError  string
	Blocked    string // почему вывод сейчас невозможен
	NextPayout string
}

// withdrawBlock — причина, по которой вывод сейчас невозможен ("" — можно).
func (a *App) withdrawBlock(q store.Queryer, userID int64, balance int, now time.Time) (string, error) {
	msk := reports.MoscowLocation()
	next := payouts.HumanDate(payouts.NextPayoutDate(now, msk))
	if a.CardKey == nil {
		return "Вывод временно недоступен. Напиши в поддержку.", nil
	}
	if !payouts.IsPayoutDay(now, msk) {
		return "Вывод доступен " + next + " — раз в месяц, 15-го числа.", nil
	}
	used, err := a.Store.HasWithdrawalInMonth(q, userID, payouts.MonthKey(now, msk))
	if err != nil {
		return "", err
	}
	if used {
		return "В этом месяце вывод уже был. Следующий — " + payouts.HumanDate(payouts.NextPayoutDate(now.AddDate(0, 0, 1), msk)) + ".", nil
	}
	if balance < payouts.MinAmount {
		return fmt.Sprintf("Минимальная сумма вывода — %s. Сейчас на балансе %s.", Rub(payouts.MinAmount), Rub(balance)), nil
	}
	return "", nil
}

func (a *App) withdrawPage(user *models.User) (WithdrawData, error) {
	summary, err := a.Store.WalletSummary(user.ID)
	if err != nil {
		return WithdrawData{}, err
	}
	data := WithdrawData{Balance: summary.Balance, TaxPct: a.taxPct(), Split: payouts.Split(summary.Balance, a.taxPct())}
	if data.Blocked, err = a.withdrawBlock(a.Store.DB, user.ID, summary.Balance, a.now()); err != nil {
		return data, err
	}
	if _, last4, err := a.Store.PayoutCard(user.ID); err == nil && last4 != "" {
		data.SavedMask = "•••• •••• •••• " + last4
	}
	return data, nil
}

func (a *App) handleWithdrawShow(w http.ResponseWriter, r *http.Request) {
	data, err := a.withdrawPage(userFromCtx(r))
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, "withdraw.html", data)
}

// handleWalletWithdraw — заявка на вывод всего баланса. Нужен только номер
// карты; сохранённую карту можно не вводить заново.
func (a *App) handleWalletWithdraw(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	r.ParseForm()
	input := strings.TrimSpace(r.FormValue("card"))

	fail := func(cardErr, blocked string) {
		data, err := a.withdrawPage(user)
		if err != nil {
			a.serverError(w, err)
			return
		}
		data.CardError, data.Card = cardErr, input
		if blocked != "" {
			data.Blocked = blocked
		}
		a.render(w, "withdraw.html", data)
	}

	if a.CardKey == nil {
		fail("", "Вывод временно недоступен. Напиши в поддержку.")
		return
	}

	digits := payouts.NormalizeCard(input)
	var cardEnc string
	if strings.Contains(input, "•") || digits == "" {
		// Оставил сохранённую карту.
		enc, _, err := a.Store.PayoutCard(user.ID)
		if err != nil {
			a.serverError(w, err)
			return
		}
		if enc == "" {
			fail("Введи номер карты", "")
			return
		}
		if digits, err = a.CardKey.Decrypt(enc); err != nil {
			fail("Введи номер карты заново", "")
			return
		}
		cardEnc = enc
	}
	if !payouts.ValidCard(digits) {
		fail("Проверь номер карты — в нём ошибка", "")
		return
	}
	if cardEnc == "" {
		var err error
		if cardEnc, err = a.CardKey.Encrypt(digits); err != nil {
			a.serverError(w, err)
			return
		}
	}
	last4 := digits[len(digits)-4:]

	now := a.now()
	tx, err := a.Store.DB.Begin()
	if err != nil {
		a.serverError(w, err)
		return
	}
	defer tx.Rollback()

	balance, err := a.Store.LockWalletBalance(tx, user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	blocked, err := a.withdrawBlock(tx, user.ID, balance, now)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if blocked != "" {
		tx.Rollback()
		fail("", blocked)
		return
	}
	split := payouts.Split(balance, a.taxPct())
	if _, err := a.Store.CreateWithdrawal(tx, store.NewWithdrawal{
		UserID: user.ID, MonthKey: payouts.MonthKey(now, reports.MoscowLocation()),
		Gross: split.Gross, Tax: split.Tax, Net: split.Net, TaxPct: a.taxPct(),
		CardLast4: last4, CardEnc: cardEnc, CreatedAt: now.UTC().Format(time.RFC3339),
	}); err != nil {
		a.serverError(w, err)
		return
	}
	if err := a.Store.SavePayoutCard(tx, user.ID, cardEnc, last4); err != nil {
		a.serverError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		a.serverError(w, err)
		return
	}

	who := user.Name
	if user.Username.Valid && user.Username.String != "" {
		who += " (@" + user.Username.String + ")"
	}
	a.notify(a.AdminTgID, fmt.Sprintf("Новая заявка на вывод: %s — %s на карту •••• %s. Админка: /admin/withdrawals",
		who, Rub(split.Net), last4))
	http.Redirect(w, r, "/wallet?withdraw=ok", http.StatusSeeOther)
}
