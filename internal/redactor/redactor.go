package redactor

import (
	"regexp"

	"github.com/marek/memento-redact/internal/rules"
)

// Diff represents a single replaced span.
type Diff struct {
	Original    string `json:"original"`
	Replacement string `json:"replacement"`
}

// Result is the output of a redaction operation.
type Result struct {
	Original string `json:"original"`
	Redacted string `json:"redacted"`
	Diffs    []Diff `json:"diffs"`
}

var (
	emailRe = regexp.MustCompile(`([a-zA-Z0-9._%%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)
	phoneRe = regexp.MustCompile(`\b(\+?\d[\d\-(). ]{6,}\d)\b`)
)

// Redact applies deterministic redaction rules to the input and returns
// a Result containing the redacted text and diffs that show what changed.
func Redact(s string, ruleSet []*rules.Rule) Result {
	red := s

	// simple deterministic masks
	red = emailRe.ReplaceAllString(red, "[REDACTED_EMAIL]")
	red = phoneRe.ReplaceAllString(red, "[REDACTED_PHONE]")
	for _, rule := range ruleSet {
		red = rule.Apply(red)
	}

	diffs := computeDiffs(s, red)
	return Result{Original: s, Redacted: red, Diffs: diffs}
}

// computeDiffs produces simple contiguous diffs between original and redacted strings.
func computeDiffs(orig, red string) []Diff {
	var diffs []Diff
	i, j := 0, 0
	for i < len(orig) && j < len(red) {
		if orig[i] == red[j] {
			i++
			j++
			continue
		}
		// start of a difference
		si, sj := i, j
		for i < len(orig) && j < len(red) && orig[i] != red[j] {
			i++
			j++
		}
		diffs = append(diffs, Diff{Original: orig[si:i], Replacement: red[sj:j]})
	}
	// trailing tails
	if i < len(orig) || j < len(red) {
		diffs = append(diffs, Diff{Original: orig[i:], Replacement: red[j:]})
	}
	return diffs
}
