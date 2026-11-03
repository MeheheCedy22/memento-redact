package rules

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Ruleset is a named bundle of rules that can be switched on or off as a whole.
type Ruleset struct {
	Name        string  `yaml:"name" json:"name"`
	Description string  `yaml:"description,omitempty" json:"description,omitempty"`
	Enabled     bool    `yaml:"enabled" json:"enabled"`
	Rules       []*Rule `yaml:"rules" json:"rules"`
}

// Rule matches sensitive values either by literal words/phrases or by regular expressions.
type Rule struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	// Type is "words" (literal, case-insensitive, whole words) or "regex" (Go RE2 syntax).
	Type     string   `yaml:"type" json:"type"`
	Patterns []string `yaml:"patterns" json:"patterns"`
	// Keep is an optional regex; matched values that also match Keep are left untouched.
	Keep string `yaml:"keep,omitempty" json:"keep,omitempty"`
	// Placeholder replaces each value; "#" becomes a number that is unique per distinct value.
	Placeholder string `yaml:"placeholder" json:"placeholder"`

	re   *regexp.Regexp
	keep *regexp.Regexp
}

// DefaultPlaceholder is used when a rule does not define its own placeholder.
const DefaultPlaceholder = "[REDACTED_#]"

// Compile validates and normalizes rulesets and prepares their matchers.
// It must be called before Find; Load and Parse call it for you.
func Compile(sets []*Ruleset) error {
	for i, set := range sets {
		if set == nil || strings.TrimSpace(set.Name) == "" {
			return fmt.Errorf("ruleset #%d: name is required", i+1)
		}
		if set.Rules == nil {
			set.Rules = []*Rule{}
		}
		for j, r := range set.Rules {
			if r == nil || strings.TrimSpace(r.Name) == "" {
				return fmt.Errorf("ruleset %q, rule #%d: name is required", set.Name, j+1)
			}
			if err := r.compile(); err != nil {
				return fmt.Errorf("ruleset %q, rule %q: %w", set.Name, r.Name, err)
			}
		}
	}
	return nil
}

func (r *Rule) compile() error {
	if r.Placeholder == "" {
		r.Placeholder = DefaultPlaceholder
	}
	patterns := []string{}
	for _, p := range r.Patterns {
		if r.Type == "words" {
			p = strings.TrimSpace(p)
		}
		if p != "" {
			patterns = append(patterns, p)
		}
	}
	r.Patterns = patterns

	var alts []string
	switch r.Type {
	case "words":
		sorted := append([]string(nil), patterns...)
		// Longest first, so "acme corp" wins over "acme" at the same position.
		sort.SliceStable(sorted, func(a, b int) bool { return len(sorted[a]) > len(sorted[b]) })
		for _, w := range sorted {
			alts = append(alts, wordPattern(w))
		}
	case "regex":
		for _, p := range patterns {
			if _, err := regexp.Compile(p); err != nil {
				return fmt.Errorf("invalid regex %q: %w", p, err)
			}
			alts = append(alts, "(?:"+p+")")
		}
	default:
		return fmt.Errorf("type must be \"words\" or \"regex\", got %q", r.Type)
	}

	r.re = nil
	if len(alts) > 0 {
		expr := strings.Join(alts, "|")
		if r.Type == "words" {
			expr = "(?i)" + expr
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			return err
		}
		r.re = re
	}
	r.keep = nil
	if r.Keep != "" {
		keep, err := regexp.Compile(r.Keep)
		if err != nil {
			return fmt.Errorf("invalid keep regex %q: %w", r.Keep, err)
		}
		r.keep = keep
	}
	return nil
}

// wordPattern quotes a literal and adds word boundaries on sides that start or end with a word character.
// Go's \b is ASCII-only, so words starting or ending with e.g. "Č" match without a boundary on that side.
func wordPattern(w string) string {
	q := regexp.QuoteMeta(w)
	if isWordChar(w[0]) {
		q = `\b` + q
	}
	if isWordChar(w[len(w)-1]) {
		q += `\b`
	}
	return q
}

func isWordChar(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// Find returns the byte ranges of text this rule redacts. If a regex has capture groups,
// only the first group that participated is redacted, so "password=(\S+)" keeps "password=".
func (r *Rule) Find(text string) [][2]int {
	if r.re == nil {
		return nil
	}
	var out [][2]int
	for _, m := range r.re.FindAllStringSubmatchIndex(text, -1) {
		start, end := m[0], m[1]
		for g := 2; g < len(m); g += 2 {
			if m[g] >= 0 {
				start, end = m[g], m[g+1]
				break
			}
		}
		if start == end || r.keep != nil && r.keep.MatchString(text[start:end]) {
			continue
		}
		out = append(out, [2]int{start, end})
	}
	return out
}

// Parse decodes and compiles rulesets. YAML is a superset of JSON, so both formats are accepted.
func Parse(data []byte) ([]*Ruleset, error) {
	sets := []*Ruleset{}
	if err := yaml.Unmarshal(data, &sets); err != nil {
		return nil, err
	}
	if err := Compile(sets); err != nil {
		return nil, err
	}
	return sets, nil
}

// Load reads and compiles rulesets from a YAML file.
func Load(path string) ([]*Ruleset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Marshal encodes rulesets as YAML.
func Marshal(sets []*Ruleset) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(sets); err != nil {
		return nil, err
	}
	return buf.Bytes(), enc.Close()
}

// Save writes rulesets to a YAML file.
func Save(path string, sets []*Ruleset) error {
	data, err := Marshal(sets)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data)
}

// EnsureSeeded copies the default rulesets file to path if path does not exist yet.
func EnsureSeeded(path, defaultPath string) error {
	if _, err := os.Stat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := os.ReadFile(defaultPath)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data)
}

// writeFileAtomic writes via a temp file and rename so a crash never leaves a half-written rules file.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".rulesets-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
