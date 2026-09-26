package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"version20/internal/auth"
	"version20/internal/db"
	"version20/internal/models"
	"version20/internal/store"
)

const (
	testBotToken = "123456:TEST-token"
	testAdminID  = "1000"
)

// Шаблоны и банк заданий читаются с диска теми же путями, что и go:embed
// в cmd/server/main.go.
var serverDir = filepath.Join("..", "..", "cmd", "server")

func testTemplates(t *testing.T) map[string]*template.Template {
	t.Helper()
	fsys := os.DirFS(serverDir)
	pages, err := fs.Glob(fsys, "web/templates/pages/*.html")
	if err != nil || len(pages) == 0 {
		t.Fatalf("шаблоны не найдены: %v", err)
	}
	partials, _ := fs.Glob(fsys, "web/templates/partials/*.html")
	funcs := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"seq": func(from, to int) []int {
			out := []int{}
			for i := from; i <= to; i++ {
				out = append(out, i)
			}
			return out
		},
	}
	out := map[string]*template.Template{}
	for _, p := range pages {
		files := append([]string{"web/templates/layout.html"}, partials...)
		files = append(files, p)
		tm, err := template.New(filepath.Base(p)).Funcs(funcs).ParseFS(fsys, files...)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		out[filepath.Base(p)] = tm
	}
	panel, _ := fs.Glob(fsys, "web/templates/panel/*.html")
	for _, p := range panel {
		if filepath.Base(p) == "layout.html" {
			continue
		}
		tm, err := template.New(filepath.Base(p)).Funcs(funcs).ParseFS(fsys, "web/templates/panel/layout.html", p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		out["panel/"+filepath.Base(p)] = tm
	}
	return out
}

func testBank(t *testing.T, gender string) []models.Task {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(serverDir, "data", "tasks_"+gender+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var tasks []models.Task
	if err := json.Unmarshal(b, &tasks); err != nil {
		t.Fatal(err)
	}
	return tasks
}

// newTestApp — App без БД: годится для маршрутов, которые до БД не доходят.
func newTestApp(t *testing.T) *App {
	return &App{
		Sessions:    auth.NewSessions("test-secret"),
		Tmpl:        testTemplates(t),
		StaticFS:    os.DirFS(filepath.Join(serverDir, "web", "static")),
		TasksMale:   testBank(t, "male"),
		TasksFemale: testBank(t, "female"),
		BotToken:    testBotToken,
		AdminTgID:   testAdminID,
		// Старые сценарии оформляют подписку сразу, без Telegram.
		Payments: PaymentConfig{TestMode: true},
		// Сообщения бота в тестах никуда не уходят.
		Notify: func(string, string) {},
	}
}

// newDBApp — App поверх настоящего PostgreSQL из TEST_DATABASE_URL, каждый
// тест в своей схеме (search_path), которая удаляется после теста.
// Пример: TEST_DATABASE_URL=postgres://v2@localhost:5432/postgres?sslmode=disable
func newDBApp(t *testing.T) *App {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL не задан — пропускаю тест с PostgreSQL")
	}

	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("t_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("CREATE SCHEMA: %v", err)
	}
	t.Cleanup(func() {
		admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		admin.Close()
	})

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	conn, err := db.Open(dsn + sep + "search_path=" + schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}

	a := newTestApp(t)
	a.Store = store.New(conn)
	return a
}

type resp struct {
	Code     int
	Location string
	Body     string
	Cookies  []*http.Cookie
}

func (a *App) do(t *testing.T, method, target string, body io.Reader, contentType string, cookies ...*http.Cookie) resp {
	t.Helper()
	req := httptest.NewRequest(method, target, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	a.Routes().ServeHTTP(rec, req)
	return resp{
		Code:     rec.Code,
		Location: rec.Header().Get("Location"),
		Body:     rec.Body.String(),
		Cookies:  rec.Result().Cookies(),
	}
}

func (a *App) doWithHeader(t *testing.T, method, target, body string, headers map[string]string) resp {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	a.Routes().ServeHTTP(rec, req)
	return resp{Code: rec.Code, Location: rec.Header().Get("Location"), Body: rec.Body.String(), Cookies: rec.Result().Cookies()}
}

func (a *App) session(tgID string) *http.Cookie {
	rec := httptest.NewRecorder()
	a.Sessions.SetCookie(rec, tgID)
	return rec.Result().Cookies()[0]
}

func (a *App) get(t *testing.T, tgID, target string) resp {
	t.Helper()
	return a.do(t, http.MethodGet, target, nil, "", a.session(tgID))
}

func (a *App) post(t *testing.T, tgID, target string, form url.Values) resp {
	t.Helper()
	return a.do(t, http.MethodPost, target, strings.NewReader(form.Encode()),
		"application/x-www-form-urlencoded", a.session(tgID))
}

// onboard проходит приветственный экран (имя, возраст, 1 фото, реф. код).
func (a *App) onboard(t *testing.T, tgID, refCode string, extra ...*http.Cookie) resp {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("name", "User "+tgID)
	mw.WriteField("ageGroup", "25-34")
	mw.WriteField("refCode", refCode)
	fw, _ := mw.CreateFormFile("photos", "me.jpg")
	fw.Write([]byte("jpeg"))
	mw.Close()
	cookies := append([]*http.Cookie{a.session(tgID)}, extra...)
	return a.do(t, http.MethodPost, "/onboarding", &buf, mw.FormDataContentType(), cookies...)
}

// answersFor — ответы на все 21 вопрос: пол + середина шкалы + варианты.
func answersFor(gender string) map[int]string {
	out := map[int]string{}
	for _, q := range models.Questions {
		switch {
		case q.ID == 0:
			out[q.ID] = gender
		case q.Type == "scale":
			out[q.ID] = fmt.Sprint(q.ID%10 + 1)
		case q.Type == "single":
			out[q.ID] = q.Options[q.ID%len(q.Options)].Value
		default:
			out[q.ID] = "ответ " + fmt.Sprint(q.ID)
		}
	}
	return out
}

func (a *App) completeQuiz(t *testing.T, tgID, gender string) resp {
	t.Helper()
	answers := answersFor(gender)
	var last resp
	for _, q := range models.Questions {
		last = a.post(t, tgID, fmt.Sprintf("/quiz/%d", q.ID), url.Values{"answer": {answers[q.ID]}})
		if last.Code != http.StatusSeeOther {
			t.Fatalf("quiz %d: status %d: %s", q.ID, last.Code, last.Body)
		}
	}
	return last
}

// newPlayer — зарегистрирован и прошёл анкету.
func (a *App) newPlayer(t *testing.T, tgID, refCode string) *models.User {
	t.Helper()
	if r := a.onboard(t, tgID, refCode); r.Location != "/quiz/0" {
		t.Fatalf("onboarding %s: %d %s %s", tgID, r.Code, r.Location, r.Body)
	}
	a.completeQuiz(t, tgID, "male")
	return a.user(t, tgID)
}

func (a *App) user(t *testing.T, tgID string) *models.User {
	t.Helper()
	u, err := a.Store.GetUserByTgID(tgID)
	if err != nil || u == nil {
		t.Fatalf("пользователь %s: %v", tgID, err)
	}
	return u
}

func (a *App) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := a.Store.DB.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func (a *App) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := a.Store.DB.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func mustContain(t *testing.T, body string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if !strings.Contains(body, s) {
			t.Errorf("в ответе нет %q", s)
		}
	}
}

func mustNotContain(t *testing.T, body string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if strings.Contains(body, s) {
			t.Errorf("в ответе не должно быть %q", s)
		}
	}
}
