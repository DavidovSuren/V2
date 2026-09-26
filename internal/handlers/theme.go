package handlers

import (
	"net/http"
	"strings"
)

const themeCookie = "v2_theme"

var themes = map[string]bool{"dark": true, "light": true, "auto": true}

// handleTheme — переключатель темы в профиле: dark | light | auto (как в
// Telegram). Cookie читает скрипт в <head>, поэтому она не HttpOnly.
func (a *App) handleTheme(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	theme := r.FormValue("theme")
	if !themes[theme] {
		theme = "dark"
	}
	http.SetCookie(w, &http.Cookie{
		Name: themeCookie, Value: theme, Path: "/",
		MaxAge: 365 * 24 * 3600, SameSite: http.SameSiteLaxMode,
	})
	back := r.FormValue("back")
	if !strings.HasPrefix(back, "/") || strings.HasPrefix(back, "//") {
		back = "/profile"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// themePref — выбранная тема (для отметки в переключателе).
func themePref(r *http.Request) string {
	if c, err := r.Cookie(themeCookie); err == nil && themes[c.Value] {
		return c.Value
	}
	return "dark"
}
