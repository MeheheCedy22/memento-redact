package rules

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func found(t *testing.T, r *Rule, text string) []string {
	t.Helper()
	var out []string
	for _, m := range r.Find(text) {
		out = append(out, text[m[0]:m[1]])
	}
	return out
}

func compiled(t *testing.T, r *Rule) *Rule {
	t.Helper()
	if err := Compile([]*Ruleset{{Name: "test", Rules: []*Rule{r}}}); err != nil {
		t.Fatalf("compile: %v", err)
	}
	return r
}

func TestWordsAreCaseInsensitiveWholeWordsLongestFirst(t *testing.T) {
	r := compiled(t, &Rule{Name: "w", Type: "words", Patterns: []string{" acme ", "acme corp", "", "Čederle", "+420"}})
	got := found(t, r, "ACME Corp, acmeville, Acme, pan Čederle, tel +420.")
	want := []string{"ACME Corp", "Acme", "Čederle", "+420"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q, want %q", got, want)
	}
	if len(r.Patterns) != 4 || r.Patterns[0] != "acme" {
		t.Fatalf("patterns should be trimmed and empty lines dropped: %q", r.Patterns)
	}
}

func TestRegexRedactsFirstCaptureGroupAndHonorsKeep(t *testing.T) {
	r := compiled(t, &Rule{Name: "r", Type: "regex", Patterns: []string{`pass=(\S+)`, `\b\d+\.\d+\.\d+\.\d+\b`}, Keep: `^10\.`})
	got := found(t, r, "pass=hunter2 10.0.0.1 8.8.8.8")
	if strings.Join(got, "|") != "hunter2|8.8.8.8" {
		t.Fatalf("unexpected matches: %q", got)
	}
	if r.Placeholder != DefaultPlaceholder {
		t.Fatalf("expected default placeholder, got %q", r.Placeholder)
	}
}

