package achievements

import (
	"errors"
	"reflect"
	"testing"

	"version20/internal/models"
)

type fakeChecker struct {
	have      map[string]bool
	inserted  []string
	insertErr error
}

func (f *fakeChecker) HasAchievement(_ int64, code string) (bool, error) {
	return f.have[code], nil
}

func (f *fakeChecker) InsertAchievement(_ int64, code, _ string) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = append(f.inserted, code)
	if f.have == nil {
		f.have = map[string]bool{}
	}
	f.have[code] = true
	return nil
}

func TestCheckAndUnlock(t *testing.T) {
	cases := []struct {
		name   string
		streak int
		level  int
		have   []string
		want   []string
	}{
		{"новичок", 0, 0, nil, nil},
		{"6 дней", 6, 1, nil, nil},
		{"7 дней", 7, 1, nil, []string{"7d"}},
		{"уже открыт", 7, 1, []string{"7d"}, nil},
		{"30 дней + 25 уровень", 30, 25, []string{"7d"}, []string{"30d", "lvl25"}},
		{"всё сразу", 365, 100, nil, []string{"7d", "30d", "180d", "365d", "lvl25", "lvl50", "lvl75", "lvl100"}},
		{"уровень без стрика", 0, 75, nil, []string{"lvl25", "lvl50", "lvl75"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeChecker{have: map[string]bool{}}
			for _, h := range c.have {
				f.have[h] = true
			}
			got, err := CheckAndUnlock(f, &models.User{ID: 1, StreakCurrent: c.streak, Level: c.level})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
			if !reflect.DeepEqual(f.inserted, c.want) {
				t.Errorf("inserted %v, want %v", f.inserted, c.want)
			}
		})
	}
}

func TestCheckAndUnlockPropagatesError(t *testing.T) {
	f := &fakeChecker{insertErr: errors.New("db down")}
	if _, err := CheckAndUnlock(f, &models.User{StreakCurrent: 7}); err == nil {
		t.Fatal("ожидалась ошибка")
	}
}

func TestMetas(t *testing.T) {
	if len(Metas) != 8 {
		t.Fatalf("бейджей %d, want 8", len(Metas))
	}
	for _, m := range Metas {
		if m.Icon == "" || m.Name == "" || m.Desc == "" {
			t.Errorf("неполная мета: %+v", m)
		}
		if MetaByCode(m.Code) != m {
			t.Errorf("MetaByCode(%q) не нашёл", m.Code)
		}
	}
	if MetaByCode("lvl100").Icon != "💎" {
		t.Error("lvl100 должен быть алмазом")
	}
	if got := MetaByCode("unknown"); got.Code != "unknown" || got.Name != "" {
		t.Errorf("неизвестный код: %+v", got)
	}
}
