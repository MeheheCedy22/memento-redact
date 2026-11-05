package redactor

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MeheheCedy22/memento-redact/internal/rules"
)

func sets(t *testing.T, sets ...*rules.Ruleset) []*rules.Ruleset {
	t.Helper()
	if err := rules.Compile(sets); err != nil {
		t.Fatalf("compile: %v", err)
	}
	return sets
}

func redact(t *testing.T, text string, sets []*rules.Ruleset) Result {
	t.Helper()
	res, err := Redact(context.Background(), text, sets, nil)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	return res
}

func TestRedactNumbersDistinctValuesAndReusesTokens(t *testing.T) {
	rs := sets(t, &rules.Ruleset{Name: "s", Enabled: true, Rules: []*rules.Rule{
		{Name: "email", Enabled: true, Type: "regex", Patterns: []string{`\S+@\S+\.com`}, Placeholder: "[EMAIL_#]"},
		{Name: "off", Enabled: false, Type: "words", Patterns: []string{"mail"}},
	}})
	res := redact(t, "mail a@x.com, b@x.com, again A@X.com", rs)
	if want := "mail [EMAIL_1], [EMAIL_2], again [EMAIL_1]"; res.Redacted != want {
		t.Fatalf("got %q, want %q", res.Redacted, want)
	}
	if want := (Entry{Token: "[EMAIL_1]", Original: "a@x.com", Ruleset: "s", Rule: "email", Count: 2, Placeholder: "[EMAIL_#]", Numbers: []int{1}}); len(res.Map) != 2 || !reflect.DeepEqual(res.Map[0], want) {
		t.Fatalf("unexpected map: %+v", res.Map)
	}
}

func TestRedactLongestOverlapWinsAndDisabledSetsAreSkipped(t *testing.T) {
	rs := sets(t,
		&rules.Ruleset{Name: "org", Enabled: true, Rules: []*rules.Rule{
			{Name: "domain", Enabled: true, Type: "words", Patterns: []string{"acme.com"}, Placeholder: "[DOMAIN]"},
		}},
		&rules.Ruleset{Name: "pii", Enabled: true, Rules: []*rules.Rule{
			{Name: "email", Enabled: true, Type: "regex", Patterns: []string{`\w+@[\w.]+`}, Placeholder: "[EMAIL_#]"},
		}},
		&rules.Ruleset{Name: "off", Enabled: false, Rules: []*rules.Rule{
			{Name: "all", Enabled: true, Type: "regex", Patterns: []string{`.+`}},
		}},
	)
	res := redact(t, "bob@acme.com via acme.com, ACME.COM", rs)
	if want := "[EMAIL_1] via [DOMAIN], [DOMAIN]"; res.Redacted != want {
		t.Fatalf("got %q, want %q", res.Redacted, want)
	}
	if len(res.Map) != 2 || res.Map[1].Original != "acme.com" || res.Map[1].Count != 2 {
		t.Fatalf("values equal ignoring case share one entry, spelled as first seen: %+v", res.Map)
	}
}

func TestRedactPerPartNumbersDomainAndTLDSeparately(t *testing.T) {
	rule := &rules.Rule{Name: "domains", Enabled: true, Type: "words", Patterns: []string{"acme.sk", "contoso.com", "acme.com", "corp.acme.sk"}, Placeholder: "[DOMAIN].[TLD]", PerPart: true}
	rs := sets(t, &rules.Ruleset{Name: "org", Enabled: true, Rules: []*rules.Rule{rule}})
	res := redact(t, "acme.sk contoso.com acme.com ACME.SK corp.acme.sk", rs)
	if want := "[DOMAIN_1].[TLD_1] [DOMAIN_2].[TLD_2] [DOMAIN_1].[TLD_2] [DOMAIN_1].[TLD_1] [DOMAIN_3].[TLD_1]"; res.Redacted != want {
		t.Fatalf("got %q, want %q", res.Redacted, want)
	}

	rule.Placeholder = "[D#].[T#]"
	if got := redact(t, "contoso.com acme.sk", rs).Redacted; got != "[D1].[T1] [D2].[T2]" {
		t.Fatalf("# inside parts: %q", got)
	}

	rule.PerPart = false
	rule.Placeholder = "[DOMAIN].[TLD]"
	if got := redact(t, "acme.sk contoso.com", rs).Redacted; got != "[DOMAIN].[TLD] [DOMAIN].[TLD]" {
		t.Fatalf("switched off: one fixed placeholder expected, got %q", got)
	}
}

