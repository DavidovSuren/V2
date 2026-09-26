package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"version20/internal/leveling"
	"version20/internal/models"
	"version20/internal/reports"
	"version20/internal/scheduler"
)

// Сквозные сценарии поверх PostgreSQL (нужен TEST_DATABASE_URL). Каждый
// проверяет то же поведение, что было у Node-версии (коммит 14bc237).

func TestOnboardingRequiresFields(t *testing.T) {
	a := newDBApp(t)
	r := a.post(t, "1", "/onboarding", url.Values{"name": {"A"}, "ageGroup": {"18-24"}})
	if r.Code != 200 || !strings.Contains(r.Body, "загрузи хотя бы одно фото") {
		t.Fatalf("без фото: %d", r.Code)
	}
	if u, _ := a.Store.GetUserByTgID("1"); u != nil {
		t.Fatal("пользователь создан без фото")
	}
}

func TestOnboardingCreatesUserAndReferral(t *testing.T) {
	a := newDBApp(t)
	if r := a.onboard(t, "1", ""); r.Location != "/quiz/0" {
		t.Fatalf("%d %q", r.Code, r.Location)
	}
	referrer := a.user(t, "1")
	if !referrer.ReferralCode.Valid || len(referrer.ReferralCode.String) != 7 {
		t.Errorf("реф. код: %v", referrer.ReferralCode)
	}
	if referrer.Name != "User 1" || referrer.AgeGroup != "25-34" || referrer.PhotosJSON != `["local-photo"]` {
		t.Errorf("профиль: %+v", referrer)
	}

	a.onboard(t, "2", referrer.ReferralCode.String)
	invited := a.user(t, "2")
	if invited.ReferredByUserID.Int64 != referrer.ID || invited.ReferredByCodeType.String != "normal" {
		t.Errorf("приглашение: %v %v", invited.ReferredByUserID, invited.ReferredByCodeType)
	}
	if n := a.count(t, "SELECT COUNT(*) FROM referrals WHERE referrer_id=$1 AND referred_id=$2", referrer.ID, invited.ID); n != 1 {
		t.Errorf("referrals: %d", n)
	}

	// Несуществующий код не ломает регистрацию.
	a.onboard(t, "3", "NOPE")
	if u := a.user(t, "3"); u.ReferredByUserID.Valid {
		t.Error("привязан к несуществующему коду")
	}

	// Повторная отправка обновляет профиль, а не создаёт второго пользователя.
	a.onboard(t, "3", "")
	if n := a.count(t, "SELECT COUNT(*) FROM users WHERE tg_id='3'"); n != 1 {
		t.Errorf("дубликат пользователя: %d", n)
	}
}

func TestIndexRoutesByState(t *testing.T) {
	a := newDBApp(t)
	r := a.get(t, "1", "/")
	if r.Code != 200 || strings.Contains(r.Body, "bootstrap-target") || !strings.Contains(r.Body, `action="/onboarding"`) {
		t.Errorf("есть сессия, нет регистрации: %d", r.Code)
	}
	a.onboard(t, "1", "")
	if r := a.get(t, "1", "/"); r.Location != "/quiz/0" {
		t.Errorf("без анкеты: %q", r.Location)
	}
	// Разделы, требующие анкету, уводят на "/".
	for _, p := range []string{"/diary", "/today/skip"} {
		if r := a.get(t, "1", p); r.Location != "/" {
			t.Errorf("%s без анкеты: %q", p, r.Location)
		}
	}
}

