package handlers

import (
	"net/http"

	"version20/internal/subscription"
)

// handleBecomeAgent — пользователь с активным Premium сам становится
// премиум-агентом: получает агентский код (приглашённые по нему платят
// полную цену, агенту — 50% в кошелёк). Повторный запрос просто ведёт в кошелёк.
func (a *App) handleBecomeAgent(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	if user.HasPremiumAgentCode() {
		http.Redirect(w, r, "/wallet", http.StatusSeeOther)
		return
	}
	if !subscription.IsPremiumActive(user.SubscriptionTier, user.SubscriptionExpiresAt) {
		http.Redirect(w, r, "/profile?agent_err=premium", http.StatusSeeOther)
		return
	}
	if _, err := a.issuePremiumAgentCode(user.ID); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/wallet", http.StatusSeeOther)
}
