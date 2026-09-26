package handlers

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

// Закрытые без доступа страницы (этап 3).
var closedPages = []struct{ method, path string }{
	{"GET", "/"}, {"GET", "/diary"}, {"POST", "/diary"}, {"GET", "/today/skip"}, {"POST", "/today/skip"},
	{"GET", "/progress"}, {"GET", "/community"}, {"POST", "/friends/add"}, {"GET", "/reports/weekly"},
}

func (a *App) request(t *testing.T, tgID, method, path string) resp {
	t.Helper()
	if method == "POST" {
		return a.post(t, tgID, path, url.Values{"emoji": {"3"}})
	}
	return a.get(t, tgID, path)
}

func TestTrialGivesThreeDays(t *testing.T) {
	a := newDBApp(t)
	a.newPlayer(t, "1", "")
	registered := time.Now()

	for _, offset := range []time.Duration{0, 47 * time.Hour, 72*time.Hour - time.Minute} {
		a.Now = func() time.Time { return registered.Add(offset) }
		if r := a.get(t, "1", "/"); r.Code != 200 {
			t.Errorf("через %v: главная %d %q", offset, r.Code, r.Location)
		}
	}

	a.Now = func() time.Time { return registered.Add(72*time.Hour + time.Minute) }
	for _, p := range closedPages {
		if r := a.request(t, "1", p.method, p.path); r.Location != "/plans?expired=1" {
			t.Errorf("%s %s после пробного: %d %q", p.method, p.path, r.Code, r.Location)
		}
	}
	for _, p := range []string{"/profile", "/wallet", "/plans"} {
		if r := a.get(t, "1", p); r.Code != 200 {
			t.Errorf("%s без подписки: %d %q", p, r.Code, r.Location)
		}
	}
	mustContain(t, a.get(t, "1", "/plans?expired=1").Body, "Три дня позади. Продолжим?", "Выбор большинства", "Условия партнёрской программы")
	if u := a.user(t, "1"); u.CompletedCount != 0 {
		t.Errorf("закрытый POST /diary засчитал задание")
	}
}

func TestSubscriptionOpensAccessAfterTrial(t *testing.T) {
	a := newDBApp(t)
	a.newPlayer(t, "1", "")
	a.Now = func() time.Time { return time.Now().Add(10 * 24 * time.Hour) }
	if r := a.get(t, "1", "/"); r.Location != "/plans?expired=1" {
		t.Fatalf("до оплаты: %q", r.Location)
	}
	// Подписка активируется на месяц от a.Now(), проверка срока — по часам сервера.
	a.Now = nil
	a.post(t, "1", "/subscribe", url.Values{"tier": {"plus369"}})
	a.Now = func() time.Time { return time.Now().Add(10 * 24 * time.Hour) }
	for _, p := range closedPages {
		if p.method != "GET" {
			continue
		}
		if r := a.get(t, "1", p.path); r.Location == "/plans?expired=1" {
			t.Errorf("%s с подпиской закрыт", p.path)
		}
	}
	mustContain(t, a.get(t, "1", "/plans").Body, "Твой тариф: Plus до", "Продлить Plus")
}

func TestPromoOpensAccessAfterTrial(t *testing.T) {
	a := newDBApp(t)
	a.newPlayer(t, "1", "")
	a.Now = func() time.Time { return time.Now().Add(10 * 24 * time.Hour) }
	a.post(t, "1", "/promo", url.Values{"code": {"lvl100"}})
	if r := a.get(t, "1", "/progress"); r.Code != 200 {
		t.Errorf("после промокода доступ закрыт: %d %q", r.Code, r.Location)
	}
}

func TestQuizOpenAfterTrial(t *testing.T) {
	a := newDBApp(t)
	a.onboard(t, "1", "")
	a.Now = func() time.Time { return time.Now().Add(10 * 24 * time.Hour) }
	if r := a.get(t, "1", "/quiz/0"); r.Code != 200 {
		t.Errorf("анкета закрыта: %d %q", r.Code, r.Location)
	}
}

func TestTrialBannerAndPlansStates(t *testing.T) {
	a := newDBApp(t)
	a.newPlayer(t, "1", "")
	mustContain(t, a.get(t, "1", "/").Body, "Пробный период: осталось 3 дн.", `href="/plans"`)
	mustContain(t, a.get(t, "1", "/plans").Body, "Осталось 3 дн. пробного периода", "888 ₽", "369 ₽", "20%", "50%", "5%", "10%")

	a.post(t, "1", "/subscribe", url.Values{"tier": {"premium888"}})
	mustNotContain(t, a.get(t, "1", "/").Body, "Пробный период")
}

func TestTermsPublic(t *testing.T) {
	a := newTestApp(t)
	r := a.do(t, "GET", "/terms", nil, "")
	if r.Code != http.StatusOK {
		t.Fatalf("/terms без входа: %d", r.Code)
	}
	mustContain(t, r.Body, "3. Партнёрская программа", "Premium 888 ₽", "20%", "50%", "15-го", "500 ₽", "НДФЛ 13%")
}
