package scheduler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"

	"version20/internal/models"
)

func loadBank(t *testing.T, gender string) []models.Task {
	t.Helper()
	b, err := os.ReadFile("../../cmd/server/data/tasks_" + gender + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var tasks []models.Task
	if err := json.Unmarshal(b, &tasks); err != nil {
		t.Fatal(err)
	}
	return tasks
}

// Эталон сгенерирован исходным backend/lib/scheduler.js (коммит 14bc237,
// до переписывания на Go): тот же банк заданий и те же ответы анкеты должны
// давать те же веса и ровно тот же порядок 365 заданий.
func TestParityWithNodeVersion(t *testing.T) {
	b, err := os.ReadFile("testdata/node_parity.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Profile        string             `json:"profile"`
		Gender         string             `json:"gender"`
		Answers        map[string]string  `json:"answers"`
		Weights        map[string]float64 `json:"weights"`
		ScheduleSha256 string             `json:"scheduleSha256"`
		First20        []string           `json:"first20"`
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("пустой эталон")
	}

	banks := map[string][]models.Task{"male": loadBank(t, "male"), "female": loadBank(t, "female")}

	for _, c := range cases {
		t.Run(c.Profile+"/"+c.Gender, func(t *testing.T) {
			answers := map[int]string{}
			for k, v := range c.Answers {
				id, _ := strconv.Atoi(k)
				answers[id] = v
			}

			weights := ComputeCategoryWeights(answers)
			for cat, want := range c.Weights {
				if math.Abs(weights[cat]-want) > 1e-9 {
					t.Errorf("вес %q: got %v, want %v (Node)", cat, weights[cat], want)
				}
			}

			schedule := BuildSchedule(banks[c.Gender], weights)
			ids := make([]string, len(schedule))
			for i, task := range schedule {
				ids[i] = task.ID
			}
			for i, want := range c.First20 {
				if ids[i] != want {
					t.Fatalf("день %d: got %s, want %s (Node)", i, ids[i], want)
				}
			}
			sum := sha256.Sum256([]byte(strings.Join(ids, "\n")))
			if got := hex.EncodeToString(sum[:]); got != c.ScheduleSha256 {
				t.Errorf("порядок 365 заданий расходится с Node-версией")
			}
		})
	}
}

func TestComputeCategoryWeightsBounds(t *testing.T) {
	best, worst := map[int]string{}, map[int]string{}
	for _, q := range models.Questions {
		if !q.Diagnostic {
			continue
		}
		switch q.Type {
		case "scale":
			best[q.ID], worst[q.ID] = "10", "1"
		case "single":
			best[q.ID], worst[q.ID] = q.Options[0].Value, q.Options[len(q.Options)-1].Value
		}
	}

	for _, cat := range models.Categories {
		if w := ComputeCategoryWeights(best)[cat]; w != 1 {
			t.Errorf("всё отлично, %s: weight=%v, want 1", cat, w)
		}
		if w := ComputeCategoryWeights(worst)[cat]; w != 5 {
			t.Errorf("всё плохо, %s: weight=%v, want 5", cat, w)
		}
		if w := ComputeCategoryWeights(nil)[cat]; w != 3 {
			t.Errorf("без ответов, %s: weight=%v, want 3", cat, w)
		}
	}
}

func TestComputeCategoryWeightsClampsScale(t *testing.T) {
	// id 1/2 — "Внешность": scale и single.
	w := ComputeCategoryWeights(map[int]string{1: "100"})["Внешность"]
	if w != 1 {
		t.Errorf("scale=100 должен обрезаться до 10 (weight 1), got %v", w)
	}
	w = ComputeCategoryWeights(map[int]string{1: "-5"})["Внешность"]
	if w != 5 {
		t.Errorf("scale=-5 должен обрезаться до 1 (weight 5), got %v", w)
	}
	w = ComputeCategoryWeights(map[int]string{1: "abc", 2: "нет такого варианта"})["Внешность"]
	if w != 3 {
		t.Errorf("некорректные ответы игнорируются (weight 3), got %v", w)
	}
}

func TestBuildScheduleUsesEveryTaskOnce(t *testing.T) {
	for _, gender := range []string{"male", "female"} {
		bank := loadBank(t, gender)
		weights := map[string]float64{"Деньги": 5, "Тело": 1}
		schedule := BuildSchedule(bank, weights)

		if len(schedule) != len(bank) {
			t.Fatalf("%s: len=%d, want %d", gender, len(schedule), len(bank))
		}
		seen := map[string]bool{}
		for _, task := range schedule {
			if seen[task.ID] {
				t.Fatalf("%s: задание %s встречается дважды", gender, task.ID)
			}
			seen[task.ID] = true
		}
	}
}

func TestBuildScheduleFavoursHeavyCategories(t *testing.T) {
	bank := loadBank(t, "male")
	weights := map[string]float64{}
	for _, c := range models.Categories {
		weights[c] = 1
	}
	weights["Деньги"] = 5

	schedule := BuildSchedule(bank, weights)
	if schedule[0].Category != "Деньги" {
		t.Errorf("день 1: %s, ожидалось самое слабое направление «Деньги»", schedule[0].Category)
	}
	counts := map[string]int{}
	for _, task := range schedule[:60] {
		counts[task.Category]++
	}
	for _, c := range models.Categories {
		if c != "Деньги" && counts[c] >= counts["Деньги"] {
			t.Errorf("за первые 60 дней «%s» (%d) не реже «Деньги» (%d)", c, counts[c], counts["Деньги"])
		}
	}
}

func TestBuildScheduleIsDeterministic(t *testing.T) {
	bank := loadBank(t, "female")
	weights := ComputeCategoryWeights(map[int]string{1: "3", 7: "2", 13: "9"})
	a, b := BuildSchedule(bank, weights), BuildSchedule(bank, weights)
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Fatalf("день %d отличается между запусками", i)
		}
	}
}
