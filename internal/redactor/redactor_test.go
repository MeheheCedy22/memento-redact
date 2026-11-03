package redactor

import (
	"path/filepath"
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

func TestRedactNumbersDistinctValuesAndReusesTokens(t *testing.T) {
	rs := sets(t, &rules.Ruleset{Name: "s", Enabled: true, Rules: []*rules.Rule{
		{Name: "email", Enabled: true, Type: "regex", Patterns: []string{`\S+@\S+\.com`}, Placeholder: "[EMAIL_#]"},
		{Name: "off", Enabled: false, Type: "words", Patterns: []string{"mail"}},
	}})
	res := Redact("mail a@x.com, b@x.com, again a@x.com", rs)
	if want := "mail [EMAIL_1], [EMAIL_2], again [EMAIL_1]"; res.Redacted != want {
		t.Fatalf("got %q, want %q", res.Redacted, want)
	}
	if len(res.Map) != 2 || res.Map[0] != (Entry{Token: "[EMAIL_1]", Original: "a@x.com", Ruleset: "s", Rule: "email", Count: 2}) {
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
	res := Redact("bob@acme.com via acme.com, ACME.COM", rs)
	if want := "[EMAIL_1] via [DOMAIN], [DOMAIN]"; res.Redacted != want {
		t.Fatalf("got %q, want %q", res.Redacted, want)
	}
	if len(res.Map) != 3 {
		t.Fatalf("fixed placeholders keep one entry per distinct original: %+v", res.Map)
	}
}

func TestRedactPerPartNumbersDomainAndTLDSeparately(t *testing.T) {
	rule := &rules.Rule{Name: "domains", Enabled: true, Type: "words", Patterns: []string{"acme.sk", "contoso.com", "acme.com", "corp.acme.sk"}, Placeholder: "[DOMAIN].[TLD]", PerPart: true}
	rs := sets(t, &rules.Ruleset{Name: "org", Enabled: true, Rules: []*rules.Rule{rule}})
	res := Redact("acme.sk contoso.com acme.com ACME.SK corp.acme.sk", rs)
	if want := "[DOMAIN_1].[TLD_1] [DOMAIN_2].[TLD_2] [DOMAIN_1].[TLD_2] [DOMAIN_1].[TLD_1] [DOMAIN_3].[TLD_1]"; res.Redacted != want {
		t.Fatalf("got %q, want %q", res.Redacted, want)
	}

	rule.Placeholder = "[D#].[T#]"
	if got := Redact("contoso.com acme.sk", rs).Redacted; got != "[D1].[T1] [D2].[T2]" {
		t.Fatalf("# inside parts: %q", got)
	}

	rule.PerPart = false
	rule.Placeholder = "[DOMAIN].[TLD]"
	if got := Redact("acme.sk contoso.com", rs).Redacted; got != "[DOMAIN].[TLD] [DOMAIN].[TLD]" {
		t.Fatalf("switched off: one fixed placeholder expected, got %q", got)
	}
}

func TestRedactEmptyInputAndNoRules(t *testing.T) {
	if res := Redact("", nil); res.Redacted != "" || res.Map == nil || len(res.Map) != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res := Redact("plain žluťoučký text", nil); res.Redacted != "plain žluťoučký text" {
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
	res := Redact(in, rs)
	if res.Redacted != want {
		t.Fatalf("got:\n%s\nwant:\n%s", res.Redacted, want)
	}
}
