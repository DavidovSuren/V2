package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestOnboardingSavesTermsAcceptance(t *testing.T) {
	a := newDBApp(t)
	a.onboard(t, "1", "")
	var at, version string
	a.Store.DB.QueryRow(`SELECT COALESCE(terms_accepted_at,''), COALESCE(terms_version,'') FROM users WHERE tg_id='1'`).Scan(&at, &version)
	if at == "" || version != TermsVersion {
		t.Errorf("согласие не сохранено: %q %q", at, version)
	}
}

func TestTermsUpdateScreen(t *testing.T) {
	a := newDBApp(t)
	a.newPlayer(t, "1", "")
	a.exec(t, "UPDATE users SET terms_version='2020-01-01' WHERE tg_id='1'")

	for _, p := range []string{"/", "/profile", "/wallet", "/plans"} {
		r := a.get(t, "1", p)
		if r.Code != 200 || !strings.Contains(r.Body, "Мы обновили соглашение") {
			t.Errorf("%s со старой редакцией: %d %q", p, r.Code, r.Location)
		}
	}
	if r := a.get(t, "1", "/terms"); !strings.Contains(r.Body, "Редакция от "+TermsVersion) {
		t.Error("/terms закрыт экраном обновления")
	}

	mustContain(t, a.post(t, "1", "/terms/accept", url.Values{}).Body, "Мы обновили соглашение")
	if r := a.post(t, "1", "/terms/accept", url.Values{"terms": {"1"}}); r.Location != "/" {
		t.Fatalf("принятие: %d %q", r.Code, r.Location)
	}
	if u := a.user(t, "1"); u.TermsVersion.String != TermsVersion {
		t.Errorf("версия: %v", u.TermsVersion)
	}
	mustNotContain(t, a.get(t, "1", "/").Body, "Мы обновили соглашение")
}

func TestTermsPageSections(t *testing.T) {
	a := newTestApp(t)
	r := a.do(t, "GET", "/terms", nil, "")
	if r.Code != http.StatusOK {
		t.Fatalf("/terms: %d", r.Code)
	}
	mustContain(t, r.Body, "Редакция от "+TermsVersion,
		"1. Кто мы и о сервисе", "2. Пробный период и подписки", "3. Партнёрская программа", "4. Вывод средств",
		"5. Какие данные мы собираем", "6. Промокоды", "7. Ответственность", "8. Контакты",
		"Plus 369 ₽", "Premium 888 ₽", "20%", "50%", "НДФЛ 13%", "500 ₽", SupportContact)
}

func TestWelcomeScreen(t *testing.T) {
	a := newTestApp(t)
	body := a.do(t, "GET", "/", nil, "").Body
	mustContain(t, body, "Новая версия себя за 365 дней", "3 дня бесплатно. Без привязки карты.",
		"Начать — 21 вопрос, 3 минуты", `name="terms"`, `href="/terms"`, `class="age-grid"`, "36–45")
	mustNotContain(t, body, `type="file"`, "фото")
}

// Имя из Telegram подставляется в форму приветствия.
func TestWelcomeNameFromTelegram(t *testing.T) {
	a := newDBApp(t)
	body := signedInitData(map[string]string{"auth_date": "1700000000", "user": `{"id":321,"first_name":"Сурен"}`})
	r := a.do(t, "POST", "/auth/bootstrap", strings.NewReader(body), "")
	c := cookieByName(r.Cookies, "v2_first_name")
	if c == nil {
		t.Fatal("нет cookie с именем")
	}
	w := a.do(t, "GET", "/", nil, "", a.session("321"), c)
	mustContain(t, w.Body, `value="Сурен"`, "Взяли из Telegram — можно поменять")
}

func TestReferralLinks(t *testing.T) {
	a := &App{}
	if got := a.referralLink("K7QX2PD"); got != "K7QX2PD" {
		t.Errorf("без бота и адреса: %q", got)
	}
	a.PublicURL = "https://v2.example/"
	if got := a.referralLink("K7QX2PD"); got != "https://v2.example/?ref=K7QX2PD" {
		t.Errorf("без бота: %q", got)
	}
	a.BotUsername = "version20bot"
	if got := a.referralLink("K7QX2PD"); got != "https://t.me/version20bot?start=K7QX2PD" {
		t.Errorf("без Mini App: %q", got)
	}
	a.MiniAppShortName = "app"
	if got := a.referralLink("K7QX2PD"); got != "https://t.me/version20bot/app?startapp=K7QX2PD" {
		t.Errorf("Mini App: %q", got)
	}
	share := shareURL("https://t.me/version20bot/app?startapp=K7QX2PD")
	u, err := url.Parse(share)
	if err != nil || u.Host != "t.me" || u.Path != "/share/url" || u.Query().Get("url") != "https://t.me/version20bot/app?startapp=K7QX2PD" ||
		!strings.HasPrefix(u.Query().Get("text"), "Я прокачиваю себя по одному действию в день") {
		t.Errorf("share: %q", share)
	}
}
