// Package handlers — HTTP-обработчики, один файл на раздел (зеркалит
// бывшие backend/routes/*.js). App держит общие зависимости.
package handlers

import (
	"html/template"
	"io/fs"
	"net/http"

	"version20/internal/auth"
	"version20/internal/models"
	"version20/internal/store"
)

type App struct {
	Store       *store.Store
	Sessions    *auth.Sessions
	Tmpl        map[string]*template.Template // ключ — имя файла страницы, напр. "home.html"
	StaticFS    fs.FS
	TasksMale   []models.Task
	TasksFemale []models.Task
	BotToken    string
	AdminTgID   string
	DevFakeAuth bool
}

// render выполняет layout.html для конкретной страницы (см. cmd/server/main.go
// loadTemplates — каждая страница парсится вместе с layout+partials в
// отдельный *template.Template, поэтому блоки {{define "content"}} разных
// страниц не конфликтуют друг с другом).
func (a *App) render(w http.ResponseWriter, page string, data any) {
	t, ok := a.Tmpl[page]
	if !ok {
		http.Error(w, "шаблон не найден: "+page, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		a.serverError(w, err)
	}
}

func (a *App) TaskBankFor(gender string) []models.Task {
	switch gender {
	case "male":
		return a.TasksMale
	case "female":
		return a.TasksFemale
	default:
		return nil
	}
}

func (a *App) CategoryTotals(gender string) map[string]int {
	totals := map[string]int{}
	for _, t := range a.TaskBankFor(gender) {
		totals[t.Category]++
	}
	return totals
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})

	mux.HandleFunc("POST /auth/bootstrap", a.handleAuthBootstrap)

	mux.HandleFunc("GET /", a.handleIndex)
	mux.HandleFunc("POST /onboarding", a.handleOnboardingSubmit)

	mux.HandleFunc("GET /quiz/{n}", a.requireOnboarded(a.handleQuizShow))
	mux.HandleFunc("POST /quiz/{n}", a.requireOnboarded(a.handleQuizSubmit))

	mux.HandleFunc("GET /today/skip", a.requireQuizDone(a.handleSkipConfirm))
	mux.HandleFunc("POST /today/skip", a.requireQuizDone(a.handleSkipSubmit))

	mux.HandleFunc("GET /diary", a.requireQuizDone(a.handleDiaryShow))
	mux.HandleFunc("POST /diary", a.requireQuizDone(a.handleDiarySubmit))

	// Профиль/сообщество/подписка и т.п. доступны сразу после регистрации,
	// анкета для них не обязательна (как и в Node-версии — requireUser там
	// проверял только регистрацию, не прохождение анкеты).
	mux.HandleFunc("GET /progress", a.requireOnboarded(a.handleProgress))
	mux.HandleFunc("GET /achievements", a.requireOnboarded(a.handleAchievements))

	mux.HandleFunc("GET /community", a.requireOnboarded(a.handleCommunity))
	mux.HandleFunc("POST /friends/add", a.requireOnboarded(a.handleFriendAdd))

	mux.HandleFunc("GET /profile", a.requireOnboarded(a.handleProfile))
	mux.HandleFunc("POST /subscribe", a.requireOnboarded(a.handleSubscribe))
	mux.HandleFunc("POST /promo", a.requireOnboarded(a.handlePromoRedeem))
	mux.HandleFunc("POST /logout", a.requireOnboarded(a.handleLogout))

	mux.HandleFunc("GET /reports/weekly", a.requireOnboarded(a.handleWeeklyReport))

	mux.HandleFunc("GET /wallet", a.requireOnboarded(a.handleWalletShow))
	mux.HandleFunc("POST /wallet/withdraw", a.requireOnboarded(a.handleWalletWithdraw))

	mux.HandleFunc("POST /admin/grant-premium-agent", a.handleAdminGrantPremiumAgent)

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(a.StaticFS)))

	return mux
}
