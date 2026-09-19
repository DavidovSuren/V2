package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"version20/internal/achievements"
	"version20/internal/promo"
)

// handlePromoRedeem — промокод в профиле (пока единственный: 100LVL сразу
// даёт 100 уровень — алмаз, все level-бейджи и полгода Premium в подарок,
// как при честном прохождении всех 365 заданий).
func (a *App) handlePromoRedeem(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	r.ParseForm()
	rawCode := strings.TrimSpace(r.FormValue("code"))

	effect, ok := promo.Lookup(rawCode)
	if !ok {
		http.Redirect(w, r, "/profile?promo_err="+url.QueryEscape("Промокод не найден"), http.StatusSeeOther)
		return
	}

	code := promo.CanonicalCode(rawCode)
	if !user.Gender.Valid {
		http.Redirect(w, r, "/profile?promo_err="+url.QueryEscape("Сначала пройди анкету"), http.StatusSeeOther)
		return
	}

	already, err := a.Store.HasRedeemedPromo(user.ID, code)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if already {
		http.Redirect(w, r, "/profile?promo_err="+url.QueryEscape("Этот промокод уже использован"), http.StatusSeeOther)
		return
	}

	now := time.Now().UTC()
	wasBelow100 := user.Level < 100

	switch effect {
	case promo.EffectLevel100:
		tx, err := a.Store.DB.Begin()
		if err != nil {
			a.serverError(w, err)
			return
		}
		defer tx.Rollback()

		if err := a.Store.RedeemPromo(tx, user.ID, code, now.Format(time.RFC3339)); err != nil {
			a.serverError(w, err)
			return
		}
		if err := a.Store.MarkAllScheduleDone(tx, user.ID); err != nil {
			a.serverError(w, err)
			return
		}
		if _, err := tx.Exec(`
			UPDATE users SET completed_count = 365, day_index = 365, level = 100 WHERE id = $1
		`, user.ID); err != nil {
			a.serverError(w, err)
			return
		}
		expires := now.AddDate(0, 6, 0).Format(time.RFC3339)
		if err := a.Store.GrantLevel100Premium(tx, user.ID, expires); err != nil {
			a.serverError(w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			a.serverError(w, err)
			return
		}

		updatedUser, err := a.Store.GetUserByID(user.ID)
		if err == nil && updatedUser != nil {
			_, _ = achievements.CheckAndUnlock(a.Store, updatedUser)
		}

		if wasBelow100 {
			http.Redirect(w, r, "/?celebrate=level100", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
	default:
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
	}
}