func TestQuizBuildsPersonalSchedule(t *testing.T) {
	a := newDBApp(t)
	a.onboard(t, "1", "")

	// Пустой ответ не принимается.
	if r := a.post(t, "1", "/quiz/3", url.Values{"answer": {"  "}}); r.Location != "/quiz/3" {
		t.Errorf("пустой ответ: %q", r.Location)
	}
	// Невалидный номер вопроса.
	if r := a.get(t, "1", "/quiz/99"); r.Location != "/quiz/0" {
		t.Errorf("/quiz/99: %q", r.Location)
	}

	last := a.completeQuiz(t, "1", "female")
	if !strings.HasPrefix(last.Location, "/?focus=") {
		t.Fatalf("после анкеты: %q", last.Location)
	}
	focus, _ := url.QueryUnescape(strings.TrimPrefix(last.Location, "/?focus="))
	if len(strings.Split(focus, ",")) != 3 {
		t.Errorf("фокус-направления: %q", focus)
	}

	u := a.user(t, "1")
	if u.Gender.String != "female" || u.XP != 50 || u.DayIndex != 0 {
		t.Errorf("после анкеты: gender=%v xp=%d day=%d", u.Gender, u.XP, u.DayIndex)
	}

	answers := answersFor("female")
	want := scheduler.BuildSchedule(testBank(t, "female"), scheduler.ComputeCategoryWeights(answers))
	if n := a.count(t, "SELECT COUNT(*) FROM user_schedule WHERE user_id=$1", u.ID); n != 365 {
		t.Fatalf("план: %d строк", n)
	}
	for _, day := range []int{0, 1, 100, 364} {
		row, _ := a.Store.GetScheduleRow(u.ID, day)
		if row.TaskID != want[day].ID || row.Status != "pending" {
			t.Errorf("день %d: %s, want %s", day, row.TaskID, want[day].ID)
		}
	}
	if n := a.count(t, "SELECT COUNT(*) FROM category_weights WHERE user_id=$1", u.ID); n != 8 {
		t.Errorf("веса: %d", n)
	}

	home := a.do(t, "GET", last.Location, nil, "", a.session("1"))
	mustContain(t, home.Body, "Твой план готов", "День 1 из 365", want[0].Text, "ВЫПОЛНИЛ(А)")

	// Анкета второй раз не проходится: раньше это падало на PK user_schedule
	// и повторно начисляло XP.
	if r := a.post(t, "1", "/quiz/20", url.Values{"answer": {"x"}}); r.Location != "/" {
		t.Errorf("повторная анкета: %d %q", r.Code, r.Location)
	}
	if r := a.get(t, "1", "/quiz/0"); r.Location != "/" {
		t.Errorf("GET анкеты после прохождения: %q", r.Location)
	}
	if u := a.user(t, "1"); u.XP != 50 {
		t.Errorf("XP после повторной попытки: %d", u.XP)
	}
}

func TestDiaryCompletesTask(t *testing.T) {
	a := newDBApp(t)
	u := a.newPlayer(t, "1", "")
	first, _ := a.Store.GetScheduleRow(u.ID, 0)

	if r := a.post(t, "1", "/diary", url.Values{"emoji": {"9"}}); r.Location != "/diary" {
		t.Errorf("неверная эмоция: %q", r.Location)
	}

	r := a.post(t, "1", "/diary", url.Values{"emoji": {"4"}, "note": {"сделал"}})
	if r.Location != "/" {
		t.Fatalf("diary: %d %q", r.Code, r.Location)
	}
	u = a.user(t, "1")
	if u.CompletedCount != 1 || u.DayIndex != 1 || u.StreakCurrent != 1 || u.StreakBest != 1 || u.XP != 60 || u.Level != 0 {
		t.Errorf("после выполнения: %+v", u)
	}
	if u.LastActionDate.String != reports.TodayMoscow() {
		t.Errorf("last_action_date: %v", u.LastActionDate)
	}
	row, _ := a.Store.GetScheduleRow(u.ID, 0)
	if row.Status != "done" {
		t.Error("задание не отмечено done")
	}
	if n := a.count(t, "SELECT COUNT(*) FROM action_log WHERE user_id=$1 AND action='done' AND category=$2", u.ID, first.Category); n != 1 {
		t.Errorf("action_log: %d", n)
	}
	if n := a.count(t, "SELECT COUNT(*) FROM diary_entries WHERE user_id=$1 AND emoji='4' AND note='сделал' AND xp_awarded=10", u.ID); n != 1 {
		t.Errorf("diary_entries: %d", n)
	}

	// Одно действие в день: повторы блокируются.
	a.post(t, "1", "/diary", url.Values{"emoji": {"5"}})
	a.post(t, "1", "/today/skip", nil)
	if u2 := a.user(t, "1"); u2.CompletedCount != 1 || u2.StreakCurrent != 1 {
		t.Errorf("второе действие за день прошло: %+v", u2)
	}
	if r := a.get(t, "1", "/today/skip"); r.Location != "/" {
		t.Errorf("экран пропуска после действия: %q", r.Location)
	}
	mustContain(t, a.get(t, "1", "/").Body, "Выполнено сегодня")

	// Динамика настроения на странице дневника.
	mustContain(t, a.get(t, "1", "/diary").Body, "Нед. ", "/5")
}

