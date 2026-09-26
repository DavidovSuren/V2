package handlers

import (
	"html/template"
	"strconv"
)

// TemplateFuncs — функции шаблонов; одни и те же в cmd/server и в тестах.
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"seq": func(from, to int) []int {
			out := make([]int, 0, to-from+1)
			for i := from; i <= to; i++ {
				out = append(out, i)
			}
			return out
		},
		"rub":    Rub,
		"digits": Digits,
	}
}

// Digits — число с неразрывными пробелами между разрядами: 10670 → "10 670".
func Digits(n int) string {
	s := strconv.Itoa(n)
	neg := false
	if n < 0 {
		neg, s = true, s[1:]
	}
	out := make([]byte, 0, len(s)+len(s)/3*3)
	for i := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, " "...)
		}
		out = append(out, s[i])
	}
	if neg {
		return "−" + string(out)
	}
	return string(out)
}

// Rub — "5 772 ₽".
func Rub(n int) string {
	return Digits(n) + " ₽"
}
