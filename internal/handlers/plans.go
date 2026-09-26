package handlers

import (
	"net/http"

	"version20/internal/referrals"
	"version20/internal/subscription"
)

type PlansData struct {
	Tiers      []TierInfo
	ActiveTier string
	ActiveName string
	Message    string
}

var planMessages = map[string]string{
	"cancelled": "Оплата отменена. Подписку можно оформить в любой момент.",
	"failed":    "Оплата не прошла. Попробуй ещё раз или выбери другую карту.",
	"pending":   "Ждём подтверждения оплаты от Telegram — обычно это пара секунд.",
}

func (a *App) handlePlans(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	active := subscription.ActiveTier(user.SubscriptionTier, user.SubscriptionExpiresAt)
	data := PlansData{Tiers: tierInfos(), ActiveTier: active, ActiveName: referrals.TierNames[active]}
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