func TestStreakXPAndAchievement(t *testing.T) {
	a := newDBApp(t)
	u := a.newPlayer(t, "1", "")
	yesterday := reports.FromNDaysAgo(1)
	a.exec(t, "UPDATE users SET streak_current=6, streak_best=6, last_action_date=$1, completed_count=6, day_index=6 WHERE id=$2", yesterday, u.ID)

	r := a.post(t, "1", "/diary", url.Values{"emoji": {"3"}})
	if r.Location != "/?celebrate=7d" {
		t.Errorf("редирект: %q", r.Location)
	}
	u = a.user(t, "1")
	if u.StreakCurrent != 7 || u.StreakBest != 7 || u.XP != 50+15 {
		t.Errorf("стрик/XP: streak=%d best=%d xp=%d", u.StreakCurrent, u.StreakBest, u.XP)
	}
	mustContain(t, a.do(t, "GET", r.Location, nil, "", a.session("1")).Body, `data-autoshow="1"`, "7 ДНЕЙ")

	// Разрыв больше дня обнуляет стрик до 1. Сегодняшние записи переносим
	// в прошлое, иначе вторая запись дневника за день упрётся в PK.
	a.exec(t, "UPDATE diary_entries SET entry_date=$1 WHERE user_id=$2", reports.FromNDaysAgo(3), u.ID)
	a.exec(t, "UPDATE action_log SET action_date=$1 WHERE user_id=$2", reports.FromNDaysAgo(3), u.ID)
	a.exec(t, "UPDATE users SET last_action_date=$1 WHERE id=$2", reports.FromNDaysAgo(3), u.ID)
	a.post(t, "1", "/diary", url.Values{"emoji": {"3"}})
	if u = a.user(t, "1"); u.StreakCurrent != 1 || u.StreakBest != 7 {
		t.Errorf("после перерыва: streak=%d best=%d", u.StreakCurrent, u.StreakBest)
	}

	ach := a.get(t, "1", "/achievements").Body
	mustContain(t, ach, "7 ДНЕЙ")
}

func TestSkipKeepsTaskAndResetsStreak(t *testing.T) {
	a := newDBApp(t)
	u := a.newPlayer(t, "1", "")
	a.exec(t, "UPDATE users SET streak_current=4 WHERE id=$1", u.ID)

	if r := a.get(t, "1", "/today/skip"); r.Code != 200 {
		t.Fatalf("экран подтверждения: %d", r.Code)
	}
	if r := a.post(t, "1", "/today/skip", nil); r.Location != "/" {
		t.Fatalf("skip: %q", r.Location)
	}
	u = a.user(t, "1")
	if u.StreakCurrent != 0 || u.DayIndex != 0 || u.CompletedCount != 0 || u.LastActionDate.String != reports.TodayMoscow() {
		t.Errorf("после пропуска: %+v", u)
	}
	if n := a.count(t, "SELECT COUNT(*) FROM action_log WHERE user_id=$1 AND action='skip'", u.ID); n != 1 {
		t.Errorf("action_log skip: %d", n)
	}
	// Выполнить в тот же день уже нельзя.
	a.post(t, "1", "/diary", url.Values{"emoji": {"3"}})
	if a.user(t, "1").CompletedCount != 0 {
		t.Error("выполнение после пропуска в тот же день")
	}

	// На следующий день — то же самое задание.
	a.exec(t, "UPDATE users SET last_action_date=$1 WHERE id=$2", reports.FromNDaysAgo(1), u.ID)
	first, _ := a.Store.GetScheduleRow(u.ID, 0)
	mustContain(t, a.get(t, "1", "/").Body, first.Text)
}

