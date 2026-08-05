package ui

import "testing"

func TestPluralWord(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "компаний"},
		{1, "компания"},
		{2, "компании"},
		{4, "компании"},
		{5, "компаний"},
		{11, "компаний"},
		{12, "компаний"},
		{14, "компаний"},
		{21, "компания"},
		{22, "компании"},
		{25, "компаний"},
		{101, "компания"},
		{111, "компаний"},
	}

	for _, tc := range cases {
		if got := PluralWord(tc.n, "компания", "компании", "компаний"); got != tc.want {
			t.Errorf("PluralWord(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestPluralIncludesNumber(t *testing.T) {
	if got := Plural(3, "репозиторий", "репозитория", "репозиториев"); got != "3 репозитория" {
		t.Errorf("Plural = %q", got)
	}
}

func TestTruncateKeepsTail(t *testing.T) {
	got := Truncate("/very/long/path/to/a/file.md", 12)
	if len([]rune(got)) > 12 {
		t.Errorf("Truncate returned %q, longer than 12", got)
	}
	if got[len(got)-7:] != "file.md" {
		t.Errorf("Truncate = %q, should keep the tail", got)
	}
}

func TestTruncateLeavesShortText(t *testing.T) {
	if got := Truncate("short", 40); got != "short" {
		t.Errorf("Truncate = %q, want unchanged", got)
	}
}
