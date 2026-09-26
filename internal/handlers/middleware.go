package handlers

import (
	"net/http"

	"version20/internal/models"
	"version20/internal/subscription"
)

// requireOnboarded — нужна сессия + пользователь уже прошёл приветствие
// (но, возможно, ещё не анкету) и принял текущую редакцию соглашения.
// Если что-то не так — на "/", он сам решит, что показать.
func (a *App) requireOnboarded(next http.HandlerFunc) http.HandlerFunc {
	return a.requireSession(func(w http.ResponseWriter, r *http.Request) {
		if needsTerms(userFromCtx(r)) {
			a.renderTermsUpdate(w)
			return
		}
		next(w, r)
	})
}

// requireSession — сессия и зарегистрированный пользователь, без проверки соглашения.
func (a *App) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tgID, ok := a.Sessions.TgIDFromRequest(r)
		if !ok {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		user, err := a.Store.GetUserByTgID(tgID)
		if err != nil {
			a.serverError(w, err)
			return
		}
		if user == nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		next(w, withUser(r, user))
	}
}

// requireQuizDone — как requireOnboarded, но ещё и анкета должна быть пройдена.
func (a *App) requireQuizDone(next http.HandlerFunc) http.HandlerFunc {
	return a.requireOnboarded(func(w http.ResponseWriter, r *http.Request) {
		user := userFromCtx(r)
		if !user.Gender.Valid {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		next(w, r)
	})
}

// requireQuizPending — анкета проходится один раз: повторная отправка
// последнего вопроса после построения плана упала бы на уникальном ключе
// user_schedule (user_id, day_index) и ещё раз начислила бы XP за анкету.
func (a *App) requireQuizPending(next http.HandlerFunc) http.HandlerFunc {
	return a.requireOnboarded(func(w http.ResponseWriter, r *http.Request) {
		if userFromCtx(r).Gender.Valid {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		next(w, r)
	})
}

// hasAccess — пробный период (3 дня с регистрации) ещё идёт или есть
// активная подписка (включая Premium за 100 уровень и промокод).
func (a *App) hasAccess(u *models.User) bool {
	return subscription.HasAccess(u.SubscriptionTier, u.SubscriptionExpiresAt, u.CreatedAt, a.now())
}

// requireAccess — закрывает игру после пробного периода без подписки:
// редирект на тарифы. Профиль, тарифы, оплата, кошелёк и анкета открыты всегда.
func (a *App) requireAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.hasAccess(userFromCtx(r)) {
			http.Redirect(w, r, "/plans?expired=1", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}