func TestLevel100GrantsDiamondAndPremium(t *testing.T) {
	a := newDBApp(t)
	u := a.newPlayer(t, "1", "")
	a.exec(t, "UPDATE users SET completed_count=364, day_index=364, level=99 WHERE id=$1", u.ID)

	r := a.post(t, "1", "/diary", url.Values{"emoji": {"5"}})
	if r.Location != "/?celebrate=level100" {
		t.Errorf("редирект: %q", r.Location)
	}
	u = a.user(t, "1")
	if u.Level != 100 || u.CompletedCount != 365 || u.DayIndex != 365 || u.SubscriptionTier != "premium888" {
		t.Errorf("уровень 100: %+v", u)
	}
	exp, err := time.Parse(time.RFC3339, u.SubscriptionExpiresAt.String)
	if err != nil || exp.Before(time.Now().AddDate(0, 5, 25)) || exp.After(time.Now().AddDate(0, 6, 1)) {
		t.Errorf("Premium на 6 месяцев: %v", u.SubscriptionExpiresAt)
	}
	for _, c := range []string{"lvl25", "lvl50", "lvl75", "lvl100"} {
		if n := a.count(t, "SELECT COUNT(*) FROM achievements WHERE user_id=$1 AND code=$2", u.ID, c); n != 1 {
			t.Errorf("нет бейджа %s", c)
		}
	}
	mustContain(t, a.do(t, "GET", r.Location, nil, "", a.session("1")).Body, "УРОВЕНЬ 100!", "Все 365 заданий пройдены!")

	// Путь пройден: дальше ни выполнить, ни пропустить.
	a.exec(t, "UPDATE users SET last_action_date=NULL WHERE id=$1", u.ID)
	a.post(t, "1", "/diary", url.Values{"emoji": {"5"}})
	a.post(t, "1", "/today/skip", nil)
	if u2 := a.user(t, "1"); u2.CompletedCount != 365 || u2.XP != u.XP {
		t.Errorf("действие после 365/365: %+v", u2)
	}
}

func TestProgressPage(t *testing.T) {
	a := newDBApp(t)
	a.newPlayer(t, "1", "")
	a.post(t, "1", "/diary", url.Values{"emoji": {"3"}})

	p1 := a.get(t, "1", "/progress").Body
	p2 := a.get(t, "1", "/progress").Body
	if p1 != p2 {
		t.Error("порядок направлений на «Прогрессе» меняется между запросами")
	}
	var last int
	for _, c := range models.Categories {
		i := strings.Index(p1, c)
		if i < 0 {
			t.Errorf("нет направления %s", c)
		}
		last = i
	}
	_ = last
}

