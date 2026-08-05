package ui

import "fmt"

// Plural picks the Russian numeric form for n.
//
//	Plural(1, "компания", "компании", "компаний")  → "1 компания"
//	Plural(3, ...)                                  → "3 компании"
//	Plural(14, ...)                                 → "14 компаний"
func Plural(n int, one, few, many string) string {
	return fmt.Sprintf("%d %s", n, PluralWord(n, one, few, many))
}

// PluralWord returns just the noun form, without the number.
func PluralWord(n int, one, few, many string) string {
	if n < 0 {
		n = -n
	}

	// 11–14 take the "many" form regardless of their last digit.
	if n%100 >= 11 && n%100 <= 14 {
		return many
	}

	switch n % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	default:
		return many
	}
}
