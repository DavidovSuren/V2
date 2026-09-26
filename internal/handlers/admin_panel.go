package handlers

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"version20/internal/models"
	"version20/internal/store"
)

// Админка (/admin/...) — отдельный веб-интерфейс вне Telegram: вход по
// ADMIN_LOGIN/ADMIN_PASSWORD, своя cookie v2_admin (auth.Sessions.Scoped).

type AdminPage struct {
	Title   string
	Flash   string
	Error   string
	Content any
}

// flashURL — адрес с сообщением (?ok= / ?err=) для показа после редиректа.
func flashURL(path, key, msg, anchor string) string {
	u := path + "?" + url.Values{key: {msg}}.Encode()
	if anchor != "" {
		u += "#" + anchor
	}
	return u
}

func (a *App) adminEnabled() bool {
	return a.AdminLogin != "" && a.AdminPassword != "" && a.AdminSessions != nil
}

func (a *App) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.adminEnabled() {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		login, ok := a.AdminSessions.TgIDFromRequest(r)
		if !ok || subtle.ConstantTimeCompare([]byte(login), []byte(a.AdminLogin)) != 1 {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (a *App) renderAdmin(w http.ResponseWriter, r *http.Request, page, title string, content any) {
	a.render(w, "panel/"+page, AdminPage{
		Title: title, Content: content,
		Flash: r.URL.Query().Get("ok"), Error: r.URL.Query().Get("err"),
	})
}

func (a *App) handleAdminLoginShow(w http.ResponseWriter, r *http.Request) {
	data := AdminPage{Title: "Вход в админку"}
	if !a.adminEnabled() {
		data.Error = "Админка выключена: задайте ADMIN_LOGIN и ADMIN_PASSWORD в окружении сервера."
	}
	a.render(w, "panel/admin_login.html", data)
}

func (a *App) handleAdminLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if !a.adminEnabled() {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	loginOK := subtle.ConstantTimeCompare([]byte(r.FormValue("login")), []byte(a.AdminLogin)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(r.FormValue("password")), []byte(a.AdminPassword)) == 1
	if !loginOK || !passOK {
		time.Sleep(time.Second) // притормаживаем перебор
		a.render(w, "panel/admin_login.html", AdminPage{Title: "Вход в админку", Error: "Неверный логин или пароль"})
		return
	}
	a.AdminSessions.SetCookie(w, a.AdminLogin)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (a *App) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	if a.AdminSessions != nil {
		a.AdminSessions.ClearCookie(w)
	}
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

func (a *App) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := a.Store.Stats(time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.renderAdmin(w, r, "admin_dashboard.html", "Обзор", stats)
}

// ---- Вопросы анкеты ----

func (a *App) handleAdminQuestions(w http.ResponseWriter, r *http.Request) {
	a.renderAdmin(w, r, "admin_questions.html", "Вопросы анкеты", a.Questions())
}

// handleAdminQuestionSave — правится текст и подписи вариантов. Тип,
// категория и количество/значения вариантов вопроса про пол не меняются:
// от них зависят расчёт весов плана и выбор банка заданий.
func (a *App) handleAdminQuestionSave(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || a.Content == nil {
		http.Redirect(w, r, "/admin/questions", http.StatusSeeOther)
		return
	}
	var q *models.Question
	for _, cur := range a.Questions() {
		if cur.ID == id {
			c := cur
			q = &c
		}
	}
	if q == nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" || !utf8.ValidString(text) || !utf8.ValidString(r.FormValue("options")) {
		http.Redirect(w, r, flashURL("/admin/questions", "err", "Текст вопроса пустой или в неверной кодировке (нужен UTF-8)", "q"+strconv.Itoa(id)), http.StatusSeeOther)
		return
	}

	options := q.Options
	if q.Type == "single" {
		var labels []string
		for _, l := range strings.Split(r.FormValue("options"), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				labels = append(labels, l)
			}
		}
		if q.ID == 0 {
			// Вопрос про пол: значения male/female фиксированы, меняются только подписи.
			if len(labels) != len(q.Options) {
				http.Redirect(w, r, flashURL("/admin/questions", "err", "У вопроса про пол должно остаться 2 варианта", "q0"), http.StatusSeeOther)
				return
			}
			options = make([]models.Option, len(labels))
			for i, l := range labels {
				options[i] = models.Option{Value: q.Options[i].Value, Label: l}
			}
		} else {
			if len(labels) < 2 {
				http.Redirect(w, r, flashURL("/admin/questions", "err", "Нужно минимум 2 варианта ответа", "q"+strconv.Itoa(id)), http.StatusSeeOther)
				return
			}
			options = make([]models.Option, len(labels))
			for i, l := range labels {
				options[i] = models.Option{Value: l, Label: l}
			}
		}
	}

	if err := a.Store.UpdateQuestion(id, text, options); err != nil {
		a.serverError(w, err)
		return
	}
	if err := a.Content.Reload(); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, flashURL("/admin/questions", "ok", "Сохранено", "q"+strconv.Itoa(id)), http.StatusSeeOther)
}

