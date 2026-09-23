package codes

import (
	"regexp"
	"testing"
)

var codeRe = regexp.MustCompile(`^[A-Z0-9_-]+$`)

func TestGenerate(t *testing.T) {
	for _, n := range []int{1, 7, 8, 16} {
		c := Generate(n)
		if len(c) != n {
			t.Errorf("Generate(%d) = %q, длина %d", n, c, len(c))
		}
		if !codeRe.MatchString(c) {
			t.Errorf("Generate(%d) = %q: недопустимые символы", n, c)
		}
	}
}

func TestGenerateIsRandom(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		seen[Generate(8)] = true
	}
	if len(seen) < 990 {
		t.Errorf("слишком много совпадений: %d уникальных из 1000", len(seen))
	}
}
