package handlers

import (
	"net/url"
	"strings"
	"testing"
)

func TestThemeSwitch(t *testing.T) {
	a := newTestApp(t)
	post := func(form url.Values) resp {
		return a.do(t, "POST", "/theme", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	}
	for _, theme := range []string{"dark", "light", "auto"} {
		r := post(url.Values{"theme": {theme}, "back": {"/profile"}})
		c := cookieByName(r.Cookies, "v2_theme")
		if r.Location != "/profile" || c == nil || c.Value != theme || c.HttpOnly {
			t.Errorf("%s: %q %+v", theme, r.Location, c)
		}
	}
	if c := cookieByName(post(url.Values{"theme": {"<x>"}}).Cookies, "v2_theme"); c.Value != "dark" {
		t.Errorf("неизвестная тема: %q", c.Value)
	}
	for _, back := range []string{"https://evil.example", "//evil.example", ""} {
		if r := post(url.Values{"theme": {"light"}, "back": {back}}); r.Location != "/profile" {
			t.Errorf("открытый редирект %q → %q", back, r.Location)
		}
	}
}

func TestDesignSystemLayout(t *testing.T) {
	a := newTestApp(t)
	body := a.do(t, "GET", "/", nil, "").Body
	mustContain(t, body, "fonts.googleapis.com/css2?family=Onest", "Playfair+Display", "v2_theme", `data-theme`,
		`<symbol id="i-diamond"`, `<symbol id="i-flame"`)
	// Скрипт темы стоит до CSS, чтобы не мигало.
	if strings.Index(body, "v2_theme") > strings.Index(body, "/static/style.css") {
		t.Error("скрипт темы после CSS")
	}
	css := a.do(t, "GET", "/static/style.css", nil, "").Body
	mustContain(t, css, "#0F1014", "#F6F4EF", "#D4B06A", "#B08A43", "@view-transition", `[data-theme="light"]`)
}

func TestNavFourTabs(t *testing.T) {
	a := newDBApp(t)
	a.newPlayer(t, "1", "")
	body := a.get(t, "1", "/profile").Body
	nav := body[strings.Index(body, `<nav class="bottom-nav">`):]
	nav = nav[:strings.Index(nav, "</nav>")]
	if n := strings.Count(nav, "nav-item"); n != 4 {
		t.Errorf("вкладок: %d, want 4", n)
	}
	mustContain(t, nav, ">Сегодня", ">Путь", ">Друзья", ">Я", `href="/progress"`, `href="/community"`)
	mustNotContain(t, nav, "/diary", "🏠", "📊")
	mustContain(t, body, `action="/theme"`, "Светлая", "Тёмная", "Как в Telegram")
}