// ---- Задания (советы) ----

type AdminTasksData struct {
	Bank       string
	Category   string
	Query      string
	Categories []string
	Tasks      []models.Task
	Total      int
}

func (a *App) handleAdminTasks(w http.ResponseWriter, r *http.Request) {
	bank := r.URL.Query().Get("bank")
	if bank != "female" {
		bank = "male"
	}
	data := AdminTasksData{
		Bank: bank, Category: r.URL.Query().Get("cat"), Query: strings.TrimSpace(r.URL.Query().Get("q")),
		Categories: models.Categories,
	}
	all := a.TaskBankFor(bank)
	data.Total = len(all)
	q := strings.ToLower(data.Query)
	for _, t := range all {
		if data.Category != "" && t.Category != data.Category {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(t.ID+" "+t.Text+" "+t.Why), q) {
			continue
		}
		data.Tasks = append(data.Tasks, t)
	}
	a.renderAdmin(w, r, "admin_tasks.html", "Задания и советы", data)
}

type AdminTaskEditData struct {
	Bank string
	Task models.Task
	Back string
}

func (a *App) findTask(bank, id string) *models.Task {
	for _, t := range a.TaskBankFor(bank) {
		if t.ID == id {
			return &t
		}
	}
	return nil
}

func (a *App) handleAdminTaskEdit(w http.ResponseWriter, r *http.Request) {
	bank, id := r.PathValue("bank"), r.PathValue("id")
	t := a.findTask(bank, id)
	if t == nil {
		http.NotFound(w, r)
		return
	}
	a.renderAdmin(w, r, "admin_task_edit.html", "Задание "+id, AdminTaskEditData{Bank: bank, Task: *t})
}

func (a *App) handleAdminTaskSave(w http.ResponseWriter, r *http.Request) {
	bank, id := r.PathValue("bank"), r.PathValue("id")
	if a.Content == nil || a.findTask(bank, id) == nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	text, why := strings.TrimSpace(r.FormValue("text")), strings.TrimSpace(r.FormValue("why"))
	editURL := "/admin/tasks/" + bank + "/" + id
	if text == "" || why == "" || !utf8.ValidString(text+why) {
		http.Redirect(w, r, flashURL(editURL, "err", "Текст и «почему» обязательны (в кодировке UTF-8)", ""), http.StatusSeeOther)
		return
	}
	if _, err := a.Store.UpdateTask(bank, id, text, why); err != nil {
		a.serverError(w, err)
		return
	}
	if err := a.Content.Reload(); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, flashURL(editURL, "ok", "Сохранено. Изменение попадёт в планы новых пользователей", ""), http.StatusSeeOther)
}

// ---- Пользователи ----

type AdminUsersData struct {
	Query string
	Users []store.AdminUserRow
}

func (a *App) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimPrefix(strings.TrimSpace(r.URL.Query().Get("q")), "@")
	users, err := a.Store.SearchUsers(q, 200)
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.renderAdmin(w, r, "admin_users.html", "Пользователи", AdminUsersData{Query: q, Users: users})
}

type AdminUserEditData struct {
	User        *models.User
	ExpiresDate string
	Balance     int
}

func (a *App) adminTargetUser(w http.ResponseWriter, r *http.Request) *models.User {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	u, err := a.Store.GetUserByID(id)
	if err != nil {
		a.serverError(w, err)
		return nil
	}
	if u == nil {
		http.NotFound(w, r)
		return nil
	}
	return u
}

