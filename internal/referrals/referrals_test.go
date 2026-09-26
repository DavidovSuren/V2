package referrals

import "testing"

func TestCommissionPct(t *testing.T) {
	cases := []struct {
		tier  string
		prior int
		want  int
	}{
		{"plus369", 0, 5}, {"plus369", 9, 5}, {"plus369", 10, 10}, {"plus369", 50, 10},
		{"premium888", 0, 20}, {"premium888", 9, 20}, {"premium888", 10, 50}, {"premium888", 11, 50},
		{"free", 0, 0}, {"free", 20, 0}, {"", 5, 0}, {"gold", 0, 0},
	}
	for _, c := range cases {
		if got := CommissionPct(c.tier, c.prior); got != c.want {
			t.Errorf("CommissionPct(%q, %d) = %d, want %d", c.tier, c.prior, got, c.want)
		}
	}
}

// Примеры из плана: 369×5% = 18, 369×10% = 37, 888×20% = 178, 888×50% = 444, 369×50% = 185.
func TestCommissionAmount(t *testing.T) {
	cases := []struct{ price, pct, want int }{
		{369, 5, 18}, {369, 10, 37}, {369, 20, 74}, {369, 50, 185},
		{888, 5, 44}, {888, 10, 89}, {888, 20, 178}, {888, 50, 444},
		{369, 0, 0}, {0, 50, 0},
	}
	for _, c := range cases {
		if got := CommissionAmount(c.price, c.pct); got != c.want {
			t.Errorf("CommissionAmount(%d, %d) = %d, want %d", c.price, c.pct, got, c.want)
		}
	}
}

func TestTiers(t *testing.T) {
	if Prices["plus369"] != 369 || Prices["premium888"] != 888 || len(Prices) != 2 {
		t.Fatalf("тарифы: %v", Prices)
	}
	if len(TierOrder) != len(Prices) {
		t.Fatalf("TierOrder: %v", TierOrder)
	}
	for _, tier := range TierOrder {
		if _, ok := Prices[tier]; !ok || TierNames[tier] == "" {
			t.Errorf("тариф %q описан не полностью", tier)
		}
		if r := Rates[tier]; r.Base <= 0 || r.Boosted <= r.Base {
			t.Errorf("проценты %q: %+v", tier, r)
		}
	}
	if BoostThreshold != 10 {
		t.Error("порог повышенного процента — 10 оплативших")
	}
}
