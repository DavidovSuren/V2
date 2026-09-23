package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	s := NewSessions("secret")
	id, ok := s.Verify(s.sign("12345"))
	if !ok || id != "12345" {
		t.Fatalf("got %q, %v", id, ok)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	s := NewSessions("secret")
	token := s.sign("12345")

	for name, bad := range map[string]string{
		"пусто":           "",
		"без подписи":     "12345",
		"подменён tg_id":  "99999" + token[len("12345"):],
		"битая подпись":   token + "x",
		"чужой секрет":    NewSessions("other").sign("12345"),
		"пустая подпись":  "12345.",
		"только точка":    ".",
		"мусор в подписи": "12345.AAAA",
	} {
		if _, ok := s.Verify(bad); ok {
			t.Errorf("%s: принято %q", name, bad)
		}
	}
}

func TestCookieRoundTrip(t *testing.T) {
	s := NewSessions("secret")
	rec := httptest.NewRecorder()
	s.SetCookie(rec, "777")

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies: %v", cookies)
	}
	c := cookies[0]
	if c.Name != CookieName || !c.HttpOnly || !c.Secure || c.Path != "/" {
		t.Errorf("атрибуты cookie: %+v", c)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)
	if id, ok := s.TgIDFromRequest(req); !ok || id != "777" {
		t.Errorf("TgIDFromRequest: %q, %v", id, ok)
	}

	if _, ok := s.TgIDFromRequest(httptest.NewRequest(http.MethodGet, "/", nil)); ok {
		t.Error("запрос без cookie принят")
	}
}

func TestClearCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	NewSessions("secret").ClearCookie(rec)
	c := rec.Result().Cookies()[0]
	if c.Name != CookieName || c.MaxAge >= 0 || c.Value != "" {
		t.Errorf("cookie не удалена: %+v", c)
	}
}
