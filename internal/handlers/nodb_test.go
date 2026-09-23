package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"
)

func signedInitData(fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+fields[k])
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(testBotToken))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(pairs, "\n")))

	v := url.Values{}
	for k, val := range fields {
		v.Set(k, val)
	}
	v.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return v.Encode()
}

func cookieByName(cs []*http.Cookie, name string) *http.Cookie {
	for _, c := range cs {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestHealth(t *testing.T) {
	a := newTestApp(t)
	r := a.do(t, http.MethodGet, "/api/health", nil, "")
	if r.Code != 200 || r.Body != `{"ok":true}` {
		t.Fatalf("%d %s", r.Code, r.Body)
	}
}

func TestStaticServed(t *testing.T) {
	a := newTestApp(t)
	r := a.do(t, http.MethodGet, "/static/bootstrap.js", nil, "")
	if r.Code != 200 || !strings.Contains(r.Body, "/auth/bootstrap") {
		t.Fatalf("%d", r.Code)
	}
}

func TestIndexWithoutSessionShowsBootstrap(t *testing.T) {
	a := newTestApp(t)
	r := a.do(t, http.MethodGet, "/", nil, "")
	if r.Code != 200 {
		t.Fatalf("status %d", r.Code)
	}
	mustContain(t, r.Body, `id="bootstrap-target"`, `action="/onboarding"`)
}

// Все защищённые страницы без сессии уводят на "/", а не падают.
func TestProtectedRoutesRedirectWithoutSession(t *testing.T) {
	a := newTestApp(t)
	for _, rt := range []struct{ method, path string }{
		{"GET", "/quiz/0"}, {"POST", "/quiz/0"}, {"GET", "/today/skip"}, {"POST", "/today/skip"},
		{"GET", "/diary"}, {"POST", "/diary"}, {"GET", "/progress"}, {"GET", "/achievements"},
		{"GET", "/community"}, {"POST", "/friends/add"}, {"GET", "/profile"}, {"POST", "/subscribe"},
		{"POST", "/promo"}, {"POST", "/logout"}, {"GET", "/reports/weekly"}, {"GET", "/wallet"},
		{"POST", "/wallet/withdraw"},
	} {
		r := a.do(t, rt.method, rt.path, nil, "")
		if r.Code != http.StatusSeeOther || r.Location != "/" {
			t.Errorf("%s %s: %d → %q", rt.method, rt.path, r.Code, r.Location)
		}
	}

	// Подделанная cookie тоже не пускает.
	forged := &http.Cookie{Name: "v2_session", Value: "1000.forged"}
	if r := a.do(t, "GET", "/profile", nil, "", forged); r.Location != "/" {
		t.Errorf("поддельная сессия принята: %d %q", r.Code, r.Location)
	}
}

func TestAdminRequiresOwner(t *testing.T) {
	a := newTestApp(t)
	if r := a.do(t, "POST", "/admin/grant-premium-agent", nil, ""); r.Code != http.StatusForbidden {
		t.Errorf("без сессии: %d", r.Code)
	}
	if r := a.post(t, "42", "/admin/grant-premium-agent", url.Values{"username": {"x"}}); r.Code != http.StatusForbidden {
		t.Errorf("не владелец: %d", r.Code)
	}
	a.AdminTgID = ""
	if r := a.post(t, "", "/admin/grant-premium-agent", url.Values{"username": {"x"}}); r.Code != http.StatusForbidden {
		t.Errorf("ADMIN_TG_ID не задан: %d", r.Code)
	}
}

func TestAdminValidatesInput(t *testing.T) {
	a := newTestApp(t)
	r := a.post(t, testAdminID, "/admin/grant-premium-agent", url.Values{})
	if r.Code != http.StatusBadRequest {
		t.Errorf("без username/tgId: %d", r.Code)
	}
}

func TestBootstrapRejectsBadInitData(t *testing.T) {
	a := newTestApp(t)
	for _, body := range []string{"", "garbage", "debug:5"} {
		r := a.do(t, "POST", "/auth/bootstrap", strings.NewReader(body), "")
		if r.Code != http.StatusUnauthorized || cookieByName(r.Cookies, "v2_session") != nil {
			t.Errorf("%q: %d", body, r.Code)
		}
	}
}

func TestBootstrapDevFakeAuth(t *testing.T) {
	a := newTestApp(t)
	a.DevFakeAuth = true
	r := a.do(t, "POST", "/auth/bootstrap", strings.NewReader("debug:555"), "")
	c := cookieByName(r.Cookies, "v2_session")
	if r.Code != 200 || c == nil {
		t.Fatalf("%d %v", r.Code, r.Cookies)
	}
	if id, ok := a.Sessions.Verify(c.Value); !ok || id != "555" {
		t.Errorf("сессия: %q %v", id, ok)
	}
	if r := a.do(t, "POST", "/auth/bootstrap", strings.NewReader("debug:"), ""); r.Code != http.StatusUnauthorized {
		t.Errorf("пустой debug id принят: %d", r.Code)
	}
}

// Без username бутстрап не трогает БД — проверяем сессию и реф. cookie.
func TestBootstrapValidInitDataSetsSessionAndRef(t *testing.T) {
	a := newTestApp(t)
	body := signedInitData(map[string]string{
		"auth_date":   "1700000000",
		"start_param": "REF1234",
		"user":        `{"id":77,"first_name":"Ivan"}`,
	})
	r := a.do(t, "POST", "/auth/bootstrap", strings.NewReader(body), "")
	if r.Code != 200 {
		t.Fatalf("%d %s", r.Code, r.Body)
	}
	c := cookieByName(r.Cookies, "v2_session")
	if id, ok := a.Sessions.Verify(c.Value); !ok || id != "77" {
		t.Errorf("сессия: %q", id)
	}
	if ref := cookieByName(r.Cookies, "v2_ref"); ref == nil || ref.Value != "REF1234" {
		t.Errorf("реф. cookie: %v", ref)
	}

	// Реферальный код из start_param подставляется в приветственную форму.
	w := a.do(t, "GET", "/", nil, "", &http.Cookie{Name: "v2_ref", Value: "REF1234"})
	mustContain(t, w.Body, `name="refCode" value="REF1234"`)
}

func TestIsYesterday(t *testing.T) {
	ns := func(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }
	cases := []struct {
		last  sql.NullString
		today string
		want  bool
	}{
		{ns("2026-09-22"), "2026-09-23", true},
		{ns("2026-09-23"), "2026-09-23", false},
		{ns("2026-09-21"), "2026-09-23", false},
		{ns("2026-02-28"), "2026-03-01", true},
		{ns("2025-12-31"), "2026-01-01", true},
		{sql.NullString{}, "2026-09-23", false},
		{ns("мусор"), "2026-09-23", false},
	}
	for _, c := range cases {
		if got := isYesterday(c.last, c.today); got != c.want {
			t.Errorf("%v → %s: %v, want %v", c.last, c.today, got, c.want)
		}
	}
}

func TestSmallHelpers(t *testing.T) {
	if dayLabel(0) != "День 1 из 365" || dayLabel(364) != "День 365 из 365" {
		t.Error("dayLabel")
	}
	for s, want := range map[string]int{"5": 5, "12": 12, "": 0, "3x": 3} {
		if atoiSafe(s) != want {
			t.Errorf("atoiSafe(%q)", s)
		}
	}
	if itoa(0) != "0" || itoa(365) != "365" {
		t.Error("itoa")
	}
	if mustJSON(`a"b`) != `"a\"b"` {
		t.Error("mustJSON")
	}
}

func TestCategoryTotalsKeepsBankOrder(t *testing.T) {
	a := newTestApp(t)
	for _, g := range []string{"male", "female"} {
		totals := a.CategoryTotals(g)
		if len(totals) != 8 {
			t.Fatalf("%s: %d направлений", g, len(totals))
		}
		sum := 0
		seen := map[string]bool{}
		var order []string
		for _, task := range a.TaskBankFor(g) {
			if !seen[task.Category] {
				seen[task.Category] = true
				order = append(order, task.Category)
			}
		}
		for i, ct := range totals {
			if ct.Category != order[i] {
				t.Errorf("%s: позиция %d = %s, want %s", g, i, ct.Category, order[i])
			}
			sum += ct.Total
		}
		if sum != 365 {
			t.Errorf("%s: сумма %d", g, sum)
		}
	}
	if a.CategoryTotals("other") != nil {
		t.Error("неизвестный пол")
	}
}

// Раньше форма приветствия без сессии (страница открыта не из Telegram)
// молча редиректила на "/" и очищалась — пользователь не понимал, что случилось.
func TestOnboardingWithoutSessionExplainsAndKeepsInput(t *testing.T) {
	a := newTestApp(t)
	form := url.Values{"name": {"Анна"}, "ageGroup": {"26-30"}}
	r := a.do(t, "POST", "/onboarding", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if r.Code != 200 {
		t.Fatalf("status %d → %q", r.Code, r.Location)
	}
	mustContain(t, r.Body, "Открой приложение через бота в Telegram", `value="Анна"`,
		`value="26-30" checked`, `id="bootstrap-target"`)
	mustNotContain(t, r.Body, "data-dev-auth")
}

func TestWelcomeDevAuthFlag(t *testing.T) {
	a := newTestApp(t)
	mustNotContain(t, a.do(t, "GET", "/", nil, "").Body, "data-dev-auth")
	a.DevFakeAuth = true
	mustContain(t, a.do(t, "GET", "/", nil, "").Body, `data-dev-auth="1"`)
}
