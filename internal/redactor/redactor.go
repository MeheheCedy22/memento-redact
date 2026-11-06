package redactor

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/MeheheCedy22/memento-redact/internal/rules"
)

// Entry maps one placeholder token back to the distinct original value it replaced.
type Entry struct {
	Token    string `json:"token"`
	Original string `json:"original"`
	Ruleset  string `json:"ruleset"`
	Rule     string `json:"rule"`
	Count    int    `json:"count"`
	// Placeholder is the rule's placeholder the token was made from; Numbers are the numbers put into it,
	// one per part with per_part. Passing entries back as known continues the numbering from them.
	Placeholder string `json:"placeholder"`
	Numbers     []int  `json:"numbers,omitempty"`
}

// Result is the output of a redaction operation.
type Result struct {
	Redacted string  `json:"redacted"`
	Map      []Entry `json:"map"`
	// Leaks are values that are redacted somewhere but still appear in the text as they are.
	Leaks []Leak `json:"leaks,omitempty"`
}

// Leak is a value from the map, an earlier text or a words list that still appears in the redacted text,
// for example a password redacted after "password=" and repeated without it, or a name inside a longer
// word. Count is how often it appears; Ruleset and Rule name where it is redacted.
type Leak struct {
	Value   string `json:"value"`
	Ruleset string `json:"ruleset"`
	Rule    string `json:"rule"`
	Count   int    `json:"count"`
}

type span struct {
	start, end int
	order      int
	set, rule  string
	ph         string
	perPart    bool
}

// Redact replaces every match of the enabled rules with placeholder tokens.
// All rules run against the original text, so no rule ever matches another rule's token.
// Overlapping matches: the longest wins, ties go to the earlier rule.
// Tokens are numbered per placeholder in order of appearance; values equal ignoring case share a token.
// known holds the map entries of earlier texts in the same conversation: their values keep their tokens,
// and new values are numbered after them.
// It stops with ctx's error when ctx ends, so an abandoned preview does not keep running.
func Redact(ctx context.Context, text string, sets []*rules.Ruleset, known []Entry) (Result, error) {
	var spans []span
	for _, set := range sets {
		if !set.Enabled {
			continue
		}
		for _, r := range set.Rules {
			if !r.Enabled {
				continue
			}
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}
			for _, m := range r.Find(text) {
				spans = append(spans, span{start: m[0], end: m[1], order: len(spans), set: set.Name, rule: r.Name, ph: r.Placeholder, perPart: r.PerPart})
			}
		}
	}

	sort.Slice(spans, func(a, b int) bool {
		la, lb := spans[a].end-spans[a].start, spans[b].end-spans[b].start
		if la != lb {
			return la > lb
		}
		return spans[a].order < spans[b].order
	})
	taken := make([]bool, len(text))
	kept := spans[:0]
	for _, s := range spans {
		if !free(taken, s.start, s.end) {
			continue
		}
		for i := s.start; i < s.end; i++ {
			taken[i] = true
		}
		kept = append(kept, s)
	}
	sort.Slice(kept, func(a, b int) bool { return kept[a].start < kept[b].start })

	res := Result{Map: []Entry{}}
	index := map[[2]string]int{}
	counters := map[string]int{}
	parts := partNumbering{next: map[string]int{}, of: map[[2]string]int{}}
	earlier := map[[2]string]Entry{}
	for _, e := range known {
		if e.Token == "" || e.Token == e.Original {
			continue // never let a known entry put a value back into the text
		}
		earlier[[2]string{e.Placeholder, strings.ToLower(e.Original)}] = e
		if len(e.Numbers) > 1 {
			parts.remember(e.Placeholder, e.Original, e.Numbers)
		} else if len(e.Numbers) == 1 {
			counters[e.Placeholder] = max(counters[e.Placeholder], e.Numbers[0])
		}
	}
	var b strings.Builder
	var plain []string // the parts of the text left as they are
	last := 0
	for _, s := range kept {
		original := text[s.start:s.end]
		key := [2]string{s.ph, strings.ToLower(original)}
		i, ok := index[key]
		if !ok {
			e, seen := earlier[key]
			if !seen {
				e = Entry{Placeholder: s.ph}
				if s.perPart {
					e.Token, e.Numbers = parts.token(s.ph, original)
				}
				if e.Token == "" {
					counters[s.ph]++
					e.Token = strings.ReplaceAll(s.ph, "#", strconv.Itoa(counters[s.ph]))
					if strings.Contains(s.ph, "#") {
						e.Numbers = []int{counters[s.ph]}
					}
				}
			}
			e.Original, e.Ruleset, e.Rule, e.Count = original, s.set, s.rule, 0
			i = len(res.Map)
			index[key] = i
			res.Map = append(res.Map, e)
		}
		res.Map[i].Count++
		b.WriteString(text[last:s.start])
		plain = append(plain, text[last:s.start])
		b.WriteString(res.Map[i].Token)
		last = s.end
	}
	b.WriteString(text[last:])
	plain = append(plain, text[last:])
	res.Redacted = b.String()
	res.Leaks = leaks(plain, sets, res.Map, known)
	return res, nil
}

