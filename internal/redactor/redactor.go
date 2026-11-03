package redactor

import (
	"sort"
	"strconv"
	"strings"

	"github.com/MeheheCedy22/memento-redact/internal/rules"
)

// Entry maps one placeholder token back to the distinct original value it replaced.
type Entry struct {
	Token    string `json:"token"`
	Original string `json:"original"`
	Ruleset  string `json:"ruleset"`
	Rule     string `json:"rule"`
	Count    int    `json:"count"`
}

// Result is the output of a redaction operation.
type Result struct {
	Redacted string  `json:"redacted"`
	Map      []Entry `json:"map"`
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
// Tokens are numbered per placeholder in order of appearance; equal values share a token.
func Redact(text string, sets []*rules.Ruleset) Result {
	var spans []span
	for _, set := range sets {
		if !set.Enabled {
			continue
		}
		for _, r := range set.Rules {
			if !r.Enabled {
				continue
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
	var b strings.Builder
	last := 0
	for _, s := range kept {
		original := text[s.start:s.end]
		key := [2]string{s.ph, original}
		i, ok := index[key]
		if !ok {
			token := ""
			if s.perPart {
				token = parts.token(s.ph, original)
			}
			if token == "" {
				counters[s.ph]++
				token = strings.ReplaceAll(s.ph, "#", strconv.Itoa(counters[s.ph]))
			}
			i = len(res.Map)
			index[key] = i
			res.Map = append(res.Map, Entry{Token: token, Original: original, Ruleset: s.set, Rule: s.rule})
		}
		res.Map[i].Count++
		b.WriteString(text[last:s.start])
		b.WriteString(res.Map[i].Token)
		last = s.end
	}
	b.WriteString(text[last:])
	res.Redacted = b.String()
	return res
}

// partNumbering numbers placeholder parts by their own value, case-insensitively.
type partNumbering struct {
	next map[string]int    // placeholder part -> last number given
	of   map[[2]string]int // (placeholder part, value part) -> number
}

// token fills each dot-separated part of ph with the number of the matching part of original:
// [DOMAIN].[TLD] gives acme.sk -> [DOMAIN_1].[TLD_1], acme.com -> [DOMAIN_1].[TLD_2].
// Extra leading labels belong to the first part (corp.acme.sk -> DOMAIN "corp.acme").
// It returns "" when original has fewer parts than ph.
func (p *partNumbering) token(ph, original string) string {
	tmpl := strings.Split(ph, ".")
	vals := strings.Split(original, ".")
	if len(tmpl) < 2 || len(vals) < len(tmpl) {
		return ""
	}
	extra := len(vals) - len(tmpl)
	vals = append([]string{strings.Join(vals[:extra+1], ".")}, vals[extra+1:]...)
	for i, part := range tmpl {
		key := [2]string{part, strings.ToLower(vals[i])}
		n, ok := p.of[key]
		if !ok {
			p.next[part]++
			n = p.next[part]
			p.of[key] = n
		}
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
	return strings.Join(tmpl, ".")
}

func free(taken []bool, start, end int) bool {
	for i := start; i < end; i++ {
		if taken[i] {
			return false
		}
	}
	return true
}
