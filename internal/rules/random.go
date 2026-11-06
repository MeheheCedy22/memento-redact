package rules

import (
	"math"
	"unicode/utf8"
)

// looksRandom reports whether s looks like a generated secret rather than a word, a name or a number:
//   - at least 16 characters, with letters and digits;
//   - letters, digits and upper/lower case switch often: in at least 30% of neighbouring pairs, separators
//     like - _ . skipped. Generated tokens switch about every other character; AuthenticationService2 or
//     my-service-name-2 hardly ever;
//   - at least 3 bits of Shannon entropy per character, so ab12ab12ab12ab12 is not random.
//
// A generated value of only letters, or only digits, is not recognized.
func looksRandom(s string) bool {
	n := utf8.RuneCountInString(s)
	if n < 16 {
		return false
	}
	counts := map[rune]int{}
	letters, digits, alnum, switches, prev := 0, 0, 0, 0, 0
	for _, c := range s {
		counts[c]++
		class := 0 // 1 lower, 2 upper, 3 digit
		switch {
		case 'a' <= c && c <= 'z':
			class, letters = 1, letters+1
		case 'A' <= c && c <= 'Z':
			class, letters = 2, letters+1
		case '0' <= c && c <= '9':
			class, digits = 3, digits+1
		default:
			continue
		}
		if prev != 0 && class != prev {
			switches++
		}
		prev = class
		alnum++
	}
	if letters == 0 || digits == 0 {
		return false
	}
	entropy := 0.0
	for _, k := range counts {
		p := float64(k) / float64(n)
		entropy -= p * math.Log2(p)
	}
	return float64(switches) >= 0.3*float64(alnum-1) && entropy >= 3
}
