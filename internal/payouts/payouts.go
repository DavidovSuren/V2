// Package payouts — правила вывода денег из кошелька партнёра.
//
// Вывод раз в месяц, только 15-го числа (по Москве), на карту банка РФ.
// Налог удерживается сразу: партнёр видит сумму к выводу, налог и сумму,
// которая придёт на карту. Выплату проводит владелец вручную из админки.
package payouts

import (
	"strings"
	"time"
)

// PayoutDay — день месяца, когда доступен вывод.
const PayoutDay = 15

// MinAmount — минимальная сумма вывода, ₽.
const MinAmount = 500

// DefaultTaxPct — НДФЛ, который удерживается с выплаты физлицу (ставка 13%
// для доходов до 2,4 млн ₽ в год). Меняется переменной TAX_WITHHOLD_PCT.
const DefaultTaxPct = 13

type Breakdown struct {
	Gross int // списывается с баланса
	Tax   int // удерживается и перечисляется в бюджет
	Net   int // приходит на карту
}

func Split(gross, taxPct int) Breakdown {
	tax := int(float64(gross)*float64(taxPct)/100 + 0.5)
	return Breakdown{Gross: gross, Tax: tax, Net: gross - tax}
}

// IsPayoutDay — сегодня 15-е число по Москве.
func IsPayoutDay(now time.Time, loc *time.Location) bool {
	return now.In(loc).Day() == PayoutDay
}

// NextPayoutDate — ближайшее 15-е (сегодня, если сегодня 15-е).
func NextPayoutDate(now time.Time, loc *time.Location) time.Time {
	n := now.In(loc)
	d := time.Date(n.Year(), n.Month(), PayoutDay, 0, 0, 0, 0, loc)
	if n.Day() > PayoutDay {
		d = d.AddDate(0, 1, 0)
	}
	return d
}

// MonthKey — один вывод на месяц: ключ вида "2026-10".
func MonthKey(now time.Time, loc *time.Location) string {
	return now.In(loc).Format("2006-01")
}

var monthsGen = []string{"", "января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря"}

// HumanDate — "15 октября".
func HumanDate(t time.Time) string {
	return itoa(t.Day()) + " " + monthsGen[t.Month()]
}

// NormalizeCard оставляет только цифры.
func NormalizeCard(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ValidCard — 16–19 цифр и корректная контрольная сумма (алгоритм Луна).
func ValidCard(digits string) bool {
	if len(digits) < 16 || len(digits) > 19 {
		return false
	}
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

// MaskCard — "2200 •••• •••• 1234".
func MaskCard(digits string) string {
	if len(digits) < 8 {
		return "••••"
	}
	return digits[:4] + " •••• •••• " + digits[len(digits)-4:]
}

// ValidINN — ИНН физлица: 12 цифр с двумя контрольными.
func ValidINN(inn string) bool {
	if len(inn) != 12 {
		return false
	}
	d := make([]int, 12)
	for i, r := range inn {
		if r < '0' || r > '9' {
			return false
		}
		d[i] = int(r - '0')
	}
	check := func(n int, w []int) int {
		s := 0
		for i := 0; i < n; i++ {
			s += d[i] * w[i]
		}
		return s % 11 % 10
	}
	w11 := []int{7, 2, 4, 10, 3, 5, 9, 4, 6, 8}
	w12 := []int{3, 7, 2, 4, 10, 3, 5, 9, 4, 6, 8}
	return check(10, w11) == d[10] && check(11, w12) == d[11]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
