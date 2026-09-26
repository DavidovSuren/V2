package handlers

import (
	"net/http"

	"version20/internal/payouts"
	"version20/internal/referrals"
	"version20/internal/subscription"
)

// TermsVersion — редакция пользовательского соглашения; сохраняется у
// пользователя при регистрации (галочка на приветствии).
const TermsVersion = "2026-10-01"

// SupportContact — куда писать (соглашение, профиль, ошибки оплаты).
const SupportContact = "@version20_help"

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