func TestReferralDiscountLadder(t *testing.T) {
	a := newDBApp(t)
	ref := a.newPlayer(t, "100", "")
	code := ref.ReferralCode.String

	// Скидка приглашённого зависит от числа уже оплативших до него:
	// 0–3 → 20%, 4–9 → 35%, 10+ → 50%.
	wantPct := []int{20, 20, 20, 20, 35, 35, 35, 35, 35, 35, 50, 50}
	wantPrice := map[int]int{20: 295, 35: 240, 50: 185}
	for i, pct := range wantPct {
		tg := "2" + strings.Repeat("0", 2) + string(rune('a'+i))
		a.newPlayer(t, tg, code)

		profile := a.get(t, tg, "/profile").Body
		mustContain(t, profile, "(-"+itoa(pct)+"%)")

		if r := a.post(t, tg, "/subscribe", url.Values{"tier": {"plus369"}}); r.Location != "/profile" {
			t.Fatalf("subscribe: %q", r.Location)
		}
		var gotPct, gotPrice, commission int
		a.Store.DB.QueryRow(`SELECT discount_pct, price_paid, commission_amount FROM subscription_payments
			WHERE user_id=$1`, a.user(t, tg).ID).Scan(&gotPct, &gotPrice, &commission)
		if gotPct != pct || gotPrice != wantPrice[pct] || commission != 0 {
			t.Errorf("оплата #%d: %d%% %d₽ комиссия %d, want %d%% %d₽", i+1, gotPct, gotPrice, commission, pct, wantPrice[pct])
		}
		u := a.user(t, tg)
		if u.SubscriptionTier != "plus369" {
			t.Errorf("тариф: %s", u.SubscriptionTier)
		}
	}

	// Без приглашения — полная цена; неизвестный тариф игнорируется.
	a.post(t, "100", "/subscribe", url.Values{"tier": {"gold"}})
	a.post(t, "100", "/subscribe", url.Values{"tier": {"premium888"}})
	var pct, price int
	a.Store.DB.QueryRow("SELECT discount_pct, price_paid FROM subscription_payments WHERE user_id=$1", ref.ID).Scan(&pct, &price)
	if pct != 0 || price != 888 {
		t.Errorf("без реферера: %d%% %d₽", pct, price)
	}
	if n := a.count(t, "SELECT COUNT(*) FROM subscription_payments WHERE user_id=$1", ref.ID); n != 1 {
		t.Errorf("неизвестный тариф записан: %d", n)
	}
}

func TestPremiumAgentFlow(t *testing.T) {
	a := newDBApp(t)
	agent := a.newPlayer(t, "200", "")
	a.exec(t, "UPDATE users SET username='agent007' WHERE id=$1", agent.ID)

	if r := a.get(t, "200", "/wallet"); r.Code != http.StatusForbidden {
		t.Errorf("кошелёк без кода: %d", r.Code)
	}
	if r := a.post(t, "200", "/wallet/withdraw", nil); r.Code != http.StatusForbidden {
		t.Errorf("вывод без кода: %d", r.Code)
	}

	if r := a.post(t, testAdminID, "/admin/grant-premium-agent", url.Values{"username": {"@nobody"}}); r.Code != http.StatusNotFound {
		t.Errorf("несуществующий: %d", r.Code)
	}
	r := a.post(t, testAdminID, "/admin/grant-premium-agent", url.Values{"username": {"@agent007"}})
	if r.Code != 200 || !strings.HasPrefix(r.Body, "код выдан: ") {
		t.Fatalf("выдача: %d %s", r.Code, r.Body)
	}
	agentCode := strings.TrimPrefix(r.Body, "код выдан: ")
	if len(agentCode) != 8 {
		t.Errorf("код: %q", agentCode)
	}
	if r := a.post(t, testAdminID, "/admin/grant-premium-agent", url.Values{"tgId": {"200"}}); r.Body != "уже выдан: "+agentCode {
		t.Errorf("повторная выдача: %s", r.Body)
	}
	mustContain(t, a.get(t, "200", "/profile").Body, `href="/wallet"`)

	// Приглашённый по агентскому коду платит полную цену, агенту — 50%.
	a.newPlayer(t, "201", agentCode)
	if u := a.user(t, "201"); u.ReferredByCodeType.String != "premium_agent" {
		t.Fatalf("тип кода: %v", u.ReferredByCodeType)
	}
	mustNotContain(t, a.get(t, "201", "/profile").Body, "old-price")
	a.post(t, "201", "/subscribe", url.Values{"tier": {"plus369"}})
	a.newPlayer(t, "202", agentCode)
	a.post(t, "202", "/subscribe", url.Values{"tier": {"premium888"}})

	var price, commission int
	a.Store.DB.QueryRow("SELECT price_paid, commission_amount FROM subscription_payments WHERE user_id=$1",
		a.user(t, "201").ID).Scan(&price, &commission)
	if price != 369 || commission != 185 {
		t.Errorf("агентская оплата 369: цена %d, комиссия %d (Node: 369 / 185)", price, commission)
	}

	w := a.get(t, "200", "/wallet")
	if w.Code != 200 {
		t.Fatalf("кошелёк: %d", w.Code)
	}
	mustContain(t, w.Body, "629 ₽", "<strong>2</strong>", "+444 ₽", "+185 ₽", "Комиссия с оплаты premium888")

	wd := a.post(t, "200", "/wallet/withdraw", nil)
	if wd.Location != "/wallet?msg=soon" {
		t.Fatalf("вывод: %q", wd.Location)
	}
	mustContain(t, a.get(t, "200", wd.Location).Body, "Вывод средств скоро появится")

	// Скидочная лестница агента не касается: у него нет "обычных" рефералов.
	if n, _ := a.Store.PriorPaidReferralsCount(agent.ID); n != 2 {
		t.Errorf("оплативших по коду: %d", n)
	}
}

