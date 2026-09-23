package subscription

import (
	"database/sql"
	"testing"
	"time"
)

func at(t time.Time) sql.NullString {
	return sql.NullString{String: t.UTC().Format(time.RFC3339), Valid: true}
}

func TestIsPremiumActive(t *testing.T) {
	future := at(time.Now().Add(24 * time.Hour))
	past := at(time.Now().Add(-time.Hour))
	// Даты, сохранённые Node-версией (toISOString, с миллисекундами).
	nodeFuture := sql.NullString{String: time.Now().Add(time.Hour).UTC().Format("2006-01-02T15:04:05.000Z"), Valid: true}

	cases := []struct {
		name    string
		tier    string
		expires sql.NullString
		premium bool
		any     bool
	}{
		{"free", "free", sql.NullString{}, false, false},
		{"premium активен", "premium888", future, true, true},
		{"premium истёк", "premium888", past, false, false},
		{"premium без даты", "premium888", sql.NullString{}, false, false},
		{"plus активен", "plus369", future, false, true},
		{"plus истёк", "plus369", past, false, false},
		{"мусор в дате", "premium888", sql.NullString{String: "завтра", Valid: true}, false, false},
		{"дата из Node", "premium888", nodeFuture, true, true},
	}
	for _, c := range cases {
		if got := IsPremiumActive(c.tier, c.expires); got != c.premium {
			t.Errorf("%s: IsPremiumActive=%v, want %v", c.name, got, c.premium)
		}
		if got := IsAnySubscriptionActive(c.tier, c.expires); got != c.any {
			t.Errorf("%s: IsAnySubscriptionActive=%v, want %v", c.name, got, c.any)
		}
	}
}
