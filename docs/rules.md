# Rules

Rules decide what is redacted. They come in **rulesets**, bundles you switch on or off as a whole or rule by rule. The server keeps them in one YAML file (`RULES_PATH`). On first start it creates that file from the factory presets in [config/default-rulesets.yml](../config/default-rulesets.yml).

You can edit rules on the **Rules** page (one tab per ruleset), export them to YAML (all, or one ruleset), import rulesets from a file (you pick which ones, then add them or replace all), or change them with the CLI ([api.md](api.md#cli)).

## Format

```yaml
- name: Organization
  description: shown as a hover hint
  enabled: true
  preset: 3f9a…                 # set by the server for factory presets; do not edit
  rules:
    - name: Hosts in our domain
      description: shown as a hover hint
      enabled: true
      type: regex                 # or: words
      patterns:
        - '(?i)\b([a-z0-9-]+(?:\.[a-z0-9-]+)*)\.domain\.tld\b'
      keep: ''                    # optional regex; matching values are not redacted
      placeholder: '[HOST_#]'     # '#' -> 1, 2, 3 per distinct value
      per_part: false             # see "Numbering each part"
      samples:                    # optional lines to try the rule on; redaction ignores them
        - 'ssh web01.domain.tld'
```

| Field | Required | Meaning |
|---|---|---|
| ruleset `name` | yes | Shown in the UI and the map. The CLI and preset upgrades find rulesets by name, so keep names unique. |
| ruleset `enabled` | | Off: none of its rules run. |
| ruleset `preset` | | Fingerprint of the factory version. Set by the server, empty for your own rulesets. |
| rule `name` | yes | Shown in the map. Keep it unique within the ruleset. |
| rule `enabled` | | Off: the rule does not run. |
| `type` | yes | `words` or `regex`. |
| `patterns` | | Words or regexes, one per item. Empty items are dropped. |
| `keep` | | A regex. Values that match it are left as they are. |
| `placeholder` | | Replacement text. `#` becomes a number. Default: `[REDACTED_#]`. |
| `per_part` | | Number each dot-separated part of the placeholder separately. |
| `samples` | | Lines to try the rule on. The Rules page shows each one redacted by this rule alone as you edit, and marks lines it leaves as they were with *not redacted*. Redaction ignores them. They are not an edit: a preset with samples still updates, and keeps them. |

YAML is a superset of JSON, so imports also accept JSON. In single-quoted YAML strings a backslash is literal and a quote is written as `''`.

The server checks every rule before it saves or previews: names are present, `type` is valid, and every pattern and `keep` regex compiles. If any check fails, nothing is saved, and the error names the ruleset and the rule.

## How matching works

- **words**: literal text, case-insensitive. Leading and trailing spaces are trimmed. When phrases overlap, the longest wins, so `acme corp` beats `acme`. Matching is whole words only, but the word-boundary check is ASCII-only: a word that starts or ends with a letter like `á` or `Č` also matches inside a longer word.
- **regex**: [Go RE2 syntax](https://github.com/google/re2/wiki/Syntax). RE2 has no lookarounds and no backreferences. Its matching time grows linearly with the text, so no regex can hang the server. Each pattern is separate, so a flag like `(?i)` applies only to the pattern it is in.
- **Capture groups**: if a regex has groups, only the first group that matched is redacted. `password=(\S+)` gives `password=[SECRET_1]`, and the key name stays readable. Without groups, the whole match is redacted. Empty matches are ignored.
- **keep** is tested against the part that would be redacted (the group, if there is one), not the whole match. Anchor it with `^…$` when it has to match the whole value.
- **All rules run on the original text**, so no rule ever matches inside another rule's placeholder.
- **Overlaps**: when matches overlap, the longest wins. For equal lengths, the rule higher in the list wins (rulesets top to bottom, then rules top to bottom).

## Regex cheat sheet

The same sheet is on the Rules page under **Regex cheat sheet**.

| Syntax | Meaning |
|---|---|
| `(?i)` | at the start: ignore upper and lower case |
| `.` | any character except a line break |
| `\.` `\(` `\[` `\\` | a literal dot, bracket, backslash: put `\` before any of `. * + ? ( ) [ ] { } \| ^ $ \` |
| `\d` `\w` `\s` | a digit · a letter, digit or `_` · whitespace, including line breaks. Uppercase means "not": `\S` is not whitespace |
| `\p{L}` | any letter, including á, č, ž (`\w` is ASCII only) |
| `[abc]` `[a-z0-9-]` | one character from the set |
| `[^"\n]` | one character not in the set (here: not a quote, not a line break) |
| `*` `+` `?` | what comes before, 0 or more · 1 or more · 0 or 1 times |
| `{3}` `{2,4}` `{8,}` | exactly 3 · 2 to 4 · 8 or more times |
| `*?` `+?` | as few times as possible |
| `a\|b` | a or b |
| `\b` | word boundary: `\bweb01\b` does not match inside `myweb01x` (ASCII letters only) |
| `^` `$` | start and end of the value; use them in Keep |
| `( )` | group: only this part is replaced |
| `(?: )` | group for `\|` or repeats that does not change what is replaced |

In this project:

- Each line in *Patterns* is its own regex.
- Only the first group that matched is replaced: `(?i)\bcustomer_id=(\w+)` gives `customer_id=[CUSTOMER_1]`. Without a group, the whole match is replaced.
- *Keep* is tested against the replaced part only. Anchor it, `^(?:test|demo)$`, or `test` also keeps `latest`.
- Not supported: lookarounds `(?=…)`, `(?!…)`, `(?<=…)` and backreferences `\1`.
- In an exported YAML file every regex is in single quotes; a quote inside is written `''`.

## Placeholders and numbering

- `#` becomes a number per distinct value: `[HOST_1]`, `[HOST_2]`. A repeated value keeps its number, so the LLM can still tell which host is which.
- Numbers are counted per placeholder text in order of appearance. Rules with the same placeholder share one count: *Hosts in our domain* and *Internal hostnames* both use `[HOST_#]`.
- Values are compared ignoring case: `web01` and `WEB01` get the same number, and the map shows the spelling seen first.
- Without `#`, every value gets the same fixed text.
- In the UI, numbering continues across the texts you copy out (a *conversation*): `web01` stays `[HOST_1]` in the next text, and new values continue after the highest number. **New conversation** starts from 1 again. Through the API, pass the earlier `map` as `known` ([api.md](api.md#post-v1preview)); without it, every text starts from 1.

### Numbering each part

With `per_part: true` (*Number each part* in the UI), a placeholder with dots is split at the dots, and each part is numbered by its own value, ignoring case:

| Value | `[DOMAIN].[TLD]` |
|---|---|
| `acme.sk` | `[DOMAIN_1].[TLD_1]` |
| `acme.com` | `[DOMAIN_1].[TLD_2]` |
| `corp.acme.sk` | `[DOMAIN_2].[TLD_1]` (extra leading labels belong to the first part) |
| `localhost` | `[DOMAIN].[TLD]` (fewer parts than the placeholder: plain placeholder) |

If a part contains `#`, the number replaces it (`[D#].[T#]` → `[D1].[T1]`). Otherwise `_N` is added before the closing `]`.

### Values that only reference a secret

`$VAR`, `${VAR}`, `$(command)`, `` `command` ``, `%VAR%`, `$env:VAR` and `{{ template }}` hold no secret themselves, so the presets' *keep* regex leaves them alone. A default value (`${VAR:-literal}`) or a literal inside `$(echo …)` is still redacted.

PowerShell `$password = Read-Host -AsSecureString` is kept too: an unquoted word after `$x =` is a command call there, not a value. `$password = 'hunter2'` is redacted.

Backticks depend on the separator. In shell, `` PASS=`cat file` `` (no spaces around `=`) runs a command, so it is kept. After `:`, `:=`, `=>` or a spaced `=`, backticks are quotes (Markdown, Go raw strings), so `` password: `hunter2` `` becomes `` password: `[SECRET_1]` ``. Typographic quotes (`“…”`, `„…“`, `‘…’`, `«…»`) work like `"…"`: only the inside is replaced.

## Presets

| Ruleset | What it finds |
|---|---|
| Organization | Hosts in your domain, the domain itself, your own list of names and codenames |
| Network | Public IPv4 and IPv6 (private ranges kept), internal hostnames, cloud account IDs, git repository paths (`git@host:path`, clone URLs ending in `.git`), MAC addresses (off by default) |
| Personal data | Email, phone numbers, usernames, payment cards, IBAN, CZ/SK birth numbers |
| Secrets | Key-value secrets, command-line passwords, PowerShell SecureString, auth headers, credentials in URLs and curl, private keys (PEM, OpenSSH, SSH2, PGP, PuTTY, age; also cut off or base64-encoded), well-known token formats, webhook URLs |

The secret patterns are adapted from the [gitleaks default config](https://github.com/gitleaks/gitleaks/blob/master/config/gitleaks.toml) and the [detect-secrets keyword list](https://github.com/Yelp/detect-secrets/blob/master/detect_secrets/plugins/keyword.py).

**After the first start**, open the **Rules** page and replace `domain.tld` with your real domain in two places: the *Hosts in our domain* regex and the *Company domain* words. Then add people, customers and project names to *Names and codenames*.

### Upgrades

On every start, the server compares your rules with the factory presets (`rules.Sync`):

- A preset you have not edited is replaced by the new factory version. Your on/off switches are kept.
- A preset you edited stays as it is, and the Rules page shows **Update available**. **Restore preset** takes the new version and drops your edits to that ruleset.
- Your own rulesets, and rulesets without a `preset` stamp, are never touched.
- New presets and presets you deleted are not added automatically. Add them under **Add preset…**.

Switching rules on or off does not count as an edit. Changing a name, description, pattern, keep regex or placeholder does.

## Writing your own rules

| Goal | Type | Pattern | Placeholder |
|---|---|---|---|
| Customer and project names | words | `Contoso`, `Project Phoenix` (one per line) | `[NAME_#]` |
| Ticket IDs | regex | `\bOPS-\d+\b` | `[TICKET_#]` |
| Only the value after a key | regex | `(?i)\bcustomer_id=(\w+)` | `[CUSTOMER_#]` |
| Short VM names like `srv-db01` | regex | `(?i)\bsrv-[a-z]+\d+\b` | `[HOST_#]` |

To skip some values, add a keep regex, for example `^(?:test|demo)$`.

To test a rule, paste a sample on the **Redact** page. The preview uses your unsaved edits. The counts next to each rule and the **Map** show what every rule replaced. To report a rule that redacts too much or too little, use **Debug** in the *Map* (see the [README](../README.md#using-it)). **Save** stores the rules for the CLI.

## Changing the factory presets

This section is for contributors. Edit [config/default-rulesets.yml](../config/default-rulesets.yml), then add a case to `TestDefaultPresets` ([internal/rules/rules_test.go](../internal/rules/rules_test.go)) for every preset rule you add or change. The test fails if a preset rule has no case. Run `go test ./internal/rules`. Installed copies that nobody has edited update on their next start.