func TestCommunityVisibility(t *testing.T) {
	a := newDBApp(t)
	a.newPlayer(t, "1", "")
	star := a.newPlayer(t, "2", "")
	a.exec(t, "UPDATE users SET level=100, name='Звезда' WHERE id=$1", star.ID)
	a.exec(t, "INSERT INTO achievements (user_id, code, unlocked_at) VALUES ($1,'lvl100','x')", star.ID)

	// Без Premium других участников не видно — только себя и счётчик скрытых.
	free := a.get(t, "1", "/community").Body
	mustContain(t, free, "Оформи подписку 888", "Ещё 1 участников скрыто")
	mustNotContain(t, free, "Звезда", "Ур. 100", "💎")

	friends := a.get(t, "1", "/community?tab=friends").Body
	mustContain(t, friends, "доступен только с подпиской 888")
	mustNotContain(t, friends, `action="/friends/add"`)

	// Plus не открывает уровни — только Premium.
	a.post(t, "1", "/subscribe", url.Values{"tier": {"plus369"}})
	mustNotContain(t, a.get(t, "1", "/community").Body, "Ур. 100")

	a.post(t, "1", "/subscribe", url.Values{"tier": {"premium888"}})
	prem := a.get(t, "1", "/community").Body
	mustContain(t, prem, "Звезда", "Ур. 100", "💎")
	mustNotContain(t, prem, "Оформи подписку 888", "скрыто")

	// Истёкшая подписка снова прячет уровни.
	a.exec(t, "UPDATE users SET subscription_expires_at=$1 WHERE tg_id='1'", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))
	mustNotContain(t, a.get(t, "1", "/community").Body, "Ур. 100")
}

