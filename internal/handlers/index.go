package handlers

import "net/http"

type WelcomeData struct {
	RefCode       string
	Error         string
	NeedBootstrap bool
	// DevAuth — DEV_ALLOW_FAKE_AUTH: bootstrap.js вне Telegram заводит
	// тестовую сессию "debug:<id>", чтобы приложение работало в обычном браузере.
	DevAuth bool
	// Name/AgeGroup — чтобы при ошибке не заставлять заполнять форму заново.
	Name     string
	AgeGroup string
}

// handleIndex — "/" ветвится по состоянию пользователя: нет сессии →
// приветствие с бутстрапом; сессия есть, но не зарегистрирован →
// приветственная форма; зарегистрирован, но без анкеты → на анкету;
// иначе — главный экран.
func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	// ?ref=КОД — кнопка бота после /start КОД (ссылка t.me/<бот>?start=КОД).
	if ref := r.URL.Query().Get("ref"); ref != "" && isSafeCode(ref) {
		http.SetCookie(w, &http.Cookie{Name: refCookieName, Value: ref, Path: "/", MaxAge: 3600, SameSite: http.SameSiteLaxMode})
	}
	tgID, ok := a.Sessions.TgIDFromRequest(r)
	if !ok {
		// Сессии ещё нет — только тут bootstrap.js должен слать initData.
		a.render(w, "welcome.html", WelcomeData{RefCode: refCodeFromCookie(r), NeedBootstrap: true, DevAuth: a.DevFakeAuth})
		return
	}

	user, err := a.Store.GetUserByTgID(tgID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if user == nil {
		a.render(w, "welcome.html", WelcomeData{RefCode: refCodeFromCookie(r), Name: firstNameFromCookie(r)})
		return
	}
	if needsTerms(user) {
		a.renderTermsUpdate(w)
		return
	}
	if !user.Gender.Valid {
		http.Redirect(w, r, "/quiz/0", http.StatusSeeOther)
		return
	}
	if !a.hasAccess(user) {
		http.Redirect(w, r, "/plans?expired=1", http.StatusSeeOther)
		return
	}

	a.renderHome(w, r, user)
}

func refCodeFromCookie(r *http.Request) string {
	if ref := r.URL.Query().Get("ref"); ref != "" && isSafeCode(ref) {
		return ref
	}
	c, err := r.Cookie(refCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

type AgeOption struct{ Value, Label string }

// AgeGroups — сетка 3×2 на экране приветствия (значения — как в Node-версии).
func (WelcomeData) AgeGroups() []AgeOption {
	return []AgeOption{
		{"17-20", "17–20"}, {"21-25", "21–25"}, {"26-30", "26–30"},
		{"31-35", "31–35"}, {"36-45", "36–45"}, {"45+", "45+"},
	}
}
