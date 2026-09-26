package handlers

import (
	"net/http"
	"time"

	"version20/internal/payouts"
	"version20/internal/referrals"
	"version20/internal/reports"
	"version20/internal/subscription"
)

type PlansData struct {
	// State — "trial" (идёт пробный), "expired" (пробный закончился, подписки
	// нет) или "active" (есть подписка).
	State          string
	TrialDaysLeft  int
	CompletedCount int
	ActiveTier     string
	ActiveName     string
	ActiveUntil    string // "24 октября"
	Tiers          []TierInfo
	BoostThreshold int
	Message        string
}

var planMessages = map[string]string{
	"cancelled": "Оплата отменена. Подписку можно оформить в любой момент.",
	"failed":    "Оплата не прошла. Попробуй ещё раз или выбери другую карту.",
	"pending":   "Ждём подтверждения оплаты от Telegram — обычно это пара секунд.",
}

// plansOrder — на экране тарифов Premium первым («Выбор большинства»).
func plansOrder() []TierInfo {
	tiers := tierInfos()
	for i, j := 0, len(tiers)-1; i < j; i, j = i+1, j-1 {
		tiers[i], tiers[j] = tiers[j], tiers[i]
	}
	return tiers
}

func (a *App) handlePlans(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	now := a.now()
	active := subscription.ActiveTier(user.SubscriptionTier, user.SubscriptionExpiresAt)
	data := PlansData{
		Tiers: plansOrder(), BoostThreshold: referrals.BoostThreshold,
		CompletedCount: user.CompletedCount,
		ActiveTier:     active, ActiveName: referrals.TierNames[active],
	}
	switch {
	case active != "":
		data.State = "active"
		if t, err := time.Parse(time.RFC3339, user.SubscriptionExpiresAt.String); err == nil {
			data.ActiveUntil = payouts.HumanDate(t.In(reports.MoscowLocation()))
		}
	case subscription.TrialDaysLeft(user.CreatedAt, now) > 0:
		data.State = "trial"
		data.TrialDaysLeft = subscription.TrialDaysLeft(user.CreatedAt, now)
	default:
		data.State = "expired"
	}

	if r.URL.Query().Get("paid") == "1" {
		data.Message = "Оплата прошла, подписка активна."
		if active == "" {
			// successful_payment от Telegram может прийти на пару секунд позже.
			data.Message = planMessages["pending"]
		}
	} else {
		data.Message = planMessages[r.URL.Query().Get("pay")]
	}
	a.render(w, "plans.html", data)
}

// taxPct — НДФЛ, удерживаемый при выводе (TAX_WITHHOLD_PCT, по умолчанию 13).
func (a *App) taxPct() int {
	if a.TaxWithholdPct > 0 {
		return a.TaxWithholdPct
	}
	return payouts.DefaultTaxPct
}
