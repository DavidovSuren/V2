package handlers

import "net/http"

// requireOnboarded — нужна сессия + пользователь уже прошёл приветствие
// (но, возможно, ещё не анкету). Если что-то не так — на "/", он сам
// решит, что показать (бутстрап / приветствие / анкету / главную).
func (a *App) requireOnboarded(next http.HandlerFunc) http.HandlerFunc {
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
