# Ruleset Guide

Rulesets are bundles of rules, stored as YAML (`RULES_PATH`, seeded from `config/default-rulesets.yml`).

```yaml
- name: Organization
  description: shown as a hover hint
  enabled: true
  preset: 3f9a…                 # fingerprint of the factory version; empty for your own rulesets
  rules:
    - name: Hosts in our domain
      enabled: true
      type: regex                 # or: words
      patterns:
        - '(?i)\b([a-z0-9-]+(?:\.[a-z0-9-]+)*)\.domain\.tld\b'
      keep: ''                    # optional regex; matching values are not redacted
      placeholder: '[HOST_#]'     # '#' -> 1, 2, 3 per distinct value
```

## Semantics

- `words`: literal, case-insensitive, whole-word (ASCII word boundaries), longest phrase first.
- `regex`: Go RE2. If the regex has capture groups, the first participating group is redacted, otherwise the whole match.
- Every rule runs on the original text; overlapping matches resolve longest-first, ties to the earlier rule.
- Same placeholder + same original value gives the same token. Numbering follows order of appearance.
- Upgrades (`rules.Sync` on start): a preset whose content still matches its `preset` fingerprint is replaced by the new factory version, keeping on/off switches. Edited presets and unstamped rulesets are never touched.

## What the presets cover, and why

Secret leaks are dominated by things without a fixed format: GitGuardian's State of Secrets Sprawl 2025 reports generic secrets (hardcoded passwords, database credentials) as over half of all detected leaks, generic passwords in 24% of private repos, and Jira tickets as the collaboration tool that leaks most (6.1% of tickets). Those can only be caught by key names, so **Key-value secrets** and **Command-line passwords** are the most important rules. Their keyword list follows Yelp detect-secrets (`pass`, `pwd`, `secret`, `api/auth/db/private/client…_key`, `contraseña`), gitleaks' generic rule (`token`, `credential`, `creds`) and adds Czech/Slovak `heslo`.

Fixed-prefix provider tokens are matched nearly without false positives; **Well-known token formats** takes its patterns from the gitleaks default config (AWS, GitHub, GitLab, Slack, Stripe, Google, OpenAI/Anthropic `sk-`, Atlassian `ATATT3`, Azure AD `…Q~`, Docker Hub, npm, PyPI, Hugging Face, Vault, Grafana, DigitalOcean, SendGrid, Telegram, JWT), plus private key blocks, URL/curl credentials, auth headers and webhook URLs.

For what people paste into LLMs (LayerX, Cyberhaven, Harmonic: internal business data, source code, customer/personal data; Samsung 2023: source code and meeting notes), the presets cover the parts a regex can find: hostnames and internal domains, public IPs, cloud account IDs, emails, phones, usernames (`user=`, `DOMAIN\user`, home folders), payment cards, IBANs and CZ/SK birth numbers (Presidio's global entity list, plus a local one). Names of people, customers and projects cannot be found by pattern: they go into *Organization → Names and codenames*.

Not covered on purpose: high-entropy detection (gitleaks' generic rule needs an entropy filter this engine does not have), bare `key:` values, URLs and UUIDs in general (too many false positives in IT text).

Sources: [gitleaks config](https://github.com/gitleaks/gitleaks/blob/master/config/gitleaks.toml), [detect-secrets keyword plugin](https://github.com/Yelp/detect-secrets/blob/master/detect_secrets/plugins/keyword.py), [GitGuardian 2025](https://blog.gitguardian.com/the-state-of-secrets-sprawl-2025-pr), [Presidio supported entities](https://presidio.dataprivacystack.org/supported_entities/), [GitHub secret scanning patterns](https://docs.github.com/en/enterprise-cloud@latest/code-security/reference/secret-security/supported-secret-scanning-patterns).

Add a case to `TestDefaultPresets` (`internal/rules/rules_test.go`) for every preset rule you add or change; the test fails if a preset rule has none.
