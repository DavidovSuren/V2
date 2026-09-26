// Package handlers — HTTP-обработчики, один файл на раздел (зеркалит
// бывшие backend/routes/*.js). App держит общие зависимости.
package handlers

import (
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"version20/internal/auth"
	"version20/internal/cardcrypt"
	"version20/internal/content"
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

	// Content — вопросы и задания из БД (редактируются в админке). Если nil,
	// используются встроенные models.Questions и TasksMale/TasksFemale.
	Content *content.Cache

	// Админка (/admin): вход по ADMIN_LOGIN/ADMIN_PASSWORD; пустые — вход выключен.
	AdminLogin    string
	AdminPassword string
	AdminSessions *auth.Sessions
	// Кабинет партнёра (/partner): вход по реферальному коду и паролю.
	PartnerSessions *auth.Sessions

	// Now — источник времени (в тестах — фиксированная дата); nil — time.Now.
	Now func() time.Time
	// Notify — отправка сообщения от бота; nil — Bot API (если задан BOT_TOKEN).
	Notify func(tgID, text string)

	// Оплата через Telegram (этап 2).
	Bot              BotAPI // nil — Bot API недоступен (нет BOT_TOKEN)
	BotUsername      string // @username бота без @, из getMe при старте
	Payments         PaymentConfig
	PublicURL        string // PUBLIC_URL — https-адрес приложения
	MiniAppShortName string // MINIAPP_SHORT_NAME — короткое имя Mini App в @BotFather

	TaxWithholdPct int // TAX_WITHHOLD_PCT — НДФЛ при выводе; 0 — по умолчанию 13
	// CardKey — ключ шифрования номеров карт (CARD_ENC_KEY); nil — вывод выключен.
	CardKey *cardcrypt.Key
}

func (a *App) Questions() []models.Question {
	if a.Content != nil {
		return a.Content.Questions()
	}
	return models.Questions
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
	if a.Content != nil {
		return a.Content.TaskBank(gender)
	}
	switch gender {
	case "male":
		return a.TasksMale
	case "female":
		return a.TasksFemale
	default:
		return nil
	}
}

// CategoryTotals — сколько заданий в каждой категории банка, в порядке
// первого появления категории в банке (как Object.keys в Node-версии),
// чтобы экран "Прогресс" не перемешивал направления при каждом заходе.
func (a *App) CategoryTotals(gender string) []CategoryTotal {
	var out []CategoryTotal
	idx := map[string]int{}
	for _, t := range a.TaskBankFor(gender) {
		i, ok := idx[t.Category]
		if !ok {
			i = len(out)
			idx[t.Category] = i
			out = append(out, CategoryTotal{Category: t.Category})
		}
		out[i].Total++
	}
	return out
}

