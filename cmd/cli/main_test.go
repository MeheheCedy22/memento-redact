package main

import (
	"reflect"
	"testing"

	"github.com/MeheheCedy22/memento-redact/internal/redactor"
	"github.com/MeheheCedy22/memento-redact/internal/rules"
)

// The restore cases mirror web/src/restore.test.ts.

func entry(token, original string) redactor.Entry {
	return redactor.Entry{Token: token, Original: original, Ruleset: "s", Rule: "r", Count: 1, Placeholder: "[X_#]"}
}

var conv = []redactor.Entry{
	entry("[HOST_1]", "web01"),
	entry("[DOMAIN_1].[TLD_1]", "acme.sk"),
	entry("[DOMAIN_1].[TLD_2]", "acme.com"),
	entry("[DOMAIN_2].[TLD_1]", "corp.contoso.sk"),
	entry("[DOMAIN]", "a.tld"), // a fixed placeholder: one token, several values
	entry("[DOMAIN]", "b.tld"),
}

func TestRestore(t *testing.T) {
	cases := []struct{ answer, want string }{
		// whole tokens, tokens next to each other, other case and escaped brackets
		{`Restart [HOST_1].[DOMAIN_1].[TLD_1] and \[host_1\], then [domain_1].[tld_2].`, "Restart web01.acme.sk and web01, then acme.com."},
		// the parts of a per-part token used alone, with extra labels in the first part
		{"[DOMAIN_1] is on [TLD_2]; [DOMAIN_2] too", "acme is on com; corp.contoso too"},
		// bracketed words that are not numbered placeholders are text
		{"$i = [array]::IndexOf($n, 1)\n[INFO] done, [ ] todo, [x] done", "$i = [array]::IndexOf($n, 1)\n[INFO] done, [ ] todo, [x] done"},
		{"nothing here", "nothing here"},
	}
	for _, c := range cases {
		if got, notes := restore(c.answer, conv); got != c.want || len(notes) != 0 {
			t.Errorf("restore(%q) = %q %q, want %q", c.answer, got, notes, c.want)
		}
	}
}

func TestRestoreLeavesWhatItCannotRestoreAndSaysWhy(t *testing.T) {
	got, notes := restore("[DOMAIN] and [HOST_9] and [host_9] and [TLD_1]", conv)
	if got != "[DOMAIN] and [HOST_9] and [host_9] and sk" {
		t.Errorf("text: %q", got)
	}
	want := []string{
		"not restored: [DOMAIN] stands for several values: a.tld, b.tld",
		"not restored: [HOST_9] is not in the conversation",
		"not restored: [host_9] is not in the conversation",
	}
	if !reflect.DeepEqual(notes, want) {
		t.Errorf("notes: %q", notes)
	}
	if _, notes := restore("[X_1] stays", nil); len(notes) != 1 {
		t.Errorf("an empty conversation must report a numbered placeholder: %q", notes)
	}
}

func TestRememberAddsOnlyNewValues(t *testing.T) {
	first := []redactor.Entry{entry("[HOST_1]", "web01")}
	got := remember(first, []redactor.Entry{entry("[HOST_1]", "WEB01"), entry("[HOST_2]", "db01")})
	if len(got) != 2 || got[0].Original != "web01" || got[1].Original != "db01" {
		t.Errorf("remember: %+v", got)
	}
}

func TestMergeReplacesByNameAndAppendsNew(t *testing.T) {
	set := func(name, desc string) *rules.Ruleset { return &rules.Ruleset{Name: name, Description: desc} }
	current := []*rules.Ruleset{set("A", "old"), set("B", "old")}
	got := merge(current, []*rules.Ruleset{set("B", "new"), set("C", "new")})
	var names []string
	for _, s := range got {
		names = append(names, s.Name+"="+s.Description)
	}
	if !reflect.DeepEqual(names, []string{"A=old", "B=new", "C=new"}) {
		t.Errorf("merge: %v", names)
	}
	if current[1].Description != "old" {
		t.Error("merge must not change the current list")
	}
}