func TestRedactEmptyInputAndNoRules(t *testing.T) {
	if res := redact(t, "", nil); res.Redacted != "" || res.Map == nil || len(res.Map) != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res := redact(t, "plain žluťoučký text", nil); res.Redacted != "plain žluťoučký text" {
		t.Fatalf("text without rules must pass through: %q", res.Redacted)
	}
}

func TestRedactPowerCLIOutputWithDefaultPresets(t *testing.T) {
	rs, err := rules.Load(filepath.Join("..", "..", "config", "default-rulesets.yml"))
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	in := `Name                   PowerState IP
----                   ---------- --
web01.prod.domain.tld  PoweredOn  10.20.30.41, fe80::250:56ff:fe9a:1b2c
edge-gw.domain.tld     PoweredOn  192.168.1.1, 203.0.113.17
PS> Connect-VIServer vcenter.domain.tld -User admin@domain.tld -Password 'S3cr3t!'
Contact jane.doe@domain.tld or +420 777 123 456. Again: web01.prod.domain.tld 203.0.113.17`
	want := `Name                   PowerState IP
----                   ---------- --
[HOST_1].[DOMAIN_1].[TLD_1]  PoweredOn  10.20.30.41, fe80::250:56ff:fe9a:1b2c
[HOST_2].[DOMAIN_1].[TLD_1]     PoweredOn  192.168.1.1, [IP_1]
PS> Connect-VIServer [HOST_3].[DOMAIN_1].[TLD_1] -User [EMAIL_1] -Password [SECRET_1]
Contact [EMAIL_2] or [PHONE_1]. Again: [HOST_1].[DOMAIN_1].[TLD_1] [IP_1]`
	res := redact(t, in, rs)
	if res.Redacted != want {
		t.Fatalf("got:\n%s\nwant:\n%s", res.Redacted, want)
	}
}

func TestRedactStopsWhenContextEnds(t *testing.T) {
	rs := sets(t, &rules.Ruleset{Name: "s", Enabled: true, Rules: []*rules.Rule{{Name: "w", Enabled: true, Type: "words", Patterns: []string{"x"}}}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Redact(ctx, "x", rs, nil); err == nil {
		t.Fatal("a cancelled redaction must return an error, never a partly redacted text")
	}
}

func TestRedactKnownEntriesKeepTheirTokensAndNumberingContinues(t *testing.T) {
	hosts := &rules.Rule{Name: "hosts", Enabled: true, Type: "words", Patterns: []string{"web01", "db01", "app01"}, Placeholder: "[HOST_#]"}
	domains := &rules.Rule{Name: "domains", Enabled: true, Type: "words", Patterns: []string{"acme.sk", "acme.com", "contoso.sk"}, Placeholder: "[DOMAIN].[TLD]", PerPart: true}
	rs := sets(t, &rules.Ruleset{Name: "org", Enabled: true, Rules: []*rules.Rule{hosts, domains}})

	first := redact(t, "web01 and db01 on acme.sk", rs)
	if first.Redacted != "[HOST_1] and [HOST_2] on [DOMAIN_1].[TLD_1]" {
		t.Fatalf("first text: %q", first.Redacted)
	}
	second, err := Redact(context.Background(), "DB01, app01, contoso.sk, acme.com", rs, first.Map)
	if err != nil {
		t.Fatal(err)
	}
	if want := "[HOST_2], [HOST_3], [DOMAIN_2].[TLD_1], [DOMAIN_1].[TLD_2]"; second.Redacted != want {
		t.Fatalf("second text: got %q, want %q", second.Redacted, want)
	}
	if len(second.Map) != 4 || second.Map[0].Original != "DB01" || second.Map[0].Count != 1 {
		t.Fatalf("the map lists this text's values, as written here: %+v", second.Map)
	}

	// an entry that would put the value itself into the text is ignored
	bad := []Entry{{Token: "web01", Original: "web01", Placeholder: "[HOST_#]", Numbers: []int{1}}}
	if got, _ := Redact(context.Background(), "web01", rs, bad); got.Redacted != "[HOST_1]" {
		t.Fatalf("a known entry must not unredact: %q", got.Redacted)
	}
}
