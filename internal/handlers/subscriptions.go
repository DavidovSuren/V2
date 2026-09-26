package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"version20/internal/leveling"
	"version20/internal/referrals"
	"version20/internal/reminders"
	"version20/internal/store"
	"version20/internal/subscription"
)

// TierInfo — тариф в интерфейсе: цена всегда полная, скидок нет.
type TierInfo struct {
	Tier  string
	Name  string
	Price int
	Rate  referrals.Rate
}

func tierInfos() []TierInfo {
	out := make([]TierInfo, 0, len(referrals.TierOrder))
	for _, tier := range referrals.TierOrder {
		out = append(out, TierInfo{Tier: tier, Name: referrals.TierNames[tier], Price: referrals.Prices[tier], Rate: referrals.Rates[tier]})
	}
	return out
}

// ProfileData — экран «Я» (макет 09).
type ProfileData struct {
	Name       string
	Initial    string
	Diamond    bool
	TierPill   string // "Premium до 24 октября" / "Пробный: 2 дн." / "Без подписки"
	TierActive bool
	Theme      string // dark | light | auto
	Balance    int
	RemindAt   string
	PromoError string
	Support    string
}

func (a *App) handleProfile(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	_, level := leveling.ProgressFromCompleted(user.CompletedCount)

	data := ProfileData{
		Name: user.Name, Initial: initial(user.Name), Diamond: level >= 100,
		Theme: themePref(r), RemindAt: reminders.Effective(user.RemindAt.String),
		PromoError: r.URL.Query().Get("promo_err"), Support: SupportContact,
	}
	if active := subscription.ActiveTier(user.SubscriptionTier, user.SubscriptionExpiresAt); active != "" {
		data.TierActive = true
		data.TierPill = referrals.TierNames[active] + " до " + humanDateOf(user.SubscriptionExpiresAt.String)
	} else if left := subscription.TrialDaysLeft(user.CreatedAt, a.now()); left > 0 {
		data.TierPill = fmt.Sprintf("Пробный: %d дн.", left)
	} else {
		data.TierPill = "Без подписки"
	}
	summary, err := a.Store.WalletSummary(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	data.Balance = summary.Balance

	a.render(w, "profile.html", data)
}

var errUnknownTier = errors.New("неизвестный тариф")

// activateSubscription — единственное место, где оплата превращается в
// подписку. В одной транзакции: продление (тот же активный тариф — от старой
// даты окончания, иначе от сейчас) на месяц, запись оплаты по полной цене и
// партнёрский доход пригласившему, если у него есть активная подписка.
// Повторный chargeID ничего не делает — Telegram может прислать
// successful_payment дважды.
func (a *App) activateSubscription(userID int64, tier, chargeID string) error {
	price, ok := referrals.Prices[tier]
	if !ok {
		return errUnknownTier
	}
	now := a.now().UTC()

	tx, err := a.Store.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if chargeID != "" {
		paid, err := a.Store.PaymentExists(tx, chargeID)
		if err != nil || paid {
			return err
		}
	}

	st, err := a.Store.LockSubscriptionState(tx, userID)
	if err != nil {
		return err
	}

	start := now
	if subscription.ActiveTier(st.Tier, st.ExpiresAt) == tier {
		if cur, err := time.Parse(time.RFC3339, st.ExpiresAt.String); err == nil && cur.After(now) {
			start = cur
		}
	}
	expires := start.AddDate(0, 1, 0).Format(time.RFC3339)

	payment := store.Payment{UserID: userID, Tier: tier, Price: price, ChargeID: chargeID, PaidAt: now.Format(time.RFC3339)}

	var referrerTgID string
	if st.ReferrerID.Valid {
		referrer, err := a.Store.GetUserByID(st.ReferrerID.Int64)
		if err != nil {
			return err
		}
		if referrer != nil {
			prior, err := a.Store.PriorPayingReferralsCount(tx, referrer.ID, userID)
			if err != nil {
				return err
			}
			pct := referrals.CommissionPct(subscription.ActiveTier(referrer.SubscriptionTier, referrer.SubscriptionExpiresAt), prior)
			if pct > 0 {
				payment.ReferrerID = sql.NullInt64{Int64: referrer.ID, Valid: true}
				payment.CommissionPct = pct
				payment.CommissionAmount = referrals.CommissionAmount(price, pct)
				referrerTgID = referrer.TgID
			}
		}
	}

	if err := a.Store.InsertSubscriptionPayment(tx, payment); err != nil {
		return err
	}
	if err := a.Store.UpdateSubscription(tx, userID, tier, expires); err != nil {
		return err
	}
	if payment.CommissionAmount > 0 {
		note := fmt.Sprintf("Партнёрский доход: %s, %s", st.Name, referrals.TierNames[tier])
		if err := a.Store.InsertWalletTransaction(tx, payment.ReferrerID.Int64, payment.CommissionAmount, userID, note, payment.PaidAt); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	if payment.CommissionAmount > 0 {
		a.notify(referrerTgID, fmt.Sprintf("+%d ₽ в кошелёк: %s оформил(а) %s",
			payment.CommissionAmount, st.Name, referrals.TierNames[tier]))
	}
	return nil
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	a.Sessions.ClearCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
