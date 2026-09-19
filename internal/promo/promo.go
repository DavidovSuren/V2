// Package promo — промокоды, вводимые в профиле. Сейчас единственный
// эффект — мгновенно выдать 100 уровень (для демо/маркетинга), но
// структура рассчитана на добавление новых кодов без переписывания хендлера.
package promo

type Effect string

const EffectLevel100 Effect = "level100"

var codes = map[string]Effect{
	"100LVL": EffectLevel100,
}

// Lookup нормализует код (регистр/пробелы) и возвращает его эффект.
func Lookup(rawCode string) (Effect, bool) {
	code := normalize(rawCode)
	effect, ok := codes[code]
	return effect, ok
}

// CanonicalCode — нормализованный код, используется как ключ при
// сохранении в promo_redemptions (чтобы "100lvl" и "100LVL" считались
// одним и тем же промокодом).
func CanonicalCode(rawCode string) string {
	return normalize(rawCode)
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