// leaks finds the values redacted somewhere (this text's map, earlier texts, the words of enabled words
// rules) that still occur, ignoring case, in the plain parts of the text. Values under 3 characters are
// skipped: a captured "1" or "on" occurs everywhere by chance.
func leaks(plain []string, sets []*rules.Ruleset, entries ...[]Entry) []Leak {
	for i := range plain {
		plain[i] = strings.ToLower(plain[i])
	}
	var out []Leak
	seen := map[string]bool{}
	check := func(value, set, rule string) {
		v := strings.ToLower(value)
		if utf8.RuneCountInString(strings.TrimSpace(v)) < 3 || seen[v] {
			return
		}
		seen[v] = true
		n := 0
		for _, p := range plain {
			n += strings.Count(p, v)
		}
		if n > 0 {
			out = append(out, Leak{Value: value, Ruleset: set, Rule: rule, Count: n})
		}
	}
	for _, list := range entries {
		for _, e := range list {
			check(e.Original, e.Ruleset, e.Rule)
		}
	}
	for _, set := range sets {
		for _, r := range set.Rules {
			if set.Enabled && r.Enabled && r.Type == "words" {
				for _, w := range r.Patterns {
					check(w, set.Name, r.Name)
				}
			}
		}
	}
	return out
}

// partNumbering numbers placeholder parts by their own value, case-insensitively.
type partNumbering struct {
	next map[string]int    // placeholder part -> last number given
	of   map[[2]string]int // (placeholder part, value part) -> number
}

// split cuts ph and original at the dots into matching parts; extra leading labels of original belong
// to the first part (corp.acme.sk for [DOMAIN].[TLD] -> "corp.acme", "sk"). It returns nil when ph has
// no dots or original has fewer parts than ph.
func split(ph, original string) (tmpl, vals []string) {
	tmpl = strings.Split(ph, ".")
	vals = strings.Split(original, ".")
	if len(tmpl) < 2 || len(vals) < len(tmpl) {
		return nil, nil
	}
	extra := len(vals) - len(tmpl)
	return tmpl, append([]string{strings.Join(vals[:extra+1], ".")}, vals[extra+1:]...)
}

// token fills each dot-separated part of ph with the number of the matching part of original, and returns
// those numbers: [DOMAIN].[TLD] gives acme.sk -> [DOMAIN_1].[TLD_1], acme.com -> [DOMAIN_1].[TLD_2].
// It returns "" when split does.
func (p *partNumbering) token(ph, original string) (string, []int) {
	tmpl, vals := split(ph, original)
	if tmpl == nil {
		return "", nil
	}
	nums := make([]int, len(tmpl))
	for i, part := range tmpl {
		key := [2]string{part, strings.ToLower(vals[i])}
		n, ok := p.of[key]
		if !ok {
			p.next[part]++
			n = p.next[part]
			p.of[key] = n
		}
		nums[i] = n
		num := strconv.Itoa(n)
		switch {
		case strings.Contains(part, "#"):
			tmpl[i] = strings.ReplaceAll(part, "#", num)
		case strings.HasSuffix(part, "]"):
			tmpl[i] = part[:len(part)-1] + "_" + num + "]"
		default:
			tmpl[i] = part + "_" + num
		}
	}
	return strings.Join(tmpl, "."), nums
}

// remember takes the part numbers of a token from an earlier text, so they are reused and continued.
func (p *partNumbering) remember(ph, original string, nums []int) {
	tmpl, vals := split(ph, original)
	if len(tmpl) != len(nums) {
		return
	}
	for i, part := range tmpl {
		p.of[[2]string{part, strings.ToLower(vals[i])}] = nums[i]
		p.next[part] = max(p.next[part], nums[i])
	}
}

func free(taken []bool, start, end int) bool {
	for i := start; i < end; i++ {
		if taken[i] {
			return false
		}
	}
	return true
}
