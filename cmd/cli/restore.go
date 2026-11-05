package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/MeheheCedy22/memento-redact/internal/redactor"
)

// The CLI's copy of web/src/restore.ts: the browser restores without the server, so the logic lives in both
// places. Keep the two in sync.

var bracketed = regexp.MustCompile(`^\[[^\]]+\]$`)

// remember adds the entries of a new text to the conversation. A value it already has (same placeholder,
// any case) keeps its first entry, as the server does.
func remember(conv, m []redactor.Entry) []redactor.Entry {
	key := func(e redactor.Entry) string { return e.Placeholder + "\x00" + strings.ToLower(e.Original) }
	seen := map[string]bool{}
	for _, e := range conv {
		seen[key(e)] = true
	}
	for _, e := range m {
		if !seen[key(e)] {
			seen[key(e)] = true
			conv = append(conv, e)
		}
	}
	return conv
}

// restore replaces the placeholders of conv in answer with the values they stand for. Matching ignores case
// and Markdown-escaped brackets (\[HOST_1\]). A placeholder that stands for several values, or a numbered one
// like [HOST_9] that is not in conv, is left as it is, with a note. Other bracketed words ([array]) are text.
func restore(answer string, conv []redactor.Entry) (string, []string) {
	type token struct {
		token  string
		values []string
	}
	known := map[string]*token{} // by lower-case token
	add := func(t, value string) {
		k := known[strings.ToLower(t)]
		if k == nil {
			k = &token{token: t}
			known[strings.ToLower(t)] = k
		}
		if !slices.Contains(k.values, value) {
			k.values = append(k.values, value)
		}
	}
	for _, e := range conv {
		add(e.Token, e.Original)
		// a per-part token like [DOMAIN_1].[TLD_1] for acme.sk also gives [DOMAIN_1] -> acme and [TLD_1] -> sk
		parts, labels := strings.Split(e.Token, "."), strings.Split(e.Original, ".")
		if len(parts) < 2 || len(labels) < len(parts) || slices.ContainsFunc(parts, func(p string) bool { return !bracketed.MatchString(p) }) {
			continue
		}
		// extra leading labels belong to the first part, as in the server's per-part numbering
		extra := len(labels) - len(parts)
		split := append([]string{strings.Join(labels[:extra+1], ".")}, labels[extra+1:]...)
		for i, p := range parts {
			add(p, split[i])
		}
	}

	keys := make([]string, 0, len(known))
	for k := range known {
		keys = append(keys, k)
	}
	// longest first, so [DOMAIN_1].[TLD_1] wins over [DOMAIN_1]
	slices.SortFunc(keys, func(a, b string) int { return len(b) - len(a) })
	escape := strings.NewReplacer(`\[`, `\\?\[`, `\]`, `\\?\]`)
	alts := []string{}
	for _, k := range keys {
		alts = append(alts, escape.Replace(regexp.QuoteMeta(k)))
	}
	alts = append(alts, `\\?\[[A-Z][A-Z0-9_]*_\d+\\?\]`)
	pattern := regexp.MustCompile(`(?i)` + strings.Join(alts, "|"))

	var notes []string
	noted := map[string]bool{}
	note := func(s string) {
		if !noted[s] {
			noted[s] = true
			notes = append(notes, s)
		}
	}
	out := pattern.ReplaceAllStringFunc(answer, func(m string) string {
		k := known[strings.ToLower(strings.ReplaceAll(m, `\`, ""))]
		switch {
		case k != nil && len(k.values) == 1:
			return k.values[0]
		case k != nil:
			note(fmt.Sprintf("not restored: %s stands for several values: %s", m, strings.Join(k.values, ", ")))
		default:
			note(fmt.Sprintf("not restored: %s is not in the conversation", m))
		}
		return m
	})
	return out, notes
}
