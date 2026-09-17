// Package scheduler строит персональный план на 365 дней — прямой порт
// backend/lib/scheduler.js.
package scheduler

import (
	"strconv"

	"version20/internal/models"
)

// ComputeCategoryWeights переводит ответы анкеты в вес приоритета (1..5) на
// каждую из 8 категорий. Чем слабее человек оценивает себя в направлении,
// тем выше приоритет — такие направления чаще и раньше попадают в план.
// answers — ответы по question id (без вопроса 0 про пол), значения — либо
// число (scale, как строка или float64), либо строка варианта (single).
func ComputeCategoryWeights(answers map[int]string) map[string]float64 {
	scores := map[string][]float64{}
	for _, cat := range models.Categories {
		scores[cat] = nil
	}

	for _, q := range models.Questions {
		if !q.Diagnostic || q.Category == "" {
			continue
		}
		answer, ok := answers[q.ID]
		if !ok || answer == "" {
			continue
		}

		var score float64
		hasScore := false

		switch q.Type {
		case "scale":
			if n, err := strconv.ParseFloat(answer, 64); err == nil {
				if n < 1 {
					n = 1
				}
				if n > 10 {
					n = 10
				}
				score = n
				hasScore = true
			}
		case "single":
			idx := -1
			for i, opt := range q.Options {
				if opt.Value == answer {
					idx = i
					break
				}
			}
			if idx >= 0 && len(q.Options) > 1 {
				score = 10 - (float64(idx)/float64(len(q.Options)-1))*9
				hasScore = true
			}
		}

		if hasScore {
			scores[q.Category] = append(scores[q.Category], score)
		}
	}

	weights := map[string]float64{}
	for _, cat := range models.Categories {
		vals := scores[cat]
		if len(vals) == 0 {
			weights[cat] = 3 // нейтральный приоритет без диагностических ответов
			continue
		}
		sum := 0.0
		for _, v := range vals {
			sum += v
		}
		avg := sum / float64(len(vals))
		w := 1 + ((10 - avg) / 9 * 4)
		if w < 1 {
			w = 1
		}
		if w > 5 {
			w = 5
		}
		weights[cat] = w
	}
	return weights
}

// BuildSchedule — weighted round robin: категории с более высоким весом
// встречаются раньше и чаще, но КАЖДОЕ задание банка используется ровно
// один раз к 365-му дню (весь банк расходуется).
func BuildSchedule(bank []models.Task, weights map[string]float64) []models.Task {
	byCategory := map[string][]models.Task{}
	order := []string{}
	for _, t := range bank {
		if _, ok := byCategory[t.Category]; !ok {
			order = append(order, t.Category)
		}
		byCategory[t.Category] = append(byCategory[t.Category], t)
	}

	pointer := map[string]int{}
	credit := map[string]float64{}
	weight := map[string]float64{}
	for _, cat := range order {
		pointer[cat] = 0
		credit[cat] = 0
		w := weights[cat]
		if w == 0 {
			w = 3
		}
		weight[cat] = w
	}

	schedule := make([]models.Task, 0, len(bank))
	remaining := len(bank)

	for remaining > 0 {
		var active []string
		totalActiveWeight := 0.0
		for _, cat := range order {
			if pointer[cat] < len(byCategory[cat]) {
				active = append(active, cat)
				totalActiveWeight += weight[cat]
			}
		}
		for _, cat := range active {
			credit[cat] += weight[cat]
		}

		chosen := active[0]
		for _, cat := range active {
			if credit[cat] > credit[chosen] {
				chosen = cat
			}
		}

		schedule = append(schedule, byCategory[chosen][pointer[chosen]])
		pointer[chosen]++
		credit[chosen] -= totalActiveWeight
		remaining--
	}

	return schedule
}
