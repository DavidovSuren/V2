package handlers

import (
	"net/http"
	"time"

	"version20/internal/models"
	"version20/internal/payouts"
	"version20/internal/referrals"
	"version20/internal/subscription"
)

// TermsVersion — редакция пользовательского соглашения. Если поменять, все,
// кто принимал старую, увидят экран «Мы обновили соглашение».
const TermsVersion = "2026-10-01"

// SupportContact — куда писать (соглашение, профиль, ошибки оплаты).
const SupportContact = "@version20_help"

func needsTerms(u *models.User) bool {
	return u.TermsVersion.String != TermsVersion
}

type TermsData struct {
	Version        string
	TrialDays      int
	Tiers          []TierInfo
	BoostThreshold int
	PayoutDay      int
	MinAmount      int
	TaxPct         int
	Support        string
}

func (a *App) termsData() TermsData {
	return TermsData{
		Version: TermsVersion, TrialDays: subscription.TrialDays,
		Tiers: tierInfos(), BoostThreshold: referrals.BoostThreshold,
		PayoutDay: payouts.PayoutDay, MinAmount: payouts.MinAmount, TaxPct: a.taxPct(),
		Support: SupportContact,
	}
}

// handleTerms — пользовательское соглашение; открыто без входа.
func (a *App) handleTerms(w http.ResponseWriter, r *http.Request) {
	a.render(w, "terms.html", a.termsData())
}

func (a *App) renderTermsUpdate(w http.ResponseWriter) {
	a.render(w, "terms_update.html", a.termsData())
}

// handleTermsAccept — принять текущую редакцию (экран обновления соглашения).
func (a *App) handleTermsAccept(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if r.FormValue("terms") == "" {
		a.renderTermsUpdate(w)
		return
	}
	if err := a.Store.AcceptTerms(userFromCtx(r).ID, TermsVersion, a.now().UTC().Format(time.RFC3339)); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
