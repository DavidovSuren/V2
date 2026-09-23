package reports

import (
	"reflect"
	"testing"
	"time"
)

func TestTodayMoscowFormat(t *testing.T) {
	if _, err := time.Parse("2006-01-02", TodayMoscow()); err != nil {
		t.Fatalf("TodayMoscow() = %q: %v", TodayMoscow(), err)
	}
}

func TestFromNDaysAgo(t *testing.T) {
	today, _ := time.Parse("2006-01-02", TodayMoscow())
	if got := FromNDaysAgo(0); got != TodayMoscow() {
		t.Errorf("FromNDaysAgo(0) = %s", got)
	}
	if got, want := FromNDaysAgo(6), today.AddDate(0, 0, -6).Format("2006-01-02"); got != want {
		t.Errorf("FromNDaysAgo(6) = %s, want %s", got, want)
	}
}

// Ожидания посчитаны по той же формуле, что в backend/lib/reports.js
// (monthlyReportFor): недели с from, 5-я неделя вбирает хвост, среднее с
// округлением до 0.1, пустые недели не выводятся.
func TestBuildMoodTrend(t *testing.T) {
	from := "2026-08-01"
	entries := []MoodEntry{
		{"2026-08-01", 1}, {"2026-08-02", 2}, {"2026-08-07", 4}, // неделя 1: 7/3 = 2.33 -> 2.3
		{"2026-08-15", 5},                    // неделя 3
		{"2026-08-29", 3}, {"2026-08-30", 4}, // неделя 5 (дни 28-29)
		{"bad-date", 5},
	}
	got := BuildMoodTrend(from, entries)
	want := []MoodPoint{{1, 2.3}, {3, 5}, {5, 3.5}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBuildMoodTrendEmpty(t *testing.T) {
	if got := BuildMoodTrend("2026-08-01", nil); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

func TestBuildMoodTrendClampsOutOfRange(t *testing.T) {
	got := BuildMoodTrend("2026-08-01", []MoodEntry{{"2026-07-30", 2}, {"2026-09-30", 4}})
	want := []MoodPoint{{1, 2}, {5, 4}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
