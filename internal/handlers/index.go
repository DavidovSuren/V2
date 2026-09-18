package handlers

import "net/http"

type WelcomeData struct {
	RefCode       string
	Error         string
	NeedBootstrap bool
}

// handleIndex — "/" ветвится по состоянию пользователя: нет сессии →
// приветствие с бутстрапом; сессия есть, но не зарегистрирован →
// приветственная форма; зарегистрирован, но без анкеты → на анкету;
// иначе — главный экран.
func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	tgID, ok := a.Sessions.TgIDFromRequest(r)
	if !ok {
		// Сессии ещё нет — только тут bootstrap.js должен слать initData.
		a.render(w, "welcome.html", WelcomeData{RefCode: refCodeFromCookie(r), NeedBootstrap: true})
		return
	}

	user, err := a.Store.GetUserByTgID(tgID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if user == nil {
		a.render(w, "welcome.html", WelcomeData{RefCode: refCodeFromCookie(r)})
		return
	}
	if !user.Gender.Valid {
		http.Redirect(w, r, "/quiz/0", http.StatusSeeOther)
		return
	}

	a.renderHome(w, r, user)
}

func refCodeFromCookie(r *http.Request) string {
	c, err := r.Cookie(refCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}
