package rules

import (
	"fmt"
	"regexp/syntax"
	"slices"
	"strings"
	"unicode"
)

// Warning points at a rule that works but probably redacts too much or too little.
type Warning struct {
	Ruleset string `json:"ruleset"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// ordinary is plain prose with nothing to hide: a rule that redacts any of it redacts too much.
const ordinary = "The quick brown fox jumps over the lazy dog. Version 2.1 was released on Monday, and 10 of 12 tests " +
	"passed in 3 seconds. Please call me back if anything is unclear, or write to the team."

// Lint returns warnings for compiled rulesets, switched off ones included: a greedy .* that takes the rest
// of the line, a regex without \b that can start or end inside a longer word, and rules that redact ordinary text.
func Lint(sets []*Ruleset) []Warning {
	var out []Warning
	for _, set := range sets {
		for _, r := range set.Rules {
			add := func(message string) { out = append(out, Warning{set.Name, r.Name, message}) }
			if r.Type == "regex" {
				for i, p := range r.Patterns {
					for _, message := range lintRegex(p) {
						add(fmt.Sprintf("Pattern %d: %s", i+1, message))
					}
				}
			}
			var words []string
			for _, span := range r.Find(ordinary) {
				word := fmt.Sprintf("%q", ordinary[span[0]:span[1]])
				if len(words) < 3 && !slices.ContainsFunc(words, func(w string) bool { return strings.EqualFold(w, word) }) {
					words = append(words, word)
				}
			}
			if words != nil {
				add("Redacts ordinary text, like " + strings.Join(words, ", ") + ".")
			}
		}
	}
	return out
}

func lintRegex(p string) []string {
	re, err := syntax.Parse(p, syntax.Perl)
	if err != nil {
		return nil // Compile reports it
	}
	var out []string
	if greedyDotStar(re) {
		out = append(out, `.* takes the rest of the line, not just the value. Use a class that stops at the value's end, like [^\s"]*.`)
	}
	// Only the redacted part matters: the whole match, or a group at the start or end of the pattern.
	top := []*syntax.Regexp{re}
	if re.Op == syntax.OpConcat {
		top = re.Sub
	}
	grouped := re.MaxCap() > 0
	if first := top[0]; !grouped || first.Op == syntax.OpCapture {
		if wordAt(first, true) {
			out = append(out, `no \b at the start, so it also matches inside a longer word.`)
		}
	}
	if last := top[len(top)-1]; !grouped || last.Op == syntax.OpCapture {
		if wordAt(last, false) {
			out = append(out, `no \b at the end, so it can stop inside a longer word.`)
		}
	}
	return out
}

// greedyDotStar reports a greedy .* or .+ anywhere in re.
func greedyDotStar(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpStar, syntax.OpPlus, syntax.OpRepeat:
		any := re.Sub[0].Op == syntax.OpAnyCharNotNL || re.Sub[0].Op == syntax.OpAnyChar
		if any && re.Flags&syntax.NonGreedy == 0 && (re.Op != syntax.OpRepeat || re.Max == -1) {
			return true
		}
	}
	for _, sub := range re.Sub {
		if greedyDotStar(sub) {
			return true
		}
	}
	return false
}

// wordAt reports whether re can begin (start) or end (!start) with a word character without a \b or anchor
// there. At the end only literals and exact counts like {12} count: a greedy + or * runs to the end of the
// word by itself, and a range like {1,63} is taken as a deliberate length limit.
func wordAt(re *syntax.Regexp, start bool) bool {
	switch re.Op {
	case syntax.OpLiteral:
		c := re.Rune[len(re.Rune)-1]
		if start {
			c = re.Rune[0]
		}
		return isWordRune(c)
	case syntax.OpCharClass:
		for i := 0; i < len(re.Rune); i += 2 {
			lo, hi := re.Rune[i], re.Rune[i+1]
			if lo <= '9' && hi >= '0' || lo <= 'Z' && hi >= 'A' || lo <= 'z' && hi >= 'a' || lo <= '_' && hi >= '_' {
				return true
			}
		}
		return false
	case syntax.OpStar, syntax.OpPlus, syntax.OpRepeat:
		if !start && re.Flags&syntax.NonGreedy == 0 && (re.Op != syntax.OpRepeat || re.Min != re.Max) {
			return false
		}
		// [\w.]+ at the start: a match after a word character would also match one character earlier,
		// so the leftmost match never starts inside a word
		if start && (re.Op != syntax.OpRepeat || re.Max == -1) && re.Sub[0].Op == syntax.OpCharClass && coversWord(re.Sub[0].Rune) {
			return false
		}
		return wordAt(re.Sub[0], start)
	case syntax.OpQuest, syntax.OpCapture:
		return wordAt(re.Sub[0], start)
	case syntax.OpConcat:
		if start {
			return wordAt(re.Sub[0], start)
		}
		return wordAt(re.Sub[len(re.Sub)-1], start)
	case syntax.OpAlternate:
		for _, sub := range re.Sub {
			if wordAt(sub, start) {
				return true
			}
		}
	}
	return false
}

// coversWord reports whether a character class (sorted ranges) holds every ASCII word character.
func coversWord(class []rune) bool {
	for _, want := range [][2]rune{{'0', '9'}, {'A', 'Z'}, {'_', '_'}, {'a', 'z'}} {
		covered := false
		for i := 0; i < len(class); i += 2 {
			covered = covered || class[i] <= want[0] && want[1] <= class[i+1]
		}
		if !covered {
			return false
		}
	}
	return true
}

func isWordRune(c rune) bool {
	return c == '_' || c < unicode.MaxASCII && (unicode.IsLetter(c) || unicode.IsDigit(c))
}
