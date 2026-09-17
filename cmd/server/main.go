// Version 2.0 — Telegram Mini App backend на Go 1.22, серверный рендеринг HTML.
package main

import (
	"bufio"
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"version20/internal/auth"
	"version20/internal/cron"
	"version20/internal/db"
	"version20/internal/handlers"
	"version20/internal/models"
	"version20/internal/store"
)

//go:embed web/templates/layout.html web/templates/partials/*.html web/templates/pages/*.html
var templatesFS embed.FS

//go:embed web/static
var staticFS embed.FS

//go:embed data/tasks_male.json data/tasks_female.json
var dataFS embed.FS

func main() {
	loadDotEnv(".env")

	port := getenv("PORT", "3000")
	databaseURL := os.Getenv("DATABASE_URL")
	botToken := os.Getenv("BOT_TOKEN")
	adminTgID := os.Getenv("ADMIN_TG_ID")
	sessionSecret := getenv("SESSION_SECRET", "dev-insecure-secret-change-me")
	devFakeAuth := os.Getenv("DEV_ALLOW_FAKE_AUTH") == "true"

	if sessionSecret == "dev-insecure-secret-change-me" {
		log.Println("[server] SESSION_SECRET не задан — используется небезопасный дефолт (только для разработки).")
	}
	if botToken == "" {
		log.Println("[server] BOT_TOKEN не задан — уведомления и сообщения бота отключены.")
	}
	if adminTgID == "" {
		log.Println("[server] ADMIN_TG_ID не задан — выдача премиум-агентских кодов недоступна.")
	}

	conn, err := db.Open(databaseURL)
	if err != nil {
		log.Fatalf("[server] не удалось открыть подключение к БД: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		log.Fatalf("[server] не удалось применить схему БД при старте: %v", err)
	}

	st := store.New(conn)

	tasksMale := loadTasks(dataFS, "data/tasks_male.json")
	tasksFemale := loadTasks(dataFS, "data/tasks_female.json")

	tmpl := loadTemplates(templatesFS)

	staticSub, err := fs.Sub(staticFS, "web/static")
	if err != nil {
		log.Fatalf("[server] не удалось подготовить статику: %v", err)
	}

	app := &handlers.App{
		Store:       st,
		Sessions:    auth.NewSessions(sessionSecret),
		Tmpl:        tmpl,
		StaticFS:    staticSub,
		TasksMale:   tasksMale,
		TasksFemale: tasksFemale,
		BotToken:    botToken,
		AdminTgID:   adminTgID,
		DevFakeAuth: devFakeAuth,
	}

	cron.Start(st, botToken)

	log.Println("Version 2.0 (Go) слушает порт " + port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, app.Routes()))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv — минимальный .env-загрузчик для локальной разработки (в
// Docker/Amvera переменные приходят из окружения контейнера напрямую).
// Не переопределяет уже установленные переменные окружения.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
}

func loadTasks(fsys embed.FS, path string) []models.Task {
	b, err := fsys.ReadFile(path)
	if err != nil {
		log.Fatalf("[server] не удалось прочитать %s: %v", path, err)
	}
	var tasks []models.Task
	if err := json.Unmarshal(b, &tasks); err != nil {
		log.Fatalf("[server] не удалось разобрать %s: %v", path, err)
	}
	return tasks
}

func loadTemplates(fsys embed.FS) map[string]*template.Template {
	pages, err := fs.Glob(fsys, "web/templates/pages/*.html")
	if err != nil || len(pages) == 0 {
		log.Fatalf("[server] не найдены шаблоны страниц: %v", err)
	}
	partials, err := fs.Glob(fsys, "web/templates/partials/*.html")
	if err != nil {
		log.Fatalf("[server] ошибка поиска partials: %v", err)
	}

	out := map[string]*template.Template{}
	for _, p := range pages {
		name := filepath.Base(p)
		files := append([]string{"web/templates/layout.html"}, partials...)
		files = append(files, p)
		t, err := template.New(name).Funcs(templateFuncs()).ParseFS(fsys, files...)
		if err != nil {
			log.Fatalf("[server] ошибка разбора шаблона %s: %v", p, err)
		}
		out[name] = t
	}
	return out
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"seq": func(from, to int) []int {
			out := make([]int, 0, to-from+1)
			for i := from; i <= to; i++ {
				out = append(out, i)
			}
			return out
		},
	}
}
