package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"version20/internal/content"
	"version20/internal/models"
)

const (
	testAdminLogin    = "boss"
	testAdminPassword = "s3cret-pass"
)

func withPanels(a *App) *App {
	a.AdminLogin, a.AdminPassword = testAdminLogin, testAdminPassword
	a.AdminSessions = a.Sessions.Scoped("admin:"+testAdminPassword, "v2_admin", time.Hour)
	a.PartnerSessions = a.Sessions.Scoped("partner", "v2_partner", time.Hour)
	return a
}

// withContent подключает вопросы/задания из БД (как в cmd/server).
func withContent(t *testing.T, a *App) *App {
	t.Helper()
	c, err := content.Load(a.Store, models.Questions, a.TasksMale, a.TasksFemale)
	if err != nil {
		t.Fatal(err)
	}
	a.Content = c
	return a
}

func (a *App) adminCookie() *http.Cookie {
	rec := httptest.NewRecorder()
	a.AdminSessions.SetCookie(rec, a.AdminLogin)
	return rec.Result().Cookies()[0]
}

func (a *App) adminGet(t *testing.T, target string) resp {
	t.Helper()
	return a.do(t, http.MethodGet, target, nil, "", a.adminCookie())
}

func (a *App) adminPost(t *testing.T, target string, form url.Values) resp {
	t.Helper()
	return a.do(t, http.MethodPost, target, strings.NewReader(form.Encode()),
		"application/x-www-form-urlencoded", a.adminCookie())
}

