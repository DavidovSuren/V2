package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"version20/internal/models"
	"version20/internal/referrals"
	"version20/internal/store"
)

// Кабинет партнёра (/partner/...) — веб-версия кошелька партнёра: вход по
// реферальному коду (или старому агентскому — он работает как реферальный)
// и паролю. Пароль задаётся в профиле Mini App (POST /partner/password).
// В cookie лежит "<user_id>-<отпечаток хеша пароля>": смена или сброс
// пароля завершает все сессии кабинета.

const minPartnerPasswordLen = 8

func passwordFingerprint(hash string) string {
	sum := sha256.Sum256([]byte(hash))
	return hex.EncodeToString(sum[:8])
}

type partnerCtxKey struct{}

func (a *App) requirePartner(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.PartnerSessions == nil {
			http.Redirect(w, r, "/partner/login", http.StatusSeeOther)
			return
		}
		subject, ok := a.PartnerSessions.TgIDFromRequest(r)
		idStr, fp, found := strings.Cut(subject, "-")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if !ok || !found || err != nil {
			http.Redirect(w, r, "/partner/login", http.StatusSeeOther)
			return
		}
		user, err := a.Store.GetUserByID(id)
		if err != nil {
			a.serverError(w, err)
			return
		}
		hash := ""
		if user != nil {
			if hash, err = a.Store.AgentPasswordHash(user.ID); err != nil {
				a.serverError(w, err)
				return
			}
		}
		if user == nil || hash == "" || passwordFingerprint(hash) != fp {
			a.PartnerSessions.ClearCookie(w)
			http.Redirect(w, r, "/partner/login", http.StatusSeeOther)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), partnerCtxKey{}, user)))
	}
}

func (a *App) handlePartnerLoginShow(w http.ResponseWriter, r *http.Request) {
	a.render(w, "panel/partner_login.html", AdminPage{Title: "Кабинет партнёра"})
}

func (a *App) handlePartnerLoginSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	code := strings.ToUpper(strings.TrimSpace(r.FormValue("code")))
	password := r.FormValue("password")
	fail := func() {
		time.Sleep(time.Second) // притормаживаем перебор
		a.render(w, "panel/partner_login.html", AdminPage{Title: "Кабинет партнёра",
			Error: "Неверный код или пароль. Пароль задаётся в профиле приложения, в блоке «Партнёрская программа»."})
	}
	if code == "" || password == "" || a.PartnerSessions == nil {
		fail()
		return
	}
	user, err := a.Store.GetUserByReferralOrAgentCode(code)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if user == nil {
		fail()
		return
	}
	hash, err := a.Store.AgentPasswordHash(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		fail()
		return
	}
	a.PartnerSessions.SetCookie(w, strconv.FormatInt(user.ID, 10)+"-"+passwordFingerprint(hash))
	http.Redirect(w, r, "/partner", http.StatusSeeOther)
}

func (a *App) handlePartnerLogout(w http.ResponseWriter, r *http.Request) {
	if a.PartnerSessions != nil {
		a.PartnerSessions.ClearCookie(w)
	}
	http.Redirect(w, r, "/partner/login", http.StatusSeeOther)
}

type PartnerDashboardData struct {
	Name            string
	Code            string
	Balance         int
	PayingReferrals int
	Referred        []store.ReferredRow
	History         []store.WalletTx
	RatePlus        referrals.Rate
	RatePremium     referrals.Rate
	BoostThreshold  int
}

func (a *App) handlePartnerDashboard(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(partnerCtxKey{}).(*models.User)
	data := PartnerDashboardData{Name: user.Name, Code: user.ReferralCode.String,
		RatePlus: referrals.Rates["plus369"], RatePremium: referrals.Rates["premium888"], BoostThreshold: referrals.BoostThreshold}
	var err error
	if data.Balance, err = a.Store.WalletBalance(user.ID); err == nil {
		if data.PayingReferrals, err = a.Store.PayingReferralsCount(user.ID); err == nil {
			if data.Referred, err = a.Store.PartnerReferred(user.ID); err == nil {
				data.History, err = a.Store.WalletHistory(user.ID)
			}
		}
	}
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, "panel/partner_dashboard.html", AdminPage{Title: "Кабинет партнёра", Content: data})
}

var partnerPasswordMessages = map[string]string{
	"short":    "Пароль должен быть не короче 8 символов",
	"mismatch": "Пароли не совпадают",
	"long":     "Пароль слишком длинный (максимум 72 байта)",
	"ok":       "Пароль сохранён — входи в кабинет партнёра по реферальному коду и паролю",
}

// handlePartnerPasswordSet — пользователь в Mini App задаёт/меняет пароль
// кабинета партнёра в браузере.
func (a *App) handlePartnerPasswordSet(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	r.ParseForm()
	password := r.FormValue("password")
	if len([]rune(password)) < minPartnerPasswordLen {
		http.Redirect(w, r, "/wallet?partner_pw=short#partner", http.StatusSeeOther)
		return
	}
	if len(password) > 72 { // предел bcrypt
		http.Redirect(w, r, "/wallet?partner_pw=long#partner", http.StatusSeeOther)
		return
	}
	if password != r.FormValue("password2") {
		http.Redirect(w, r, "/wallet?partner_pw=mismatch#partner", http.StatusSeeOther)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if err := a.Store.SetAgentPasswordHash(user.ID, string(hash)); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/wallet?partner_pw=ok#partner", http.StatusSeeOther)
}
