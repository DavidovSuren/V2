// Package referrals — тарифы, скидки и комиссия премиум-агентов.
// Прямой порт backend/lib/referrals.js.
package referrals

var Prices = map[string]int{
	"plus369":    369,
	"premium888": 888,
}

const PremiumAgentCommissionPct = 50

// Скидка для НОВОГО приглашённого зависит от того, сколько людей его
// пригласивший уже успешно привёл к оплате (paidReferralsCount — на момент
// оплаты этого нового человека, ДО текущей оплаты).
// Примеры из ТЗ: 1–3 оплативших приглашённых -> 20%; >3 -> 35%; пригласил
// 10 -> 11-й уже получает 50% (т.е. порог считается по факту "10 уже есть").
func DiscountPctForReferrer(paidReferralsCount int) int {
	if paidReferralsCount >= 10 {
		return 50
	}
	if paidReferralsCount >= 4 {
		return 35
	}
	return 20
}

func PriceAfterDiscount(basePrice, discountPct int) int {
	return int(float64(basePrice)*(100-float64(discountPct))/100 + 0.5)
}