func formPost(t *testing.T, a *App, target string, form url.Values, cookies ...*http.Cookie) resp {
	t.Helper()
	return a.do(t, http.MethodPost, target, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", cookies...)
}

func TestAdminAccessWithoutDB(t *testing.T) {
	a := newTestApp(t)

	// Без ADMIN_LOGIN/ADMIN_PASSWORD админка выключена.
	mustContain(t, a.do(t, "GET", "/admin/login", nil, "").Body, "Админка выключена")
	if r := a.do(t, "GET", "/admin", nil, ""); r.Location != "/admin/login" {
		t.Fatalf("выключенная админка: %d %q", r.Code, r.Location)
	}

	withPanels(a)
	for _, p := range []string{"/admin", "/admin/questions", "/admin/tasks", "/admin/users", "/admin/users/1"} {
		if r := a.do(t, "GET", p, nil, ""); r.Location != "/admin/login" {
			t.Errorf("%s без входа: %d %q", p, r.Code, r.Location)
		}
	}
	// Пользовательская сессия Mini App к админке не подходит.
	if r := a.do(t, "GET", "/admin", nil, "", a.session(testAdminLogin)); r.Location != "/admin/login" {
		t.Errorf("сессия Mini App пустила в админку")
	}
	// Как и чужая подпись: cookie кабинета партнёра с тем же значением.
	rec := httptest.NewRecorder()
	a.PartnerSessions.SetCookie(rec, testAdminLogin)
	pc := rec.Result().Cookies()[0]
	pc.Name = "v2_admin"
	if r := a.do(t, "GET", "/admin", nil, "", pc); r.Location != "/admin/login" {
		t.Errorf("подпись другой области пустила в админку")
	}
	if r := a.do(t, "POST", "/admin/users/1/reset-partner-password", nil, ""); r.Location != "/admin/login" {
		t.Errorf("POST без входа: %d %q", r.Code, r.Location)
	}
	if r := a.do(t, "GET", "/partner", nil, ""); r.Location != "/partner/login" {
		t.Errorf("/partner без входа: %d %q", r.Code, r.Location)
	}
	mustContain(t, a.do(t, "GET", "/partner/login", nil, "").Body, "Кабинет партнёра")
	if r := a.do(t, "GET", "/admin/agents", nil, "", a.adminCookie()); r.Code == http.StatusOK && strings.Contains(r.Body, "Агенты") {
		t.Error("раздел «Агенты» ещё существует")
	}
}

func TestAdminLogin(t *testing.T) {
	a := withPanels(newDBApp(t))

	bad := formPost(t, a, "/admin/login", url.Values{"login": {testAdminLogin}, "password": {"wrong"}})
	mustContain(t, bad.Body, "Неверный логин или пароль")
	if len(bad.Cookies) != 0 {
		t.Fatal("cookie при неверном пароле")
	}

	ok := formPost(t, a, "/admin/login", url.Values{"login": {testAdminLogin}, "password": {testAdminPassword}})
	if ok.Location != "/admin" || len(ok.Cookies) == 0 {
		t.Fatalf("вход: %d %q", ok.Code, ok.Location)
	}
	dash := a.do(t, "GET", "/admin", nil, "", ok.Cookies[0])
	if dash.Code != 200 {
		t.Fatalf("дашборд: %d", dash.Code)
	}
	mustContain(t, dash.Body, "пользователей", "активный Premium")

	// Смена пароля админа инвалидирует старые сессии.
	b := withPanels(newTestApp(t))
	b.Store = a.Store
	b.AdminPassword = "другой"
	b.AdminSessions = b.Sessions.Scoped("admin:другой", "v2_admin", time.Hour)
	if r := b.do(t, "GET", "/admin", nil, "", ok.Cookies[0]); r.Location != "/admin/login" {
		t.Fatal("старая сессия пережила смену пароля")
	}
}

func TestAdminEditQuestions(t *testing.T) {
	a := withContent(t, withPanels(newDBApp(t)))
	if r := a.onboard(t, "1", ""); r.Location != "/quiz/0" { // анкета ещё не пройдена
		t.Fatalf("onboarding: %q", r.Location)
	}

	mustContain(t, a.adminGet(t, "/admin/questions").Body, models.Questions[1].Text)

	r := a.adminPost(t, "/admin/questions/1", url.Values{"text": {"Как ты оцениваешь внешность сегодня?"}})
	if r.Code != http.StatusSeeOther || !strings.Contains(r.Location, "ok=") {
		t.Fatalf("сохранение: %d %q", r.Code, r.Location)
	}
	mustContain(t, a.get(t, "1", "/quiz/1").Body, "Как ты оцениваешь внешность сегодня?")

	// Варианты single-вопроса: новые подписи видны в анкете.
	a.adminPost(t, "/admin/questions/2", url.Values{"text": {models.Questions[2].Text}, "options": {"Ежедневно\nЧасто\nРедко\nНикогда"}})
	mustContain(t, a.get(t, "1", "/quiz/2").Body, `value="Ежедневно"`, "Никогда")

	// Вопрос про пол: значения фиксированы, меняются только подписи.
	a.adminPost(t, "/admin/questions/0", url.Values{"text": {"Кто ты?"}, "options": {"Парень\nДевушка"}})
	q0 := a.get(t, "1", "/quiz/0").Body
	mustContain(t, q0, `value="male"`, "Парень", `value="female"`, "Девушка")
	if r := a.adminPost(t, "/admin/questions/0", url.Values{"text": {"Кто ты?"}, "options": {"Один"}}); !strings.Contains(r.Location, "err=") {
		t.Errorf("вопрос про пол с 1 вариантом принят: %q", r.Location)
	}
	if r := a.adminPost(t, "/admin/questions/3", url.Values{"text": {"  "}}); !strings.Contains(r.Location, "err=") {
		t.Errorf("пустой текст принят: %q", r.Location)
	}

	// Анкета проходится до конца с новыми вариантами.
	answers := map[int]string{0: "male", 2: "Ежедневно"}
	for _, q := range a.Questions() {
		ans, ok := answers[q.ID]
		if !ok {
			switch q.Type {
			case "scale":
				ans = "5"
			case "single":
				ans = q.Options[0].Value
			default:
				ans = "текст"
			}
		}
		a.post(t, "1", "/quiz/"+itoa(q.ID), url.Values{"answer": {ans}})
	}
	if !a.user(t, "1").Gender.Valid {
		t.Fatal("анкета не завершилась")
	}
}

func TestAdminEditTasks(t *testing.T) {
	a := withContent(t, withPanels(newDBApp(t)))
	task := a.TasksMale[0]

	list := a.adminGet(t, "/admin/tasks?bank=male&cat="+url.QueryEscape(task.Category))
	mustContain(t, list.Body, task.ID)
	if a.adminGet(t, "/admin/tasks/male/nope").Code != http.StatusNotFound {
		t.Error("несуществующее задание")
	}

	// Пользователь с уже построенным планом.
	old := a.newPlayer(t, "1", "")
	a.exec(t, "UPDATE user_schedule SET task_id=$1, task_text=$2 WHERE user_id=$3 AND day_index=0", task.ID, task.Text, old.ID)

	r := a.adminPost(t, "/admin/tasks/male/"+task.ID, url.Values{"text": {"Новое задание"}, "why": {"Новый совет"}})
	if !strings.Contains(r.Location, "ok=") {
		t.Fatalf("сохранение: %q", r.Location)
	}
	mustContain(t, a.adminGet(t, "/admin/tasks/male/"+task.ID).Body, "Новое задание", "Новый совет")
	if got := a.findTask("male", task.ID); got == nil || got.Why != "Новый совет" {
		t.Fatalf("кэш не обновлён: %+v", got)
	}
	if r := a.adminPost(t, "/admin/tasks/male/"+task.ID, url.Values{"text": {"x"}, "why": {""}}); !strings.Contains(r.Location, "err=") {
		t.Errorf("пустое «почему» принято")
	}
	// Не-UTF-8 (например, cp1251 из консоли Windows) — ошибка формы, а не 500.
	if r := a.adminPost(t, "/admin/tasks/male/"+task.ID, url.Values{"text": {"\xd3\xec"}, "why": {"x"}}); !strings.Contains(r.Location, "err=") {
		t.Errorf("не-UTF-8: %d %q", r.Code, r.Location)
	}

	// Уже построенный план не меняется.
	var text string
	if err := a.Store.DB.QueryRow("SELECT task_text FROM user_schedule WHERE user_id=$1 AND day_index=0", old.ID).Scan(&text); err != nil {
		t.Fatal(err)
	}
	if text != task.Text {
		t.Errorf("старый план изменился: %q", text)
	}
	// Новый план строится из отредактированного банка.
	a.newPlayer(t, "2", "")
	var n int
	a.Store.DB.QueryRow("SELECT COUNT(*) FROM user_schedule s JOIN users u ON u.id=s.user_id WHERE u.tg_id='2' AND s.task_text='Новое задание'").Scan(&n)
	if n != 1 {
		t.Errorf("в новом плане нет отредактированного задания")
	}
}

func TestAdminUsers(t *testing.T) {
	a := withPanels(newDBApp(t))
	u := a.newPlayer(t, "1", "")
	a.exec(t, "UPDATE users SET username='ann' WHERE id=$1", u.ID)
	id := itoa(int(u.ID))

	mustContain(t, a.adminGet(t, "/admin/users?q=@ann").Body, "/admin/users/"+id)
	mustContain(t, a.adminGet(t, "/admin/users/"+id).Body, "Профиль и подписка")

	r := a.adminPost(t, "/admin/users/"+id, url.Values{"name": {"Анна"}, "username": {"@ann2"}, "tier": {"premium888"}, "expires": {"2099-12-31"}})
	if !strings.Contains(r.Location, "ok=") {
		t.Fatalf("сохранение пользователя: %q", r.Location)
	}
	got := a.user(t, "1")
	if got.Name != "Анна" || got.Username.String != "ann2" || got.SubscriptionTier != "premium888" ||
		!strings.HasPrefix(got.SubscriptionExpiresAt.String, "2099-12-31") {
		t.Fatalf("пользователь: %+v", got)
	}
	if r := a.adminPost(t, "/admin/users/"+id, url.Values{"name": {"Анна"}, "tier": {"plus369"}}); !strings.Contains(r.Location, "err=") {
		t.Error("платный тариф без даты принят")
	}
	if r := a.adminPost(t, "/admin/users/"+id, url.Values{"name": {"Анна"}, "tier": {"gold"}}); !strings.Contains(r.Location, "err=") {
		t.Error("неизвестный тариф принят")
	}

	// Карточка пользователя показывает партнёрские данные для всех.
	mustContain(t, a.adminGet(t, "/admin/users/"+id).Body, "Партнёр", got.ReferralCode.String, "reset-partner-password")
}

func TestPartnerCabinet(t *testing.T) {
	a := withPanels(newDBApp(t))
	partner := a.newPlayer(t, "1", "")
	a.post(t, "1", "/subscribe", url.Values{"tier": {"premium888"}})
	code := partner.ReferralCode.String

	// Приглашённый оплатил Premium — партнёру 20% = 178 ₽.
	a.newPlayer(t, "2", code)
	a.post(t, "2", "/subscribe", url.Values{"tier": {"premium888"}})

	// Пока пароль не задан — войти нельзя.
	login := url.Values{"code": {strings.ToLower(code)}, "password": {"partner-pass-1"}}
	mustContain(t, formPost(t, a, "/partner/login", login).Body, "Неверный код или пароль")

	// Пароль задаётся в профиле Mini App — у любого пользователя.
	mustContain(t, a.get(t, "1", "/profile").Body, `action="/partner/password"`)
	if r := a.post(t, "1", "/partner/password", url.Values{"password": {"short"}, "password2": {"short"}}); !strings.Contains(r.Location, "partner_pw=short") {
		t.Errorf("короткий пароль: %q", r.Location)
	}
	if r := a.post(t, "1", "/partner/password", url.Values{"password": {"partner-pass-1"}, "password2": {"partner-pass-2"}}); !strings.Contains(r.Location, "partner_pw=mismatch") {
		t.Errorf("несовпадение: %q", r.Location)
	}
	if r := a.post(t, "1", "/partner/password", url.Values{"password": {"partner-pass-1"}, "password2": {"partner-pass-1"}}); !strings.Contains(r.Location, "partner_pw=ok") {
		t.Fatalf("пароль: %q", r.Location)
	}

	mustContain(t, formPost(t, a, "/partner/login", url.Values{"code": {code}, "password": {"wrong-pass"}}).Body, "Неверный код или пароль")
	r := formPost(t, a, "/partner/login", login) // код в нижнем регистре тоже подходит
	if r.Location != "/partner" || len(r.Cookies) == 0 {
		t.Fatalf("вход партнёра: %d %q %s", r.Code, r.Location, r.Body)
	}
	pc := r.Cookies[0]
	dash := a.do(t, "GET", "/partner", nil, "", pc)
	mustContain(t, dash.Body, code, "178 ₽", "User 2", "premium888", "Кабинет партнёра")

	// Старый агентский код тоже подходит для входа.
	a.Store.SetPremiumAgentCode(partner.ID, "AGENT01")
	if r := formPost(t, a, "/partner/login", url.Values{"code": {"agent01"}, "password": {"partner-pass-1"}}); r.Location != "/partner" {
		t.Errorf("вход по агентскому коду: %q", r.Location)
	}

	// Сессия кабинета не пускает в Mini App и админку.
	mc := *pc
	mc.Name = "v2_session"
	if r := a.do(t, "GET", "/profile", nil, "", &mc); r.Location != "/" {
		t.Errorf("сессия кабинета подошла к Mini App: %d %q", r.Code, r.Location)
	}

	// Сброс пароля админом завершает сессию кабинета.
	a.adminPost(t, "/admin/users/"+itoa(int(partner.ID))+"/reset-partner-password", nil)
	if r := a.do(t, "GET", "/partner", nil, "", pc); r.Location != "/partner/login" {
		t.Errorf("сессия пережила сброс пароля: %d %q", r.Code, r.Location)
	}
}