type CategoryTotal struct {
	Category string
	Total    int
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})

	mux.HandleFunc("POST /auth/bootstrap", a.handleAuthBootstrap)
	mux.HandleFunc("POST /telegram/webhook", a.handleTelegramWebhook)

	mux.HandleFunc("GET /", a.handleIndex)
	mux.HandleFunc("POST /onboarding", a.handleOnboardingSubmit)

	mux.HandleFunc("GET /quiz/{n}", a.requireQuizPending(a.handleQuizShow))
	mux.HandleFunc("POST /quiz/{n}", a.requireQuizPending(a.handleQuizSubmit))

	mux.HandleFunc("GET /today/skip", a.requireQuizDone(a.requireAccess(a.handleSkipConfirm)))
	mux.HandleFunc("POST /today/skip", a.requireQuizDone(a.requireAccess(a.handleSkipSubmit)))

	mux.HandleFunc("GET /diary", a.requireQuizDone(a.requireAccess(a.handleDiaryShow)))
	mux.HandleFunc("POST /diary", a.requireQuizDone(a.requireAccess(a.handleDiarySubmit)))

	// Профиль/сообщество/подписка и т.п. доступны сразу после регистрации,
	// анкета для них не обязательна (как и в Node-версии — requireUser там
	// проверял только регистрацию, не прохождение анкеты).
	mux.HandleFunc("GET /progress", a.requireOnboarded(a.requireAccess(a.handleProgress)))
	mux.HandleFunc("GET /achievements", a.requireOnboarded(a.handleAchievements))

	mux.HandleFunc("GET /community", a.requireOnboarded(a.requireAccess(a.handleCommunity)))
	mux.HandleFunc("POST /friends/add", a.requireOnboarded(a.requireAccess(a.handleFriendAdd)))

	mux.HandleFunc("GET /profile", a.requireOnboarded(a.handleProfile))
	mux.HandleFunc("GET /plans", a.requireOnboarded(a.handlePlans))
	mux.HandleFunc("POST /subscribe", a.requireOnboarded(a.handleSubscribe))
	mux.HandleFunc("POST /promo", a.requireOnboarded(a.handlePromoRedeem))
	mux.HandleFunc("POST /logout", a.requireOnboarded(a.handleLogout))

	mux.HandleFunc("GET /reports/weekly", a.requireOnboarded(a.requireAccess(a.handleWeeklyReport)))
	mux.HandleFunc("GET /terms", a.handleTerms)

	mux.HandleFunc("GET /wallet", a.requireOnboarded(a.handleWalletShow))
	mux.HandleFunc("GET /wallet/withdraw", a.requireOnboarded(a.handleWithdrawShow))
	mux.HandleFunc("POST /wallet/withdraw", a.requireOnboarded(a.handleWalletWithdraw))
	mux.HandleFunc("POST /partner/password", a.requireOnboarded(a.handlePartnerPasswordSet))

	// Веб-админка (вне Telegram).
	mux.HandleFunc("GET /admin/login", a.handleAdminLoginShow)
	mux.HandleFunc("POST /admin/login", a.handleAdminLoginSubmit)
	mux.HandleFunc("POST /admin/logout", a.handleAdminLogout)
	mux.HandleFunc("GET /admin", a.requireAdmin(a.handleAdminDashboard))
	mux.HandleFunc("GET /admin/questions", a.requireAdmin(a.handleAdminQuestions))
	mux.HandleFunc("POST /admin/questions/{id}", a.requireAdmin(a.handleAdminQuestionSave))
	mux.HandleFunc("GET /admin/tasks", a.requireAdmin(a.handleAdminTasks))
	mux.HandleFunc("GET /admin/tasks/{bank}/{id}", a.requireAdmin(a.handleAdminTaskEdit))
	mux.HandleFunc("POST /admin/tasks/{bank}/{id}", a.requireAdmin(a.handleAdminTaskSave))
	mux.HandleFunc("GET /admin/users", a.requireAdmin(a.handleAdminUsers))
	mux.HandleFunc("GET /admin/users/{id}", a.requireAdmin(a.handleAdminUserEdit))
	mux.HandleFunc("POST /admin/users/{id}", a.requireAdmin(a.handleAdminUserSave))
	mux.HandleFunc("POST /admin/users/{id}/reset-partner-password", a.requireAdmin(a.handleAdminPartnerResetPassword))
	mux.HandleFunc("GET /admin/withdrawals", a.requireAdmin(a.handleAdminWithdrawals))
	mux.HandleFunc("GET /admin/withdrawals.csv", a.requireAdmin(a.handleAdminWithdrawalsCSV))
	mux.HandleFunc("POST /admin/withdrawals/{id}/paid", a.requireAdmin(a.handleAdminWithdrawalPaid))
	mux.HandleFunc("POST /admin/withdrawals/{id}/reject", a.requireAdmin(a.handleAdminWithdrawalReject))

	// Кабинет партнёра (вне Telegram).
	mux.HandleFunc("GET /partner/login", a.handlePartnerLoginShow)
	mux.HandleFunc("POST /partner/login", a.handlePartnerLoginSubmit)
	mux.HandleFunc("POST /partner/logout", a.handlePartnerLogout)
	mux.HandleFunc("GET /partner", a.requirePartner(a.handlePartnerDashboard))

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(a.StaticFS)))

	return mux
}
