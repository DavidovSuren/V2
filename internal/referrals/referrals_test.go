package referrals

import "testing"

func TestDiscountPctForReferrer(t *testing.T) {
	cases := map[int]int{0: 20, 1: 20, 3: 20, 4: 35, 9: 35, 10: 50, 11: 50, 100: 50}
	for paid, want := range cases {
		if got := DiscountPctForReferrer(paid); got != want {
			t.Errorf("paid=%d: %d%%, want %d%%", paid, got, want)
		}
	}
}

// Ожидаемые значения посчитаны Math.round из backend/lib/referrals.js.
func TestPriceAfterDiscount(t *testing.T) {
	cases := []struct{ base, pct, want int }{
		{369, 0, 369}, {369, 20, 295}, {369, 35, 240}, {369, 50, 185},
		{888, 0, 888}, {888, 20, 710}, {888, 35, 577}, {888, 50, 444},
	}
	for _, c := range cases {
		if got := PriceAfterDiscount(c.base, c.pct); got != c.want {
			t.Errorf("%d -%d%%: %d, want %d", c.base, c.pct, got, c.want)
		}
	}
}

// Node: Math.round(pricePaid * 50 / 100). Раньше Go отбрасывал дробную
// часть, и агент получал 184 ₽ вместо 185 ₽ с оплаты 369 ₽.
func TestCommissionAmount(t *testing.T) {
	cases := map[int]int{369: 185, 888: 444, 0: 0, 1: 1}
	for paid, want := range cases {
		if got := CommissionAmount(paid); got != want {
			t.Errorf("paid=%d: %d, want %d", paid, got, want)
		}
	}
}

func TestPrices(t *testing.T) {
	if Prices["plus369"] != 369 || Prices["premium888"] != 888 || len(Prices) != 2 {
		t.Fatalf("тарифы: %v", Prices)
	}
	if PremiumAgentCommissionPct != 50 {
		t.Fatal("комиссия агента должна быть 50%")
	}
}
