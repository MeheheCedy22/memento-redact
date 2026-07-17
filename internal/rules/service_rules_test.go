package rules

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionRulesCoverGenericCredentialFields(t *testing.T) {
	ruleset := mustLoadProductionRules(t)
	cases := []struct {
		name         string
		ruleName     string
		input        string
		wantContains string
		wantNot      string
	}{
		{name: "json username", ruleName: "json_username", input: `username: "alice"`, wantContains: `[REDACTED_USERNAME]`, wantNot: "alice"},
		{name: "json password", ruleName: "json_password", input: `password: "s3cr3t"`, wantContains: `[REDACTED_SECRET]`, wantNot: "s3cr3t"},
		{name: "auth header", ruleName: "http_authorization_header", input: `Authorization: Bearer abc.def.ghi`, wantContains: `[REDACTED_AUTH]`, wantNot: "abc.def.ghi"},
		{name: "cookie header", ruleName: "http_cookie_header", input: `Cookie: sessionid=abc123; csrftoken=xyz`, wantContains: `[REDACTED_COOKIE]`, wantNot: "sessionid=abc123"},
		{name: "command password flag", ruleName: "command_password_flag", input: `deploy --password hunter2`, wantContains: `[REDACTED_SECRET]`, wantNot: "hunter2"},
		{name: "command username flag", ruleName: "command_username_flag", input: `deploy --user alice`, wantContains: `[REDACTED_USERNAME]`, wantNot: "alice"},
		{name: "connection string", ruleName: "generic_connection_string_credentials", input: `mysql://appuser:appsecret@db.example.com/app`, wantContains: `[REDACTED_SECRET]`, wantNot: "appsecret"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := findRule(t, ruleset, tc.ruleName)
			got := rule.Apply(tc.input)
			if !strings.Contains(got, tc.wantContains) {
				t.Fatalf("expected %q to contain %q", got, tc.wantContains)
			}
			if tc.wantNot != "" && strings.Contains(got, tc.wantNot) {
				t.Fatalf("expected %q not to contain %q", got, tc.wantNot)
			}
		})
	}
}

func TestProductionRulesCoverServiceSpecificSecrets(t *testing.T) {
	ruleset := mustLoadProductionRules(t)
	cases := []struct {
		name         string
		ruleName     string
		input        string
		wantContains string
		wantNot      string
	}{
		{name: "java system property", ruleName: "java_system_password_property", input: `-Dspring.datasource.password=jdbcpass`, wantContains: `[REDACTED_SECRET]`, wantNot: "jdbcpass"},
		{name: "tomcat manager password", ruleName: "tomcat_manager_password", input: `tomcat.manager.password = managerpass`, wantContains: `[REDACTED_SECRET]`, wantNot: "managerpass"},
		{name: "wildfly management password", ruleName: "wildfly_management_password", input: `--management-password wildflypass`, wantContains: `[REDACTED_SECRET]`, wantNot: "wildflypass"},
		{name: "apache auth directive", ruleName: "apache_basic_auth_file", input: `AuthUserFile /etc/apache2/.htpasswd`, wantContains: `[REDACTED_APACHE_AUTH_DIRECTIVE]`, wantNot: "/etc/apache2/.htpasswd"},
		{name: "mysql cli password", ruleName: "mysql_cli_password", input: `mysqldump -uroot -psecret dbname`, wantContains: `[REDACTED_SECRET]`, wantNot: "secret"},
		{name: "oracle sqlplus", ruleName: "oracle_sqlplus_credentials", input: `sqlplus appuser/oraclepwd@orclpdb1`, wantContains: `[REDACTED_SECRET]`, wantNot: "oraclepwd"},
		{name: "ldap bind password", ruleName: "ldap_bind_password", input: `ldapsearch -x -D "cn=admin,dc=example,dc=com" -W`, wantContains: `[REDACTED_LDAP_SECRET]`, wantNot: "-W"},
		{name: "sshpass password", ruleName: "sshpass_password", input: `sshpass -p hunter2 ssh user@host`, wantContains: `[REDACTED_SECRET]`, wantNot: "hunter2"},
		{name: "powershell secure string", ruleName: "powershell_secure_string_plaintext", input: `ConvertTo-SecureString "PlainTextPass" -AsPlainText -Force`, wantContains: `[REDACTED_SECRET]`, wantNot: "PlainTextPass"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := findRule(t, ruleset, tc.ruleName)
			got := rule.Apply(tc.input)
			if !strings.Contains(got, tc.wantContains) {
				t.Fatalf("expected %q to contain %q", got, tc.wantContains)
			}
			if tc.wantNot != "" && strings.Contains(got, tc.wantNot) {
				t.Fatalf("expected %q not to contain %q", got, tc.wantNot)
			}
		})
	}
}

func TestProductionRulesCoverDirectoryAndSSHData(t *testing.T) {
	ruleset := mustLoadProductionRules(t)
	cases := []struct {
		name         string
		ruleName     string
		input        string
		wantContains string
		wantNot      string
	}{
		{name: "ldap uri credentials", ruleName: "ldap_uri_credentials", input: `ldap://binduser:bindpass@ldap.example.com`, wantContains: `[REDACTED_SECRET]`, wantNot: "bindpass"},
		{name: "ldap dn", ruleName: "ldap_user_dn", input: `userdn: cn=admin,dc=example,dc=com`, wantContains: `[REDACTED_DN]`, wantNot: "cn=admin"},
		{name: "ssh authorized key comment", ruleName: "ssh_authorized_keys_comment", input: `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMycomment user@host`, wantContains: `[REDACTED_SSH_KEY]`, wantNot: "user@host"},
		{name: "powershell password param", ruleName: "powershell_password_param", input: `-Password "Secret123"`, wantContains: `[REDACTED_POWERSHELL_SECRET]`, wantNot: "Secret123"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := findRule(t, ruleset, tc.ruleName)
			got := rule.Apply(tc.input)
			if !strings.Contains(got, tc.wantContains) {
				t.Fatalf("expected %q to contain %q", got, tc.wantContains)
			}
			if tc.wantNot != "" && strings.Contains(got, tc.wantNot) {
				t.Fatalf("expected %q not to contain %q", got, tc.wantNot)
			}
		})
	}
}

func mustLoadProductionRules(t *testing.T) []*Rule {
	t.Helper()
	loaded, err := LoadRules(filepath.Join("..", "..", "config", "rules.yml"))
	if err != nil {
		t.Fatalf("load production rules: %v", err)
	}
	return loaded
}

func findRule(t *testing.T, ruleset []*Rule, name string) *Rule {
	t.Helper()
	for _, rule := range ruleset {
		if rule.Name == name {
			return rule
		}
	}
	t.Fatalf("rule %q not found", name)
	return nil
}
