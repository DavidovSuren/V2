// Command seed наполняет локальную БД демо-данными: пользователи на разных
// этапах (новичок без анкеты, активные со стриком, 100 уровень), подписки,
// реферальная цепочка с премиум-агентами (выданный админом и ставший агентом
// сам через Premium), кошелёк, друзья, дневник.
//
// Запуск (из корня репозитория):
//
//	DATABASE_URL=postgres://v2:v2@localhost:5433/v2?sslmode=disable go run ./cmd/seed -reset
//
// Войти под любым пользователем локально: DEV_ALLOW_FAKE_AUTH=true и
// bootstrap с телом "debug:<tg_id>" (tg_id 100001..100014).
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"

	"version20/internal/achievements"
	"version20/internal/content"
	"version20/internal/db"
	"version20/internal/leveling"
	"version20/internal/models"
	"version20/internal/referrals"
	"version20/internal/scheduler"
	"version20/internal/store"
)

type spec struct {
	tgID, username, name, gender, age string
	// history — действия по дням, последний элемент = сегодня (МСК).
	// 'd' — выполнено, 's' — пропуск, '.' — не заходил.
	history  string
	noQuiz   bool
	agent    string // premium_agent_code
	refBy    string // tg_id пригласившего
	refAgent bool   // пришёл по агентскому коду
	pays     string // тариф, который оплатил: plus369 | premium888
}

// Пароль кабинета агента (/partner) у всех засеянных агентов.
const agentPassword = "agent12345"

var notes = []string{
	"", "", "Было непросто, но сделал(а)", "Лёгкий день", "Понравилось!",
	"Заметил(а) разницу уже сегодня", "Устал(а), но выполнил(а)", "",
}

