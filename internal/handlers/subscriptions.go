package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"version20/internal/leveling"
	"version20/internal/models"
	"version20/internal/referrals"
)

type TierQuote struct {
	Tier        string
	BasePrice   int
	DiscountPct int
	PricePaid   int
}

type ProfileData struct {
	Name              string
	AgeGroup          string
	Gender            string
	SubscriptionLabel string
	ProgressPct       float64
	Level             int
	XP                int
	StreakCurrent     int
	ReferralCode      string
	HasPremiumAgent   bool
	Tiers             []TierQuote
}

func (a *App) tierQuotes(user *models.User) []TierQuote {
	quotes := make([]TierQuote, 0, len(referrals.Prices))
	for _, tier := range []string{"plus369", "premium888"} {
		basePrice := referrals.Prices[tier]
		discount := 0
		if user.ReferredByUserID.Valid {
			if user.ReferredByCodeType.String == "premium_agent" {
				discount = 0
			} else {
				count, _ := a.Store.PriorPaidReferralsCount(user.ReferredByUserID.Int64)
				discount = referrals.DiscountPctForReferrer(count)
			}
		}
		quotes = append(quotes, TierQuote{
			Tier: tier, BasePrice: basePrice, DiscountPct: discount,
			PricePaid: referrals.PriceAfterDiscount(basePrice, discount),
		})
	}
	return quotes
}

func (a *App) handleProfile(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	progressPct, level := leveling.ProgressFromCompleted(user.CompletedCount)

	tierLabel := map[string]string{
		"free": "Бесплатно", "plus369": "Version 2.0 Plus", "premium888": "Version 2.0 Premium 💎",
	}[user.SubscriptionTier]
	if tierLabel == "" {
		tierLabel = "Бесплатно"
	}

	a.render(w, "profile.html", ProfileData{
		Name: user.Name, AgeGroup: user.AgeGroup, Gender: user.Gender.String,
		SubscriptionLabel: tierLabel, ProgressPct: progressPct, Level: level, XP: user.XP,
		StreakCurrent: user.StreakCurrent, ReferralCode: user.ReferralCode.String,
		HasPremiumAgent: user.HasPremiumAgentCode(),
		Tiers:           a.tierQuotes(user),
	})
}

func (a *App) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	user := userFromCtx(r)
	r.ParseForm()
	tier := r.FormValue("tier")
	basePrice, ok := referrals.Prices[tier]
	if !ok {
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	discountPct := 0
	var referrerCommissionUserID sql.NullInt64
	commissionAmount := 0

	if user.ReferredByUserID.Valid {
		if user.ReferredByCodeType.String == "premium_agent" {
			discountPct = 0
		} else {
			count, _ := a.Store.PriorPaidReferralsCount(user.ReferredByUserID.Int64)
			discountPct = referrals.DiscountPctForReferrer(count)
		}
	}
	pricePaid := referrals.PriceAfterDiscount(basePrice, discountPct)

	if user.ReferredByUserID.Valid && user.ReferredByCodeType.String == "premium_agent" {
		referrerCommissionUserID = user.ReferredByUserID
		commissionAmount = pricePaid * referrals.PremiumAgentCommissionPct / 100
	}

	now := time.Now().UTC()
	expires := now.AddDate(0, 1, 0).Format(time.RFC3339)

	tx, err := a.Store.DB.Begin()
	if err != nil {
		a.serverError(w, err)
		return
	}
	defer tx.Rollback()

	if err := a.Store.InsertSubscriptionPayment(tx, user.ID, tier, basePrice, discountPct, pricePaid,
		referrerCommissionUserID, commissionAmount, now.Format(time.RFC3339)); err != nil {
		a.serverError(w, err)
		return
	}
	if err := a.Store.UpdateSubscription(tx, user.ID, tier, expires); err != nil {
		a.serverError(w, err)
		return
	}
	if referrerCommissionUserID.Valid {
		if err := a.Store.InsertWalletTransaction(tx, referrerCommissionUserID.Int64, commissionAmount, user.ID,
			"Комиссия с оплаты "+tier, now.Format(time.RFC3339)); err != nil {
			a.serverError(w, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		a.serverError(w, err)
		return
	}

	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	a.Sessions.ClearCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
