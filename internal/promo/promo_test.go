package promo

import "testing"

func TestLookup(t *testing.T) {
	for _, raw := range []string{"100LVL", "100lvl", " 100 Lvl ", "\t100LVL\n", "LVL100", "lvl100", "Lvl 100"} {
		effect, ok := Lookup(raw)
		if !ok || effect != EffectLevel100 {
			t.Errorf("Lookup(%q) = %v, %v", raw, effect, ok)
		}
		if CanonicalCode(raw) != "100LVL" {
			t.Errorf("CanonicalCode(%q) = %q", raw, CanonicalCode(raw))
		}
	}
	for _, raw := range []string{"", "100", "LVL", "100LVLX", "LVL1000"} {
		if _, ok := Lookup(raw); ok {
			t.Errorf("Lookup(%q) не должен находить промокод", raw)
		}
	}
}
