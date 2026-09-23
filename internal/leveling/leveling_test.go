package leveling

import (
	"math"
	"testing"
)

func TestProgressFromCompleted(t *testing.T) {
	cases := []struct {
		completed int
		level     int
	}{
		{0, 0}, {1, 0}, {3, 0}, {4, 1}, {37, 10}, {92, 25}, {182, 49}, {183, 50},
		{364, 99}, {365, 100}, {400, 100},
	}
	for _, c := range cases {
		pct, level := ProgressFromCompleted(c.completed)
		if level != c.level {
			t.Errorf("completed=%d: level=%d, want %d", c.completed, level, c.level)
		}
		wantPct := math.Min(100, float64(c.completed)/365*100)
		if math.Abs(pct-wantPct) > 1e-9 {
			t.Errorf("completed=%d: pct=%v, want %v", c.completed, pct, wantPct)
		}
	}
}

// Уровень 100 (алмаз) — только при 365/365, не раньше.
func TestLevel100OnlyAtFullYear(t *testing.T) {
	if _, l := ProgressFromCompleted(364); l == 100 {
		t.Fatal("364 задания не должны давать уровень 100")
	}
}

func TestXPForTaskCompletion(t *testing.T) {
	cases := map[int]int{0: 10, 1: 10, 6: 10, 7: 15, 13: 15, 14: 20, 21: 25, 28: 30, 35: 30, 365: 30}
	for streak, want := range cases {
		if got := XPForTaskCompletion(streak); got != want {
			t.Errorf("streak=%d: xp=%d, want %d", streak, got, want)
		}
	}
}

func TestCategoryProgressPct(t *testing.T) {
	cases := []struct{ done, total, want int }{
		{0, 0, 0}, {0, 45, 0}, {1, 45, 2}, {23, 46, 50}, {45, 45, 100}, {1, 3, 33}, {2, 3, 67},
	}
	for _, c := range cases {
		if got := CategoryProgressPct(c.done, c.total); got != c.want {
			t.Errorf("%d/%d: %d, want %d", c.done, c.total, got, c.want)
		}
	}
}

// 50 (анкета) + 6×10 + 7×15 + 7×20 + 7×25 + 338×30.
func TestFullYearXP(t *testing.T) {
	if got := FullYearXP(); got != 10670 {
		t.Fatalf("FullYearXP() = %d, want 10670", got)
	}
}

func TestConstantsMatchNodeVersion(t *testing.T) {
	if TotalTasks != 365 || WeeklyReportXP != 25 || MonthlyReportXP != 60 || QuizCompleteXP != 50 {
		t.Fatal("константы XP/заданий расходятся с backend/lib/leveling.js")
	}
}
