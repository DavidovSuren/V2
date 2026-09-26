// Package referrals — тарифы и партнёрская программа: каждый подписчик
// получает процент с оплат приглашённых им людей. Скидок для приглашённых
// нет — они всегда платят полную цену.
package referrals

var Prices = map[string]int{
	"plus369":    369,
	"premium888": 888,
}

// TierOrder — порядок тарифов в интерфейсе (от младшего к старшему).
var TierOrder = []string{"plus369", "premium888"}

var TierNames = map[string]string{
	"plus369":    "Plus",
	"premium888": "Premium",
}

// BoostThreshold — сколько разных приглашённых должны оплатить подписку,
// чтобы процент со следующих оплат вырос (с 11-го оплатившего).
const BoostThreshold = 10

// Rate — процент партнёра: базовый и повышенный (после BoostThreshold оплативших).
type Rate struct {
	Base    int
	Boosted int
}

// Rates — процент по тарифу пригласившего на момент оплаты приглашённого.
// Без активной подписки партнёрский доход не начисляется.
var Rates = map[string]Rate{
	"plus369":    {Base: 5, Boosted: 10},
	"premium888": {Base: 20, Boosted: 50},
}

// CommissionPct — процент для пригласившего с тарифом referrerTier ("" или
// "free" — нет подписки), у которого до этой оплаты уже оплачивали подписку
// priorPaying разных приглашённых (платящий сейчас не считается).
func CommissionPct(referrerTier string, priorPaying int) int {
	rate, ok := Rates[referrerTier]
	if !ok {
		return 0
	}
	if priorPaying >= BoostThreshold {
		return rate.Boosted
	}
	return rate.Base
}

// CommissionAmount — сумма в рублях, округление по математике (369 × 50% = 185).
func CommissionAmount(pricePaid, pct int) int {
	return int(float64(pricePaid)*float64(pct)/100 + 0.5)
}