func TestCompileRejectsInvalidRules(t *testing.T) {
	cases := map[string]*Ruleset{
		"invalid regex":    {Name: "s", Rules: []*Rule{{Name: "r", Type: "regex", Patterns: []string{"("}}}},
		"invalid keep":     {Name: "s", Rules: []*Rule{{Name: "r", Type: "words", Keep: "["}}},
		"unknown type":     {Name: "s", Rules: []*Rule{{Name: "r", Type: "glob"}}},
		"missing name":     {Name: "s", Rules: []*Rule{{Type: "words"}}},
		"missing set name": {Name: " "},
	}
	for name, set := range cases {
		if err := Compile([]*Ruleset{set}); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	err := Compile([]*Ruleset{{Name: "Net", Rules: []*Rule{{Name: "IP", Type: "regex", Patterns: []string{"("}}}}})
	if err == nil || !strings.Contains(err.Error(), `"Net"`) || !strings.Contains(err.Error(), `"IP"`) {
		t.Fatalf("error should name ruleset and rule: %v", err)
	}
}

func TestParseAcceptsJSONAndYAML(t *testing.T) {
	jsonSets, err := Parse([]byte(`[{"name":"s","enabled":true,"rules":[{"name":"r","enabled":true,"type":"regex","patterns":["\\bID-\\d+"]}]}]`))
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	yamlSets, err := Parse([]byte("- name: s\n  enabled: true\n  rules:\n    - name: r\n      enabled: true\n      type: regex\n      patterns: ['\\bID-\\d+']\n"))
	if err != nil {
		t.Fatalf("yaml: %v", err)
	}
	for _, sets := range [][]*Ruleset{jsonSets, yamlSets} {
		if got := found(t, sets[0].Rules[0], "ref ID-42"); len(got) != 1 || got[0] != "ID-42" {
			t.Fatalf("unexpected matches: %q", got)
		}
	}
}

func TestSyncSeedsThenUpgradesOnlyUneditedPresets(t *testing.T) {
	dir := t.TempDir()
	defaults := filepath.Join(dir, "defaults.yml")
	path := filepath.Join(dir, "data", "rulesets.yml")
	writeDefaults := func(pattern string) {
		t.Helper()
		yml := "- name: Net\n  enabled: true\n  rules:\n    - {name: IP, enabled: true, type: regex, patterns: ['" + pattern + "']}\n    - {name: MAC, enabled: false, type: regex, patterns: [mac]}\n" +
			"- name: Org\n  enabled: true\n  rules:\n    - {name: Words, enabled: true, type: words, patterns: ['" + pattern + "']}\n"
		if err := os.WriteFile(defaults, []byte(yml), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeDefaults("v1")
	if err := Sync(path, defaults); err != nil {
		t.Fatalf("seed: %v", err)
	}
	sets, err := Load(path)
	if err != nil || len(sets) != 2 || sets[0].Preset == "" {
		t.Fatalf("seeded presets must be stamped: %v %+v", err, sets)
	}

	// user: flips switches on Net (not an edit), adds a word to Org (an edit), adds an own ruleset
	sets[0].Enabled, sets[0].Rules[1].Enabled = false, true
	sets[1].Rules[0].Patterns = append(sets[1].Rules[0].Patterns, "acme")
	sets = append(sets, &Ruleset{Name: "Mine", Enabled: true, Rules: []*Rule{}})
	if err := Save(path, sets); err != nil {
		t.Fatal(err)
	}

	writeDefaults("v2")
	if err := Sync(path, defaults); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	sets, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := sets[0].Rules[0].Patterns[0]; got != "v2" {
		t.Errorf("unedited preset not upgraded: %q", got)
	}
	if sets[0].Enabled || !sets[0].Rules[1].Enabled {
		t.Errorf("upgrade must keep on/off switches: %+v", sets[0])
	}
	if got := strings.Join(sets[1].Rules[0].Patterns, ","); got != "v1,acme" {
		t.Errorf("edited preset must be kept: %q", got)
	}
	if len(sets) != 3 || sets[2].Name != "Mine" {
		t.Errorf("own rulesets must be kept: %+v", sets)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temp files left behind: %v", entries)
	}
}

func TestSyncLeavesUnstampedRulesetsAlone(t *testing.T) {
	dir := t.TempDir()
	defaults := filepath.Join(dir, "defaults.yml")
	path := filepath.Join(dir, "rulesets.yml")
	if err := os.WriteFile(defaults, []byte("- {name: Net, enabled: true, rules: [{name: IP, enabled: true, type: words, patterns: [new]}]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := "- {name: Net, enabled: true, rules: [{name: IP, enabled: true, type: words, patterns: [old]}]}\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Sync(path, defaults); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != old {
		t.Fatalf("rulesets without a preset stamp may be user-edited and must not change: %s", data)
	}
}

func TestLoadMissingFileFails(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yml")); err == nil {
		t.Fatal("a missing rules file must be an error, not an empty ruleset")
	}
}

// tokens are realistic-length examples of the fixed-prefix formats in "Well-known token formats".
var tokens = []string{
	"eyJhbGciOi.eyJzdWIiOi.SflKxwRJ",
	"AKIAIOSFODNN7EXAMPLE",
	"ghp_" + strings.Repeat("a1", 18),
	"glpat-" + strings.Repeat("x", 20),
	"xoxb-1234567890-abcdefghij",
	"xapp-1-A0123-456-abc123",
	"sk_live_" + strings.Repeat("Ab1", 8),
	"AIza" + strings.Repeat("B", 35),
	"sk-proj-" + strings.Repeat("Q", 30),
	"ATATT3" + strings.Repeat("xY_", 40),
	"abc8Q~" + strings.Repeat("z", 32),
	"dckr_pat_" + strings.Repeat("d", 27),
	"npm_" + strings.Repeat("n", 36),
	"pypi-AgEIcHlwaS5vcmc" + strings.Repeat("p", 60),
	"hf_" + strings.Repeat("h", 34),
	"hvs." + strings.Repeat("v", 95),
	"glsa_" + strings.Repeat("g", 32) + "_0123abcd",
	"dop_v1_" + strings.Repeat("f", 64),
	"SG." + strings.Repeat("s", 22) + "." + strings.Repeat("t", 43),
	"123456789:AA" + strings.Repeat("T", 33),
}

// TestDefaultPresets checks every factory rule against inputs it must and must not redact.
func TestDefaultPresets(t *testing.T) {
	sets, err := Load(filepath.Join("..", "..", "config", "default-rulesets.yml"))
	if err != nil {
		t.Fatalf("default rulesets must load: %v", err)
	}
	// a base64-encoded PEM block, as in Kubernetes secrets
	pem64 := func(label string) string {
		return base64.StdEncoding.EncodeToString([]byte("-----BEGIN " + label + "-----\nMIIfake\n-----END " + label + "-----\n"))
	}
	cases := []struct {
		rule  string
		input string
		want  []string
	}{
		{"Hosts in our domain", "web01.prod.domain.tld, DB01.domain.tld, alice@domain.tld, domain.tld.evil.com", []string{"web01.prod", "DB01"}},
		{"Company domain", "web01.domain.tld mail@Domain.TLD notdomain.tld", []string{"domain.tld", "Domain.TLD"}},
		{"Names and codenames", "nothing configured yet", nil},
		{"Public IPv4", "10.1.2.3 172.16.0.1 172.32.0.1 192.168.1.1 100.64.0.1 127.0.0.1 169.254.1.1 255.255.255.0 224.0.0.1 8.8.8.8 203.0.113.17 999.1.1.1", []string{"172.32.0.1", "8.8.8.8", "203.0.113.17"}},
		{"Public IPv4", "build 4.2.0.0.1, OID 1.3.6.1.4.1.311, dns 8.8.4.4. proxy 1.2.3.4:443", []string{"8.8.4.4", "1.2.3.4"}},
		{"Public IPv6", "2001:db8:85a3::8a2e:370:7334 fe80::250:56ff:fe9a:1b2c fd00::1 ff02::1 12:30:45 00:50:56:9a:1b:2c 2001:0db8:0000:0000:0000:ff00:0042:8329", []string{"2001:db8:85a3::8a2e:370:7334", "2001:0db8:0000:0000:0000:ff00:0042:8329"}},
		{"Git repositories", "git clone git@bitbucket.org:acme/infra-tools.git; git clone https://gitlab.example.com/team/app.git && git remote add up ssh://git@gitlab.example.com:2222/team/sub/tool.git; docs: https://github.com/acme/web", []string{"acme/infra-tools.git", "team/app.git", "team/sub/tool.git"}},
		{"MAC address", "00:50:56:9a:1b:2c and 00-50-56-9A-1B-2C", []string{"00:50:56:9a:1b:2c", "00-50-56-9A-1B-2C"}},
		{"Email", "mail jane.doe+x@corp.example.com now, clone git@bitbucket.org:acme/infra.git", []string{"jane.doe+x@corp.example.com"}},
		{"Phone number", "+420 777 123 456, +1 (555) 123-4567, 555-123-4567, +420777123456, 10.20.30.40, 2024", []string{"+420 777 123 456", "+1 (555) 123-4567", "555-123-4567", "+420777123456"}},
		{"Internal hostnames", "srv-db01.corp.local printer.lan api.svc.cluster.local pg.default.svc.cluster.local localhost.local example.com", []string{"srv-db01.corp", "printer"}},
		{"Cloud account IDs", "account_id: `123456789012` tenant_id = “72f988bf-86f1-41af-91ab-2d7cd011db47”", []string{"123456789012", "72f988bf-86f1-41af-91ab-2d7cd011db47"}},
		{"Cloud account IDs", `arn:aws:iam::123456789012:role/x /subscriptions/0b1f6471-1bf0-4dda-aec3-111122223333/rg AWS_ACCOUNT_ID=210987654321 "tenantId": "72f988bf-86f1-41af-91ab-2d7cd011db47" account_id=42`, []string{"123456789012", "0b1f6471-1bf0-4dda-aec3-111122223333", "210987654321", "72f988bf-86f1-41af-91ab-2d7cd011db47"}},
		{"Usernames", `JIRA_USER=marek user: root CORP\jdoe HKLM\Software /home/alice/x C:\Users\Bob\AppData C:\Users\Public "username": "carol" User-Agent: curl`, []string{"marek", "jdoe", "alice", "Bob", "carol"}},
		{"Payment card number", "4111 1111 1111 1111, 5500-0000-0000-0004, 378282246310005, 1234 5678 9012 3456", []string{"4111 1111 1111 1111", "5500-0000-0000-0004", "378282246310005"}},
		{"IBAN", "CZ65 0800 0000 1920 0014 5399, DE89370400440532013000, AB12", []string{"CZ65 0800 0000 1920 0014 5399", "DE89370400440532013000"}},
		{"Birth number (CZ/SK)", "850101/1234, 905512/123, 851301/1234, 2024/1234", []string{"850101/1234", "905512/123"}},
		{"Key-value secrets", `password=hunter2 "apiKey": "abc123" db_password: s3cr3t -Dspring.datasource.password=jdbc tokens are fine`, []string{"hunter2", "abc123", "s3cr3t", "jdbc"}},
		{"Key-value secrets", "JIRA_PASS=SVSIUIUFNOE890i0-x\nexport SMTP_PASSWORD=\"correct horse battery\"\nldap.pass: 'a b'\nDB_PASS=\"unclosed\nempty_token=\"\"", []string{"SVSIUIUFNOE890i0-x", "correct horse battery", "a b", "unclosed"}},
		{"Key-value secrets", `config['db_password'] = 'p w' "credentials": { heslo := tajne 'password' => 'x' client_secret=abc&scope=y aws_secret_access_key = wJalr`, []string{"p w", "tajne", "x", "abc", "wJalr"}},
		{"Usernames", `	DB_USER=$(jq -r '.db["user"]' "$CONFIG_FILE") USER=$USER login: "${LOGIN}"`, nil},
		{"Usernames", "admin_user: `jdoe` login = “ops” uid: „svc“ USER=`whoami` $admin_user = Read-Host -Prompt 'User' $user = 'carol'", []string{"jdoe", "ops", "svc", "carol"}},
		{"Usernames", `<entry key="dbUsername" value="svc_app"/> <entry key="dbLogin">ops</entry> <username>deploy</username>`, []string{"svc_app", "ops", "deploy"}},
		{"Key-value secrets", "DB_PASSWORD=$(jq -r '.db[\"password\"]' \"$CONFIG_FILE\")\nPASS=\"${JIRA_PASS}\" TOKEN=$TOKEN API_KEY=%API_KEY% token=$env:TOKEN pass=$1 secret: \"{{ .Values.secret }}\" token: ${{ secrets.TOKEN }} PASS=$(cat /run/secrets/db) PASS=`cat f` DB_PASS=\"$DB_USER:$DB_PASS\"", nil},
		{"Key-value secrets", `DB_PASS=${DB_PASS:-hunter2} PASS=$(echo 'p@ss' | base64 -d) TOKEN=$(printf -n s3cr3t) PASS=$(echo $OTHER)`, []string{"${DB_PASS:-hunter2}", "'p@ss'", "s3cr3t"}},
		{"Key-value secrets", "password: `hunter2` api_key = “abc123” token: «xyz789» secret := `raw` PASS=`cat f` $db_password = Read-Host -AsSecureString $db_pass = 'p w' $api_token = $null", []string{"hunter2", "abc123", "xyz789", "raw", "p w"}},
		{"Key-value secrets", `<entry key="dbPassword" value="hunter2"/> <entry key="apiToken">tok123</entry> <password>maven-pw</password> <input type="password"><label>`, []string{"hunter2", "tok123", "maven-pw"}},
		{"Command-line passwords", `--password "$PASS" -Password $env:PASS --token=${TOKEN} --api-key $(vault read -field=key x)`, nil},
		{"Auth headers", `curl -H "Authorization: Bearer $JIRA_TOKEN" -H "X-Api-Key: ${KEY}"`, nil},
		{"Credentials in URLs and curl", `https://$USER:$PASS@host curl -u "$JIRA_USER:$JIRA_PASS" https://x`, nil},
		{"Command-line passwords", `Connect-VIServer vc -User admin -Password 'S3cr3t!' --token=abc mkdir -p dir --http-password=x docker login --password-stdin < f mysqldump -uroot -pS3cr3t db -P 3306 sshpass -p hunter2 ssh h`, []string{"'S3cr3t!'", "abc", "x", "S3cr3t", "hunter2"}},
		{"PowerShell plain-text SecureString", `ConvertTo-SecureString "PlainTextPass" -AsPlainText -Force`, []string{`"PlainTextPass"`}},
		{"Auth headers", "curl -H \"Authorization: Bearer abc.def\" https://x\nCookie: sid=1; csrftoken=2\nPRIVATE-TOKEN: glpat-x\nHost: x", []string{"Bearer abc.def", "sid=1; csrftoken=2", "glpat-x"}},
		{"Credentials in URLs and curl", "mysql://app:s3cret@db.example.com/app https://example.com/a:b curl -s -u admin:hunter2 https://jira", []string{"app:s3cret", "admin:hunter2"}},
		{"Private keys", "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNz\n-----END OPENSSH PRIVATE KEY----- -----BEGIN PGP PRIVATE KEY BLOCK-----\nx\n-----END PGP PRIVATE KEY BLOCK-----", []string{"-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNz\n-----END OPENSSH PRIVATE KEY-----", "-----BEGIN PGP PRIVATE KEY BLOCK-----\nx\n-----END PGP PRIVATE KEY BLOCK-----"}},
		{"Private keys", "-----BEGIN PGP SECRET KEY BLOCK-----\n\nlQOYfake\n-----END PGP SECRET KEY BLOCK-----\n" +
			"---- BEGIN SSH2 ENCRYPTED PRIVATE KEY ----\nComment: \"rsa-key\"\nP2/56wfake\n---- END SSH2 ENCRYPTED PRIVATE KEY ----\n" +
			`{"pem": "-----BEGIN EC PRIVATE KEY-----\nProc-Type: 4,ENCRYPTED\nMHcCAQEEIfake"}` + "\n" +
			"Public-Lines: 1\nAAAAC3NzaPublic\nPrivate-Lines: 2\nAAAAIFakePrivate\nLine2==\nPrivate-MAC: 0123abcd\n" +
			"age: AGE-SECRET-KEY-1" + strings.Repeat("Q", 58) + "\n" +
			"tls.crt: " + pem64("CERTIFICATE") + "\ntls.key: " + pem64("PRIVATE KEY") + "\nclient-key: " + pem64("OPENSSH PRIVATE KEY") + "\n" +
			"-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEAfake\r\nQyNTUxOQ==\nnext step: done", []string{
			"-----BEGIN PGP SECRET KEY BLOCK-----\n\nlQOYfake\n-----END PGP SECRET KEY BLOCK-----",
			"---- BEGIN SSH2 ENCRYPTED PRIVATE KEY ----\nComment: \"rsa-key\"\nP2/56wfake\n---- END SSH2 ENCRYPTED PRIVATE KEY ----",
			`-----BEGIN EC PRIVATE KEY-----\nProc-Type: 4,ENCRYPTED\nMHcCAQEEIfake`,
			"AAAAIFakePrivate\nLine2==",
			"AGE-SECRET-KEY-1" + strings.Repeat("Q", 58),
			pem64("PRIVATE KEY"),
			pem64("OPENSSH PRIVATE KEY"),
			"-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEAfake\r\nQyNTUxOQ==",
		}},
		{"Well-known token formats", strings.Join(tokens, " ") + " ghp_short notakey", tokens},
		{"Webhook URLs", "https://hooks.slack.com/services/T000/B000/XXXXXXXXXXXXXXXXXXXXXXXX https://discord.com/api/webhooks/123/abc-DEF https://contoso.webhook.office.com/webhookb2/abc@def/IncomingWebhook/x/y", []string{"services/T000/B000/XXXXXXXXXXXXXXXXXXXXXXXX", "123/abc-DEF", "webhookb2/abc@def/IncomingWebhook/x/y"}},
	}
	tested := map[string]bool{}
	for _, tc := range cases {
		t.Run(tc.rule, func(t *testing.T) {
			r := findRule(t, sets, tc.rule)
			tested[tc.rule] = true
			if got := found(t, r, tc.input); strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	for _, set := range sets {
		for _, r := range set.Rules {
			if !tested[r.Name] {
				t.Errorf("default rule %q has no test case", r.Name)
			}
		}
	}
}

func findRule(t *testing.T, sets []*Ruleset, name string) *Rule {
	t.Helper()
	for _, set := range sets {
		for _, r := range set.Rules {
			if r.Name == name {
				return r
			}
		}
	}
	t.Fatalf("rule %q not found", name)
	return nil
}