func (a *App) handleAdminUserEdit(w http.ResponseWriter, r *http.Request) {
	u := a.adminTargetUser(w, r)
	if u == nil {
		return
	}
	data := AdminUserEditData{User: u}
	if u.SubscriptionExpiresAt.Valid && len(u.SubscriptionExpiresAt.String) >= 10 {
		data.ExpiresDate = u.SubscriptionExpiresAt.String[:10]
	}
	if u.HasPremiumAgentCode() {
		data.Balance, _ = a.Store.WalletBalance(u.ID)
	}
	a.renderAdmin(w, r, "admin_user_edit.html", u.Name, data)
}

var adminTiers = map[string]bool{"free": true, "plus369": true, "premium888": true}

func (a *App) handleAdminUserSave(w http.ResponseWriter, r *http.Request) {
	u := a.adminTargetUser(w, r)
	if u == nil {
		return
	}
	r.ParseForm()
	editURL := "/admin/users/" + strconv.FormatInt(u.ID, 10)
	name := strings.TrimSpace(r.FormValue("name"))
	username := strings.TrimPrefix(strings.TrimSpace(r.FormValue("username")), "@")
	tier := r.FormValue("tier")
	if name == "" || !adminTiers[tier] || !utf8.ValidString(name+username) {
		http.Redirect(w, r, flashURL(editURL, "err", "Проверьте имя и тариф", ""), http.StatusSeeOther)
		return
	}
	expires := ""
	if tier != "free" {
		d, err := time.Parse("2006-01-02", r.FormValue("expires"))
		if err != nil {
			http.Redirect(w, r, flashURL(editURL, "err", "Для платного тарифа нужна дата окончания", ""), http.StatusSeeOther)
			return
		}
		expires = d.Add(24*time.Hour - time.Second).UTC().Format(time.RFC3339)
	}
	if err := a.Store.AdminUpdateUser(u.ID, name, username, tier, expires); err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			http.Redirect(w, r, flashURL(editURL, "err", "Такой username уже занят", ""), http.StatusSeeOther)
			return
		}
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, flashURL(editURL, "ok", "Сохранено", ""), http.StatusSeeOther)
}

// ---- Агенты ----

func (a *App) handleAdminAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := a.Store.ListAgents()
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.renderAdmin(w, r, "admin_agents.html", "Агенты", agents)
}

// handleAdminAgentGrant — выдать агентский код по @username / tg_id (форма
// на странице агентов) или по id (кнопка в карточке пользователя).
func (a *App) handleAdminAgentGrant(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	back := r.FormValue("back")
	if back != "/admin/agents" && !strings.HasPrefix(back, "/admin/users/") {
		back = "/admin/agents"
	}
	var target *models.User
	var err error
	if id, perr := strconv.ParseInt(r.FormValue("user_id"), 10, 64); perr == nil {
		target, err = a.Store.GetUserByID(id)
	} else {
		who := strings.TrimPrefix(strings.TrimSpace(r.FormValue("who")), "@")
		if who == "" {
			http.Redirect(w, r, flashURL(back, "err", "Укажите @username или tg_id", ""), http.StatusSeeOther)
			return
		}
		target, err = a.Store.GetUserByTgID(who)
		if err == nil && target == nil {
			target, err = a.Store.GetUserByUsername(who)
		}
	}
	if err != nil {
		a.serverError(w, err)
		return
	}
	if target == nil {
		http.Redirect(w, r, flashURL(back, "err", "Пользователь не найден", ""), http.StatusSeeOther)
		return
	}
	if target.HasPremiumAgentCode() {
		http.Redirect(w, r, flashURL(back, "err", "У пользователя уже есть код "+target.PremiumAgentCode.String, ""), http.StatusSeeOther)
		return
	}
	code, err := a.issuePremiumAgentCode(target.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, flashURL(back, "ok", "Код выдан: "+code, ""), http.StatusSeeOther)
}

func (a *App) handleAdminAgentRevoke(w http.ResponseWriter, r *http.Request) {
	u := a.adminTargetUser(w, r)
	if u == nil {
		return
	}
	if err := a.Store.RevokePremiumAgentCode(u.ID); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, flashURL("/admin/agents", "ok", "Код отозван", ""), http.StatusSeeOther)
}

func (a *App) handleAdminAgentResetPassword(w http.ResponseWriter, r *http.Request) {
	u := a.adminTargetUser(w, r)
	if u == nil {
		return
	}
	if err := a.Store.SetAgentPasswordHash(u.ID, ""); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, flashURL("/admin/agents", "ok", "Пароль сброшен — агент задаст новый в профиле Mini App", ""), http.StatusSeeOther)
}
