package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"version20/internal/models"
	"version20/internal/scheduler"
)

type QuizPageData struct {
	Index    int
	Total    int
	PctWidth int
	Question models.Question
	Answer   string
	IsFirst  bool
	IsLast   bool
}

func (a *App) handleQuizShow(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 0 || n >= len(models.Questions) {
		http.Redirect(w, r, "/quiz/0", http.StatusSeeOther)
		return
	}
	user := userFromCtx(r)
	q := models.Questions[n]

	answer, _ := a.Store.GetQuizAnswer(user.ID, q.ID)

	a.render(w, "quiz.html", QuizPageData{
		Index: n, Total: len(models.Questions),
		PctWidth: (n + 1) * 100 / len(models.Questions),
		Question: q, Answer: answer,
		IsFirst: n == 0, IsLast: n == len(models.Questions)-1,
	})
}

func (a *App) handleQuizSubmit(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 0 || n >= len(models.Questions) {
		http.Redirect(w, r, "/quiz/0", http.StatusSeeOther)
		return
	}
	user := userFromCtx(r)
	q := models.Questions[n]

	r.ParseForm()
	answer := strings.TrimSpace(r.FormValue("answer"))
	if answer == "" {
		http.Redirect(w, r, "/quiz/"+strconv.Itoa(n), http.StatusSeeOther)
		return
	}

	if err := a.Store.UpsertQuizAnswer(a.Store.DB, user.ID, q.ID, mustJSON(answer)); err != nil {
		a.serverError(w, err)
		return
	}

	if n < len(models.Questions)-1 {
		http.Redirect(w, r, "/quiz/"+strconv.Itoa(n+1), http.StatusSeeOther)
		return
	}

	a.finishQuiz(w, r, user)
}

func (a *App) finishQuiz(w http.ResponseWriter, r *http.Request, user *models.User) {
	answers := map[int]string{}
	for _, q := range models.Questions {
		val, err := a.Store.GetQuizAnswer(user.ID, q.ID)
		if err != nil || val == "" {
			http.Redirect(w, r, "/quiz/0", http.StatusSeeOther)
			return
		}
		answers[q.ID] = val
	}

	gender := answers[0]
	if gender != "male" && gender != "female" {
		http.Redirect(w, r, "/quiz/0", http.StatusSeeOther)
		return
	}

	weights := scheduler.ComputeCategoryWeights(answers)
	bank := a.TaskBankFor(gender)
	schedule := scheduler.BuildSchedule(bank, weights)

	tx, err := a.Store.DB.Begin()
	if err != nil {
		a.serverError(w, err)
		return
	}
	defer tx.Rollback()

	for _, cat := range models.Categories {
		if err := a.Store.UpsertCategoryWeight(tx, user.ID, cat, weights[cat]); err != nil {
			a.serverError(w, err)
			return
		}
	}
	for i, t := range schedule {
		if err := a.Store.InsertScheduleRow(tx, user.ID, i, t); err != nil {
			a.serverError(w, err)
			return
		}
	}
	const quizXP = 50
	if err := a.Store.FinishQuiz(tx, user.ID, gender, quizXP); err != nil {
		a.serverError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		a.serverError(w, err)
		return
	}

	// Топ-3 направления с наибольшим приоритетом — покажем на главном экране.
	type kv struct {
		Cat string
		W   float64
	}
	var ranked []kv
	for _, cat := range models.Categories {
		ranked = append(ranked, kv{cat, weights[cat]})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].W > ranked[j].W })
	focus := make([]string, 0, 3)
	for i := 0; i < 3 && i < len(ranked); i++ {
		focus = append(focus, ranked[i].Cat)
	}

	http.Redirect(w, r, "/?focus="+url.QueryEscape(strings.Join(focus, ",")), http.StatusSeeOther)
}

func mustJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