func main() {
	reset := flag.Bool("reset", false, "очистить все таблицы перед заполнением")
	resetContent := flag.Bool("reset-content", false, "сбросить вопросы и задания к исходным (отменяет правки из админки)")
	dataDir := flag.String("data", "cmd/server/data", "каталог с tasks_male.json / tasks_female.json")
	flag.Parse()

	conn, err := db.Open(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Migrate(conn); err != nil {
		log.Fatal(err)
	}
	if *reset {
		_, err := conn.Exec(`TRUNCATE users, quiz_answers, category_weights, user_schedule, diary_entries,
			action_log, achievements, referrals, subscription_payments, wallet_transactions,
			friendships, promo_redemptions RESTART IDENTITY CASCADE`)
		if err != nil {
			log.Fatal(err)
		}
	}
	if *resetContent {
		if _, err := conn.Exec(`TRUNCATE questions, tasks`); err != nil {
			log.Fatal(err)
		}
	}
	st := store.New(conn)
	// Вопросы и задания — из БД (как в приложении), с досевом из кода/JSON.
	cache, err := content.Load(st, models.Questions,
		loadTasks(*dataDir+"/tasks_male.json"), loadTasks(*dataDir+"/tasks_female.json"))
	if err != nil {
		log.Fatal(err)
	}
	banks := map[string][]models.Task{"male": cache.TaskBank("male"), "female": cache.TaskBank("female")}
	rng := rand.New(rand.NewSource(42))

	specs := []spec{
		{tgID: "100001", username: "alex_agent", name: "Алексей", gender: "male", age: "26-30",
			history: repeat("d", 40) + "s" + repeat("d", 32), agent: "AGENT01", pays: "premium888"},
		{tgID: "100002", username: "masha", name: "Мария", gender: "female", age: "21-25",
			history: repeat("dds", 10) + repeat("d", 9), pays: "plus369"},
		{tgID: "100003", username: "igor", name: "Игорь", gender: "male", age: "31-35",
			history: "dddd.dddds.ddd" + ".."},
		{tgID: "100004", username: "anna", name: "Анна", gender: "female", age: "21-25",
			history: repeat("d", 14), refBy: "100001", refAgent: true, pays: "premium888"},
		{tgID: "100005", username: "dima", name: "Дмитрий", gender: "male", age: "17-20",
			history: repeat("d", 8), refBy: "100002", pays: "plus369"},
		{tgID: "100006", username: "olga", name: "Ольга", gender: "female", age: "36-45",
			noQuiz: true, refBy: "100002"},
		{tgID: "100007", username: "sergey_pro", name: "Сергей", gender: "male", age: "26-30",
			history: repeat("d", 365)},
		{tgID: "100008", username: "kate", name: "Катя", gender: "female", age: "26-30",
			history: repeat("d", 5), refBy: "100001", refAgent: true, pays: "plus369"},
		{tgID: "100009", username: "", name: "Павел", gender: "male", age: "45+",
			history: "d"},
		// Лена оформила Premium и сама стала агентом (кнопка «Стать агентом»).
		{tgID: "100010", username: "lena", name: "Лена", gender: "female", age: "31-35",
			history: repeat("ddds", 6) + "dd", refBy: "100002", pays: "premium888", agent: "LENA8888"},
		{tgID: "100011", username: "nikita", name: "Никита", gender: "male", age: "21-25",
			history: repeat("d", 3), refBy: "100001", refAgent: true},
		{tgID: "100012", username: "vika", name: "Вика", gender: "female", age: "17-20",
			noQuiz: true},
		{tgID: "100013", username: "roma", name: "Рома", gender: "male", age: "21-25",
			history: repeat("d", 10), refBy: "100010", refAgent: true, pays: "premium888"},
		{tgID: "100014", username: "sonya", name: "Соня", gender: "female", age: "26-30",
			history: repeat("dd.", 4), refBy: "100010", refAgent: true, pays: "plus369"},
	}

	ids := map[string]int64{}
	now := time.Now().UTC()
	today := mustDate(todayMoscow())

	for _, s := range specs {
		created := today.AddDate(0, 0, -len(s.history)-1)
		var refBy sql.NullInt64
		var refType sql.NullString
		if s.refBy != "" {
			refBy = sql.NullInt64{Int64: ids[s.refBy], Valid: true}
			t := "normal"
			if s.refAgent {
				t = "premium_agent"
			}
			refType = sql.NullString{String: t, Valid: true}
		}
		id, err := st.CreateUser(store.NewUser{
			TgID:               s.tgID,
			Username:           sql.NullString{String: s.username, Valid: s.username != ""},
			Name:               s.name,
			AgeGroup:           s.age,
			PhotosJSON:         `["local-photo"]`,
			CreatedAt:          created.Format(time.RFC3339),
			ReferralCode:       "REF" + s.tgID[3:],
			ReferredByUserID:   refBy,
			ReferredByCodeType: refType,
		})
		if err != nil {
			log.Fatalf("user %s: %v", s.tgID, err)
		}
		ids[s.tgID] = id
		if s.agent != "" {
			must(st.SetPremiumAgentCode(id, s.agent))
			hash, err := bcrypt.GenerateFromPassword([]byte(agentPassword), bcrypt.DefaultCost)
			must(err)
			must(st.SetAgentPasswordHash(id, string(hash)))
		}
		if refBy.Valid {
			must(st.CreateReferral(conn, refBy.Int64, id, refType.String, created.Format(time.RFC3339)))
		}
		if !s.noQuiz {
			seedProgress(conn, st, rng, id, s, cache.Questions(), banks[s.gender], today)
		}
		if s.pays != "" {
			pay(conn, st, id, s, now)
		}
		u, err := st.GetUserByID(id)
		must(err)
		_, err = achievements.CheckAndUnlock(st, u)
		must(err)
	}

	// Друзья (взаимно, как при добавлении по @username).
	for _, p := range [][2]string{{"100001", "100002"}, {"100001", "100004"}, {"100002", "100005"},
		{"100002", "100010"}, {"100007", "100001"}, {"100003", "100009"}} {
		ts := now.Format(time.RFC3339)
		must(st.AddFriendship(ids[p[0]], ids[p[1]], ts))
		must(st.AddFriendship(ids[p[1]], ids[p[0]], ts))
	}

	log.Printf("seed готов: %d пользователей (tg_id 100001..100014). Вход: DEV_ALLOW_FAKE_AUTH=true, initData \"debug:<tg_id>\"", len(specs))
	log.Printf("кабинет агента /partner: коды AGENT01 и LENA8888, пароль %q", agentPassword)
}

// seedProgress проходит анкету случайными ответами, строит план тем же
// scheduler'ом, что и приложение, и проигрывает историю дней так же, как
// handleDiarySubmit/MarkSkip, чтобы стрики, XP и уровни были согласованы.
func seedProgress(conn *sql.DB, st *store.Store, rng *rand.Rand, id int64, s spec, questions []models.Question, bank []models.Task, today time.Time) {
	answers := map[int]string{}
	for _, q := range questions {
		var a string
		switch {
		case q.ID == 0:
			a = s.gender
		case q.Type == "scale":
			a = strconv.Itoa(2 + rng.Intn(8))
		case q.Type == "single":
			a = q.Options[rng.Intn(len(q.Options))].Value
		default:
			a = "Хочу стать увереннее и собраннее"
		}
		answers[q.ID] = a
		b, _ := json.Marshal(a)
		must(st.UpsertQuizAnswer(conn, id, q.ID, string(b)))
	}
	weights := scheduler.ComputeCategoryWeightsFor(questions, answers)
	for _, cat := range models.Categories {
		must(st.UpsertCategoryWeight(conn, id, cat, weights[cat]))
	}
	schedule := scheduler.BuildSchedule(bank, weights)
	for i, t := range schedule {
		must(st.InsertScheduleRow(conn, id, i, t))
	}
	must(st.FinishQuiz(conn, id, s.gender, leveling.QuizCompleteXP))

	completed, streak, best := 0, 0, 0
	lastDate := ""
	start := today.AddDate(0, 0, -(len(s.history) - 1))
	for i, c := range s.history {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		switch c {
		case 'd':
			if completed >= len(schedule) {
				continue
			}
			if lastDate != "" && mustDate(lastDate).AddDate(0, 0, 1).Format("2006-01-02") == date {
				streak++
			} else {
				streak = 1
			}
			if streak > best {
				best = streak
			}
			xp := leveling.XPForTaskCompletion(streak)
			completed++
			_, level := leveling.ProgressFromCompleted(completed)
			must(st.UpdateScheduleStatus(conn, id, completed-1, "done"))
			must(st.InsertDiaryEntry(conn, id, date, strconv.Itoa(2+rng.Intn(4)), notes[rng.Intn(len(notes))], xp))
			must(st.InsertActionLog(conn, id, date, "done", schedule[completed-1].Category))
			must(st.ApplyCompletion(conn, id, completed, xp, level, streak, best, date))
			if level >= 100 {
				exp := time.Now().UTC().AddDate(0, 6, 0).Format(time.RFC3339)
				must(st.GrantLevel100Premium(conn, id, exp))
			}
			lastDate = date
		case 's':
			must(st.MarkSkip(id, date))
			must(st.InsertActionLog(conn, id, date, "skip", schedule[completed].Category))
			streak = 0
			lastDate = date
		default:
			// не заходил: стрик обнулится при следующем выполнении
		}
	}
}

// pay повторяет handleSubscriptionPay: скидка по обычному коду, 50% комиссии
// премиум-агенту в кошелёк.
func pay(conn *sql.DB, st *store.Store, id int64, s spec, now time.Time) {
	u, err := st.GetUserByID(id)
	must(err)
	base := referrals.Prices[s.pays]
	discount := 0
	var commissionTo sql.NullInt64
	commission := 0
	if u.ReferredByUserID.Valid {
		if u.ReferredByCodeType.String == "premium_agent" {
			commissionTo = u.ReferredByUserID
		} else {
			n, err := st.PriorPaidReferralsCount(u.ReferredByUserID.Int64)
			must(err)
			discount = referrals.DiscountPctForReferrer(n)
		}
	}
	paid := referrals.PriceAfterDiscount(base, discount)
	if commissionTo.Valid {
		commission = referrals.CommissionAmount(paid)
	}
	paidAt := now.AddDate(0, 0, -3)
	must(st.InsertSubscriptionPayment(conn, id, s.pays, base, discount, paid, commissionTo, commission, paidAt.Format(time.RFC3339)))
	// Не затираем подарочный Premium за 100 уровень.
	if u.SubscriptionTier == "free" {
		must(st.UpdateSubscription(conn, id, s.pays, paidAt.AddDate(0, 1, 0).Format(time.RFC3339)))
	}
	if commissionTo.Valid {
		must(st.InsertWalletTransaction(conn, commissionTo.Int64, commission, id, "Комиссия с оплаты "+s.pays, paidAt.Format(time.RFC3339)))
	}
}

func todayMoscow() string {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		loc = time.FixedZone("MSK", 3*3600)
	}
	return time.Now().In(loc).Format("2006-01-02")
}

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	must(err)
	return t
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

func loadTasks(path string) []models.Task {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	var tasks []models.Task
	must(json.Unmarshal(b, &tasks))
	return tasks
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