func TestBecomeAgent(t *testing.T) {
	a := newDBApp(t)
	a.newPlayer(t, "1", "")

	// Без Premium — отказ с понятной ошибкой в профиле.
	if r := a.post(t, "1", "/agent/become", nil); r.Location != "/profile?agent_err=premium" {
		t.Fatalf("free: redirect %q", r.Location)
	}
	mustContain(t, a.get(t, "1", "/profile?agent_err=premium").Body, "только с активной подпиской Premium")
	if u := a.user(t, "1"); u.HasPremiumAgentCode() {
		t.Fatal("free не должен стать агентом")
	}

	a.post(t, "1", "/subscribe", url.Values{"tier": {"premium888"}})
	mustContain(t, a.get(t, "1", "/profile").Body, `action="/agent/become"`)
	if r := a.post(t, "1", "/agent/become", nil); r.Location != "/wallet" {
		t.Fatalf("premium: redirect %q", r.Location)
	}
	u := a.user(t, "1")
	if len(u.PremiumAgentCode.String) != 8 {
		t.Fatalf("код: %q", u.PremiumAgentCode.String)
	}
	mustContain(t, a.get(t, "1", "/profile").Body, u.PremiumAgentCode.String)

	// Повторный запрос не меняет код.
	a.post(t, "1", "/agent/become", nil)
	if a.user(t, "1").PremiumAgentCode.String != u.PremiumAgentCode.String {
		t.Fatal("код изменился")
	}

	// Приглашённый по коду приносит 50% в кошелёк.
	a.newPlayer(t, "2", u.PremiumAgentCode.String)
	a.post(t, "2", "/subscribe", url.Values{"tier": {"premium888"}})
	if bal, _ := a.Store.WalletBalance(u.ID); bal != 444 {
		t.Fatalf("баланс агента %d, want 444", bal)
	}
}

func TestContactsAndAddFriend(t *testing.T) {
	a := newDBApp(t)
	me := a.newPlayer(t, "1", "")
	a.exec(t, "UPDATE users SET username='me', subscription_tier='premium888', subscription_expires_at=$1 WHERE id=$2",
		time.Now().Add(24*time.Hour).UTC().Format(time.RFC3339), me.ID)
	friend := a.newPlayer(t, "2", "")
	a.exec(t, "UPDATE users SET username='buddy', name='Бадди' WHERE id=$1", friend.ID)
	a.newPlayer(t, "3", me.ReferralCode.String) // приглашённый — тоже в контактах
	a.exec(t, "UPDATE users SET name='Приглашённый' WHERE tg_id='3'")
	a.newPlayer(t, "4", "")
	a.exec(t, "UPDATE users SET name='Чужой' WHERE tg_id='4'")

	r := a.post(t, "1", "/friends/add", url.Values{"username": {"@nobody"}})
	if r.Location != "/community?tab=friends&err=notfound" {
		t.Errorf("не найден: %q", r.Location)
	}
	mustContain(t, a.get(t, "1", r.Location).Body, "Такой пользователь не найден в приложении")

	r = a.post(t, "1", "/friends/add", url.Values{"username": {"@me"}})
	mustContain(t, a.get(t, "1", r.Location).Body, "Нельзя добавить самого себя")

	a.post(t, "1", "/friends/add", url.Values{"username": {"@buddy"}})
	a.post(t, "1", "/friends/add", url.Values{"username": {"buddy"}}) // повтор не дублирует
	if n := a.count(t, "SELECT COUNT(*) FROM friendships"); n != 2 {
		t.Errorf("дружба в обе стороны: %d строк", n)
	}

	contacts := a.get(t, "1", "/community?tab=friends").Body
	mustContain(t, contacts, "Бадди", "Приглашённый", `action="/friends/add"`)
	mustNotContain(t, contacts, "Чужой")
}

