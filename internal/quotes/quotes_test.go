package quotes

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"version20/internal/models"
)

func TestCatalog(t *testing.T) {
	total := len(general)
	if len(general) != 12 {
		t.Errorf("общих цитат %d, want 12", len(general))
	}
	for _, cat := range models.Categories {
		qs := byCategory[cat]
		if len(qs) != 6 {
			t.Errorf("%s: %d цитат, want 6", cat, len(qs))
		}
		total += len(qs)
	}
	if total != 60 || len(byCategory) != 8 {
		t.Fatalf("всего %d цитат в %d направлениях", total, len(byCategory))
	}

	seen := map[string]bool{}
	check := func(q string) {
		if n := utf8.RuneCountInString(q); n > 120 {
			t.Errorf("длиннее 120 символов (%d): %q", n, q)
		}
		if seen[q] {
			t.Errorf("повтор: %q", q)
		}
		seen[q] = true
		// Без рода: ни «сделал(а)», ни прошедшего времени мужского/женского рода от первого лица.
		if strings.Contains(q, "(а)") || strings.Contains(q, "!") {
			t.Errorf("лишнее в цитате: %q", q)
		}
	}
	for _, q := range general {
		check(q)
	}
	for _, qs := range byCategory {
		for _, q := range qs {
			check(q)
		}
	}
}

func TestSameDaySameQuote(t *testing.T) {
	a := ForUser("42", "2026-10-01", "Стиль")
	for i := 0; i < 10; i++ {
		if ForUser("42", "2026-10-01", "Стиль") != a {
			t.Fatal("цитата меняется при обновлении страницы")
		}
	}
	found := false
	for _, q := range byCategory["Стиль"] {
		found = found || q == a
	}
	if !found {
		t.Errorf("цитата не из направления задания: %q", a)
	}
	for _, cat := range []string{"", "Нет такого"} {
		q := ForUser("42", "2026-10-01", cat)
		ok := false
		for _, g := range general {
			ok = ok || g == q
		}
		if !ok {
			t.Errorf("без направления %q — не общая цитата: %q", cat, q)
		}
	}
}

// Направление задания меняется день ото дня, как в плане.
func TestDifferentDays(t *testing.T) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	distinct := map[string]bool{}
	for i := 0; i < 30; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		distinct[ForUser("12345", date, models.Categories[i%len(models.Categories)])] = true
	}
	if len(distinct) < 20 {
		t.Errorf("за 30 дней %d разных цитат, want ≥ 20", len(distinct))
	}
}

func TestDifferentUsersSameDay(t *testing.T) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	differ := 0
	for i := 0; i < 60; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		if ForUser("1001", date, "Деньги") != ForUser("2002", date, "Деньги") {
			differ++
		}
	}
	if differ <= 30 {
		t.Errorf("у двух пользователей разные цитаты только в %d из 60 дней", differ)
	}
}
