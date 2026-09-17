// Package models содержит общие структуры данных приложения.
package models

// Question — прямой порт backend/lib/questions.js. Психологическая логика:
// по каждому направлению — субъективная самооценка (scale 1-10) +
// поведенческий вопрос (single, варианты от "лучшего" состояния к "худшему",
// используется для подсчёта веса в scheduler). 4 открытых вопроса в конце —
// качественный контекст, на вес не влияют.
type Question struct {
	ID         int      `json:"id"`
	Category   string   `json:"cat"` // пусто у вопроса про пол и у открытых вопросов
	Text       string   `json:"text"`
	Type       string   `json:"type"` // single | scale | text
	Diagnostic bool     `json:"diagnostic"`
	Options    []Option `json:"options,omitempty"`
}

// Option — для вопроса 0 (пол) у вариантов есть отдельные value/label;
// для остальных single-вопросов Options хранит только Label (Value == Label).
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

func opts(labels ...string) []Option {
	out := make([]Option, len(labels))
	for i, l := range labels {
		out[i] = Option{Value: l, Label: l}
	}
	return out
}

var Categories = []string{
	"Внешность", "Стиль", "Тело", "Деньги",
	"Отношения", "Окружение", "Карьера", "Образ жизни",
}

var Questions = []Question{
	{ID: 0, Text: "Ты мужчина или женщина?", Type: "single", Diagnostic: false,
		Options: []Option{{Value: "male", Label: "Мужчина"}, {Value: "female", Label: "Женщина"}}},

	{ID: 1, Category: "Внешность", Text: "Насколько ты доволен(льна) своей внешностью прямо сейчас? (1–10)", Type: "scale", Diagnostic: true},
	{ID: 2, Category: "Внешность", Text: "Как часто ты уделяешь время уходу за собой — кожа, волосы, тело?", Type: "single", Diagnostic: true,
		Options: opts("Каждый день", "Несколько раз в неделю", "Раз в неделю", "Почти никогда")},

	{ID: 3, Category: "Стиль", Text: "Насколько уверенно ты чувствуешь себя в том, как одет(а)? (1–10)", Type: "scale", Diagnostic: true},
	{ID: 4, Category: "Стиль", Text: "Сколько вещей в твоём гардеробе ты реально носишь?", Type: "single", Diagnostic: true,
		Options: opts("Больше 60%", "40–60%", "20–40%", "Меньше 20%")},

	{ID: 5, Category: "Тело", Text: "Оцени уровень своей физической энергии в течение дня (1–10)", Type: "scale", Diagnostic: true},
	{ID: 6, Category: "Тело", Text: "Как часто ты двигаешься или занимаешься спортом?", Type: "single", Diagnostic: true,
		Options: opts("5+ раз в неделю", "3–4 раза в неделю", "1–2 раза в неделю", "Почти никогда")},

	{ID: 7, Category: "Деньги", Text: "Насколько спокойно тебе живётся с текущим отношением к деньгам? (1–10)", Type: "scale", Diagnostic: true},
	{ID: 8, Category: "Деньги", Text: "Какой процент дохода ты откладываешь?", Type: "single", Diagnostic: true,
		Options: opts("Больше 20%", "10–20%", "Меньше 10%", "Ничего")},

	{ID: 9, Category: "Отношения", Text: "Насколько ты удовлетворён(а) близкими отношениями сейчас? (1–10)", Type: "scale", Diagnostic: true},
	{ID: 10, Category: "Отношения", Text: "Как часто ты сам(а) делаешь первый шаг в общении с близкими людьми?", Type: "single", Diagnostic: true,
		Options: opts("Почти всегда", "Часто", "Иногда", "Почти никогда")},

	{ID: 11, Category: "Окружение", Text: "Насколько твоё окружение сейчас вдохновляет тебя расти? (1–10)", Type: "scale", Diagnostic: true},
	{ID: 12, Category: "Окружение", Text: "Готов(а) ли ты менять окружение ради своего роста?", Type: "single", Diagnostic: true,
		Options: opts("Определённо да", "Скорее да", "Скорее нет", "Нет")},

	{ID: 13, Category: "Карьера", Text: "Насколько ты доволен(льна) своей работой или учёбой сейчас? (1–10)", Type: "scale", Diagnostic: true},
	{ID: 14, Category: "Карьера", Text: "Есть ли у тебя план развития на 1–3 года?", Type: "single", Diagnostic: true,
		Options: opts("Да, детальный", "Есть общее направление", "Пока нет", "Не думал(а) об этом")},

	{ID: 15, Category: "Образ жизни", Text: "Оцени качество своего сна и режима дня (1–10)", Type: "scale", Diagnostic: true},
	{ID: 16, Category: "Образ жизни", Text: "Как часто ты пробуешь что-то новое, выходишь из привычной рутины?", Type: "single", Diagnostic: true,
		Options: opts("Каждую неделю", "1–2 раза в месяц", "Редко", "Почти никогда")},

	{ID: 17, Text: "Если бы через 365 дней тебя было не узнать — что изменилось бы в первую очередь?", Type: "text", Diagnostic: false},
	{ID: 18, Text: "Какую свою привычку ты давно хочешь изменить, но всё никак не получается?", Type: "text", Diagnostic: false},
	{ID: 19, Text: "Кто или что сейчас больше всего мешает тебе стать лучшей версией себя?", Type: "text", Diagnostic: false},
	{ID: 20, Text: "Одно слово, которое описывает твою идеальную Version 2.0", Type: "text", Diagnostic: false},
}

func QuestionByID(id int) (Question, bool) {
	for _, q := range Questions {
		if q.ID == id {
			return q, true
		}
	}
	return Question{}, false
}