func TestPromo100LVL(t *testing.T) {
	a := newDBApp(t)
	a.onboard(t, "1", "")
	r := a.post(t, "1", "/promo", url.Values{"code": {"100lvl"}})
	mustContain(t, a.get(t, "1", r.Location).Body, "Сначала пройди анкету")

	a.completeQuiz(t, "1", "male")
	r = a.post(t, "1", "/promo", url.Values{"code": {"WRONG"}})
	mustContain(t, a.get(t, "1", r.Location).Body, "Промокод не найден")

	mustNotContain(t, a.get(t, "1", "/").Body, "diamond-badge")

	// Пользователи вводят и "LVL100" — это то же самое, что "100LVL".
	r = a.post(t, "1", "/promo", url.Values{"code": {" lvl100 "}})
	if r.Location != "/?celebrate=level100" {
		t.Fatalf("промокод: %q", r.Location)
	}
	u := a.user(t, "1")
	if u.Level != 100 || u.CompletedCount != 365 || u.DayIndex != 365 || u.StreakCurrent != 365 ||
		u.StreakBest != 365 || u.SubscriptionTier != "premium888" || u.XP != leveling.FullYearXP() {
		t.Errorf("после промокода: %+v", u)
	}

	// Всё открыто на 100%: прогресс, все направления, алмаз у иконки.
	progress := a.get(t, "1", "/progress").Body
	mustContain(t, progress, "365 / 365", itoa(leveling.FullYearXP()), "365 дней")
	mustNotContain(t, progress, ">0%<")
	mustContain(t, a.get(t, "1", "/").Body, "diamond-badge", "100%")
	mustContain(t, a.get(t, "1", "/profile").Body, "diamond-badge", "<strong>100</strong>")
	if n := a.count(t, "SELECT COUNT(*) FROM achievements WHERE user_id=$1", u.ID); n != 8 {
		t.Errorf("достижений %d, want 8", n)
	}
	if n := a.count(t, "SELECT COUNT(*) FROM user_schedule WHERE user_id=$1 AND status='done'", u.ID); n != 365 {
		t.Errorf("план выполнен: %d/365", n)
	}

	for _, code := range []string{"100LVL", "LVL100"} {
		r = a.post(t, "1", "/promo", url.Values{"code": {code}})
		mustContain(t, a.get(t, "1", r.Location).Body, "Этот промокод уже использован")
	}
	if n := a.count(t, "SELECT COUNT(*) FROM promo_redemptions WHERE user_id=$1", u.ID); n != 1 {
		t.Errorf("погашений: %d", n)
	}
}

func TestUsernameSyncedFromTelegram(t *testing.T) {
	a := newDBApp(t)
	boot := func(username string) []*http.Cookie {
		body := signedInitData(map[string]string{
			"auth_date": "1700000000",
			"user":      `{"id":555,"first_name":"Ann","username":"` + username + `"}`,
		})
		r := a.do(t, "POST", "/auth/bootstrap", strings.NewReader(body), "")
		if r.Code != 200 {
			t.Fatalf("bootstrap: %d %s", r.Code, r.Body)
		}
		return r.Cookies
	}

	cookies := boot("ann_v2")
	uc := cookieByName(cookies, "v2_username")
	if uc == nil || uc.Value != "ann_v2" {
		t.Fatalf("username cookie: %v", uc)
	}
	a.onboard(t, "555", "", uc)
	if u := a.user(t, "555"); u.Username.String != "ann_v2" {
		t.Fatalf("username не сохранён при регистрации: %v", u.Username)
	}

	boot("ann_renamed")
	if u := a.user(t, "555"); u.Username.String != "ann_renamed" {
		t.Errorf("username не обновился: %v", u.Username)
	}

	// Благодаря этому работает выдача агентского кода по @username.
	r := a.post(t, testAdminID, "/admin/grant-premium-agent", url.Values{"username": {"@ann_renamed"}})
	if !strings.HasPrefix(r.Body, "код выдан: ") {
		t.Errorf("выдача по username: %d %s", r.Code, r.Body)
	}
}

func TestWeeklyReportAndLogout(t *testing.T) {
	a := newDBApp(t)
	a.newPlayer(t, "1", "")
	a.post(t, "1", "/diary", url.Values{"emoji": {"3"}})
	u := a.user(t, "1")
	a.exec(t, "INSERT INTO action_log (user_id, action_date, action, category) VALUES ($1, $2, 'skip', NULL)", u.ID, reports.FromNDaysAgo(2))

	rep := a.get(t, "1", "/reports/weekly").Body
	mustContain(t, rep, "<strong>1 / 7</strong>", "<strong>2</strong>", "<strong>1 дней</strong>")

	r := a.post(t, "1", "/logout", nil)
	c := cookieByName(r.Cookies, "v2_session")
	if r.Location != "/" || c == nil || c.MaxAge >= 0 {
		t.Errorf("logout: %q %v", r.Location, c)
	}
}
