# Ruleset Guide

Rulesets are bundles of rules, stored as YAML (`RULES_PATH`, seeded from `config/default-rulesets.yml`).

```yaml
- name: Organization
  description: shown as a hover hint
  enabled: true
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

## Factory presets

- **Organization**: hosts under `domain.tld`, the domain itself (`[DOMAIN].[TLD]`), an empty names/codenames list.
- **Network**: public IPv4/IPv6 (private, loopback, link-local, CGNAT, multicast kept), MAC (off by default).
- **Personal data**: email, phone numbers.
- **Secrets**: key=value secrets, CLI password flags, PowerShell SecureString plaintext, auth headers, URL credentials, private keys, well-known token formats.

Add a case to `TestDefaultPresets` (`internal/rules/rules_test.go`) for every preset rule you add or change; the test fails if a preset rule has none.
