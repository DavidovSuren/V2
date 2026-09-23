// Package promo — промокоды, вводимые в профиле. Сейчас единственный
// эффект — мгновенно выдать 100 уровень (для демо/маркетинга), но
// структура рассчитана на добавление новых кодов без переписывания хендлера.
package promo

type Effect string

const EffectLevel100 Effect = "level100"

var codes = map[string]Effect{
	"100LVL": EffectLevel100,
}

// aliases — другие написания того же промокода. Все они сохраняются под
// каноническим кодом, так что применить его можно только один раз.
var aliases = map[string]string{
	"LVL100": "100LVL",
}

// Lookup нормализует код (регистр/пробелы) и возвращает его эффект.
func Lookup(rawCode string) (Effect, bool) {
	effect, ok := codes[CanonicalCode(rawCode)]
	return effect, ok
}

// CanonicalCode — нормализованный код, используется как ключ при
// сохранении в promo_redemptions (чтобы "100lvl" и "100LVL" считались
// одним и тем же промокодом).
func CanonicalCode(rawCode string) string {
	code := normalize(rawCode)
	if canon, ok := aliases[code]; ok {
		return canon
	}
	return code
}

func normalize(s string) string {
	out := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			continue
		}
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out = append(out, c)
	}
	return string(out)
}
