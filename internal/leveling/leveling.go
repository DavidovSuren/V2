// Package leveling считает уровень/прогресс/XP — прямой порт backend/lib/leveling.js.
package leveling

import "math"

const TotalTasks = 365

var LevelMilestones = []int{25, 50, 75, 100}
var StreakMilestones = []int{7, 30, 180, 365}

const (
	WeeklyReportXP  = 25
	MonthlyReportXP = 60
	QuizCompleteXP  = 50
)

// Прогресс и уровень считаются только от количества реально выполненных
// заданий — 365/365 = 100% = уровень 100 (и алмаз). Один уровень ≈ 3.65 задания.
func ProgressFromCompleted(completed int) (progressPct float64, level int) {
	progressPct = math.Min(100, float64(completed)/float64(TotalTasks)*100)
	level = int(math.Min(100, math.Floor(progressPct)))
	return
}

// XP — отдельная "игровая" валюта для мотивации, на level/progress не влияет.
// Базовое начисление за задание + бонус за каждую полную неделю стрика.
func XPForTaskCompletion(streakCurrentAfter int) int {
	base := 10
	streakBonus := streakCurrentAfter / 7 * 5
	if streakBonus > 20 {
		streakBonus = 20
	}
	return base + streakBonus
}

func CategoryProgressPct(doneInCategory, totalInCategoryBank int) int {
	if totalInCategoryBank == 0 {
		return 0
	}
	return int(math.Round(float64(doneInCategory) / float64(totalInCategoryBank) * 100))
}
