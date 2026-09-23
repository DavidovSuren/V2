package models

import "testing"

func TestQuestionnaireShape(t *testing.T) {
	if len(Questions) != 21 {
		t.Fatalf("вопросов %d, want 21 (пол + 16 диагностических + 4 открытых)", len(Questions))
	}
	for i, q := range Questions {
		if q.ID != i {
			t.Errorf("Questions[%d].ID = %d — handlers.quiz индексирует по позиции", i, q.ID)
		}
		if got, ok := QuestionByID(q.ID); !ok || got.ID != q.ID {
			t.Errorf("QuestionByID(%d) не нашёл", q.ID)
		}
	}
	if _, ok := QuestionByID(999); ok {
		t.Error("QuestionByID(999) должен вернуть false")
	}

	gender := Questions[0]
	if gender.Type != "single" || gender.Diagnostic || len(gender.Options) != 2 ||
		gender.Options[0].Value != "male" || gender.Options[1].Value != "female" {
		t.Errorf("вопрос о поле: %+v", gender)
	}
}

func TestEachCategoryHasScaleAndBehaviourQuestion(t *testing.T) {
	if len(Categories) != 8 {
		t.Fatalf("направлений %d, want 8", len(Categories))
	}
	for _, cat := range Categories {
		var scale, single int
		for _, q := range Questions {
			if q.Category != cat {
				continue
			}
			if !q.Diagnostic {
				t.Errorf("%s: вопрос %d не диагностический", cat, q.ID)
			}
			switch q.Type {
			case "scale":
				scale++
			case "single":
				single++
				if len(q.Options) != 4 {
					t.Errorf("вопрос %d: вариантов %d, want 4", q.ID, len(q.Options))
				}
				for _, o := range q.Options {
					if o.Value != o.Label {
						t.Errorf("вопрос %d: value %q != label %q", q.ID, o.Value, o.Label)
					}
				}
			}
		}
		if scale != 1 || single != 1 {
			t.Errorf("%s: scale=%d single=%d, want 1/1", cat, scale, single)
		}
	}

	var text int
	for _, q := range Questions {
		if q.Type == "text" {
			text++
			if q.Diagnostic || q.Category != "" {
				t.Errorf("открытый вопрос %d не должен влиять на вес", q.ID)
			}
		}
	}
	if text != 4 {
		t.Errorf("открытых вопросов %d, want 4", text)
	}
}
