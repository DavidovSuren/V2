package main

import (
	"os"
	"path/filepath"
	"testing"

	"version20/internal/models"
)

func TestTaskBanks(t *testing.T) {
	for _, gender := range []string{"male", "female"} {
		t.Run(gender, func(t *testing.T) {
			bank := loadTasks(dataFS, "data/tasks_"+gender+".json")
			if len(bank) != 365 {
				t.Fatalf("заданий %d, want 365", len(bank))
			}

			known := map[string]bool{}
			for _, c := range models.Categories {
				known[c] = true
			}
			ids := map[string]bool{}
			perCat := map[string]int{}
			for _, task := range bank {
				if ids[task.ID] {
					t.Errorf("дубликат id %s", task.ID)
				}
				ids[task.ID] = true
				if !known[task.Category] {
					t.Errorf("%s: неизвестное направление %q", task.ID, task.Category)
				}
				if task.Text == "" || task.Why == "" {
					t.Errorf("%s: пустой текст или «почему»", task.ID)
				}
				if task.Gender != gender && task.Gender != "both" {
					t.Errorf("%s: gender=%q в банке %s", task.ID, task.Gender, gender)
				}
				perCat[task.Category]++
			}
			if len(perCat) != len(models.Categories) {
				t.Errorf("направлений в банке %d, want %d", len(perCat), len(models.Categories))
			}
		})
	}
}

// Все страницы должны разбираться вместе с layout/partials — иначе сервер
// упадёт на старте в loadTemplates.
func TestTemplatesParse(t *testing.T) {
	tmpl := loadTemplates(templatesFS)
	for _, page := range []string{
		"welcome.html", "quiz.html", "home.html", "skip_confirm.html", "diary.html", "done.html",
		"progress.html", "community.html", "profile.html",
		"report.html", "wallet.html", "plans.html", "pay.html", "terms.html", "withdraw.html", "reminder.html",
		"panel/admin_login.html", "panel/admin_dashboard.html", "panel/admin_questions.html",
		"panel/admin_tasks.html", "panel/admin_task_edit.html", "panel/admin_users.html",
		"panel/admin_user_edit.html", "panel/admin_withdrawals.html",
		"panel/partner_login.html", "panel/partner_dashboard.html",
	} {
		if tmpl[page] == nil {
			t.Errorf("нет шаблона %s", page)
		} else if tmpl[page].Lookup("layout") == nil {
			t.Errorf("%s: нет блока layout", page)
		}
	}
}

func TestStaticEmbedded(t *testing.T) {
	for _, f := range []string{"web/static/style.css", "web/static/bootstrap.js"} {
		if _, err := staticFS.ReadFile(f); err != nil {
			t.Errorf("%s не встроен: %v", f, err)
		}
	}
}

func TestLoadDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "# comment\n\nV2_TEST_A=1\n V2_TEST_B = two \nV2_TEST_KEEP=from-file\nbroken-line\nV2_TEST_EQ=a=b\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("V2_TEST_KEEP", "from-env")
	for _, k := range []string{"V2_TEST_A", "V2_TEST_B", "V2_TEST_EQ"} {
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}

	loadDotEnv(path)

	want := map[string]string{"V2_TEST_A": "1", "V2_TEST_B": "two", "V2_TEST_KEEP": "from-env", "V2_TEST_EQ": "a=b"}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestTemplateFuncs(t *testing.T) {
	f := templateFuncs()
	if f["add"].(func(int, int) int)(2, -1) != 1 {
		t.Error("add")
	}
	if got := f["seq"].(func(int, int) []int)(1, 3); len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Errorf("seq = %v", got)
	}
}
