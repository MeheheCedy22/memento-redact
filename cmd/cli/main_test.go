package main

import (
	"flag"
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

func TestParseTakesFlagsAnywhere(t *testing.T) {
	for _, args := range [][]string{{"--add", "f.yml"}, {"f.yml", "--add"}, {"f.yml", "--add", "extra"}} {
		fs := flag.NewFlagSet("x", flag.ContinueOnError)
		add := fs.Bool("add", false, "")
		if arg := parse(fs, args); arg != "f.yml" || !*add {
			t.Errorf("%q: arg %q, add %v", args, arg, *add)
		}
	}
	if arg := parse(flag.NewFlagSet("x", flag.ContinueOnError), []string{"-"}); arg != "-" {
		t.Errorf("- (stdin) must stay an argument, got %q", arg)
	}
}

func TestProfileRoundTripAndPartialApply(t *testing.T) {
	sets := func() []*rules.Ruleset {
		return []*rules.Ruleset{
			{Name: "Net", Enabled: true, Rules: []*rules.Rule{{Name: "IP", Enabled: true}, {Name: "MAC", Enabled: false}}},
			{Name: "Org", Enabled: true, Rules: []*rules.Rule{{Name: "Names", Enabled: true}}},
		}
	}
	work := sets()
	work[0].Rules[0].Enabled, work[1].Enabled = false, false
	saved := profileOf(work)

	got := sets()
	applyProfile(got, saved)
	if !reflect.DeepEqual(profileOf(got), saved) {
		t.Errorf("applying a profile must give its switches back: %+v", profileOf(got))
	}
	// a profile that names only Net leaves Org as it is
	got = sets()
	applyProfile(got, saved[:1])
	if got[0].Rules[0].Enabled || !got[1].Enabled {
		t.Errorf("partial profile: %+v", profileOf(got))
	}
}

func TestPickKeepsFileOrderAndIgnoresCase(t *testing.T) {
	file := []*rules.Ruleset{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	var names []string
	for _, s := range pick(file, []string{"c", " a"}) {
		names = append(names, s.Name)
	}
	if !reflect.DeepEqual(names, []string{"A", "C"}) {
		t.Errorf("pick: %v", names)
	}
}
