// Package reports — чистые вычисления для еженедельного/ежемесячного
// отчёта (даты, динамика настроения). Порт backend/lib/reports.js.
// Сами данные достаёт handler через store — этот пакет ничего не знает о БД.
package reports

import (
	"math"
	"time"

	"version20/internal/leveling"
)

const mskLayout = "2006-01-02"

func TodayMoscow() string {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		loc = time.FixedZone("MSK", 3*60*60)
	}
	return time.Now().In(loc).Format(mskLayout)
}

func FromNDaysAgo(n int) string {
	t, _ := time.Parse(mskLayout, TodayMoscow())
	return t.AddDate(0, 0, -n).Format(mskLayout)
}

type MoodPoint struct {
	Week    int
	AvgMood float64
}

// MoodEntry — минимум, нужный для расчёта динамики настроения по неделям.
type MoodEntry struct {
	EntryDate string
	Emoji     int
}

func BuildMoodTrend(from string, entries []MoodEntry) []MoodPoint {
	fromDate, _ := time.Parse(mskLayout, from)
	buckets := make([][]int, 5)

	for _, e := range entries {
		d, err := time.Parse(mskLayout, e.EntryDate)
		if err != nil {
			continue
		}
		weekIdx := int(d.Sub(fromDate).Hours() / 24 / 7)
		if weekIdx < 0 {
			weekIdx = 0
		}
		if weekIdx > 4 {
			weekIdx = 4
		}
		buckets[weekIdx] = append(buckets[weekIdx], e.Emoji)
	}

	var trend []MoodPoint
	for i, vals := range buckets {
		if len(vals) == 0 {
			continue
		}
		sum := 0
		for _, v := range vals {
			sum += v
		}
		avg := math.Round(float64(sum)/float64(len(vals))*10) / 10
		trend = append(trend, MoodPoint{Week: i + 1, AvgMood: avg})
	}
	return trend
}

const (
	WeeklyReportXP  = leveling.WeeklyReportXP
	MonthlyReportXP = leveling.MonthlyReportXP
)
