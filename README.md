# memento-redact

[![ci](https://github.com/MeheheCedy22/memento-redact/actions/workflows/ci.yml/badge.svg)](https://github.com/MeheheCedy22/memento-redact/actions/workflows/ci.yml)

Redact logs, configs, emails and command output before you paste them into an LLM, then put the real values back into its answer. Self-hosted: your text never leaves your machine.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/screenshots/redact-dark.png">
  <img alt="The Redact page: PowerCLI output on the left, the same text with highlighted placeholders such as [HOST_1], [IP_1] and [SECRET_1] on the right, the rulesets and the map of placeholders below" src="docs/screenshots/redact-light.png">
</picture>

## Why use it

- **Nothing leaves your machine.** One small container on your computer. The text you redact is never written to disk or to logs; only your rules are stored.
- **The LLM can still help.** Every distinct value gets its own placeholder (`[HOST_1]`, `[HOST_2]`) that stays the same for the whole conversation, and key names, private IPs and `$VARIABLES` stay readable. The answer still makes sense, and it comes back with the real values.
- **More than find-and-replace.** 24 preset rules for hosts, IPs, emails, phone numbers, passwords, private keys and dozens of token formats work from the first start. Before you copy, a leak check tells you if a redacted value still appears somewhere in the text.
- **You see everything.** The original and the redacted text side by side, every placeholder with what it replaced, and nothing is sent anywhere until you click **Copy**.

## Features

- **Consistent placeholders**: the same value keeps its token, also in the next text of the same conversation; new values continue the numbering. Domains can be numbered per part: `[DOMAIN_1].[TLD_2]`.
- **Restore answers**: paste the LLM's answer and get it back with the real values, even when it changed the case or escaped the brackets. Runs in your browser only.
- **Leak check**: values redacted in one place but left in another (a password repeated without its key, a name inside a longer word) are outlined, and **Copy** asks first.
- **Presets that keep up**: Organization, Network, Personal data and Secrets, adapted from gitleaks and detect-secrets. Unedited presets update themselves; switch any ruleset or rule on or off.
- **Rules you can trust**: words or regexes, warnings for risky patterns (`.*`, a missing `\b`), sample lines checked as you type, optional detection of random-looking tokens near key words.
- **Map**: every placeholder with its original value, rule and count; sort by any column, click to jump to it in the text.
- **Profiles and debug reports**: save combinations of switches; mark wrong redactions and copy a report for fixing the rules.
- **CLI and HTTP API** with the same features as the UI: redact, conversations, restore, rules, profiles, checks.
- **Small and hardened**: one container of about 35 MB (amd64 and arm64) and no database. It listens on localhost only and runs read-only as an unprivileged user.

## Quick start

```bash
docker compose up -d --build
# open http://localhost:8080
```

Rules live in the `memento-data` volume and survive restarts and upgrades. The port is bound to `127.0.0.1` because there is no login and your rules list your sensitive names.

**First step**: on the **Rules** page, replace `domain.tld` with your real domain in *Organization* (*Hosts in our domain* and *Company domain*), add names of people, customers and projects to *Names and codenames*, and click **Save**.

## Using it

1. **Redact**: paste text into *Original*. Drag the bottom-right corner to make the two boxes taller; this browser remembers the height. *Redacted* updates as you type, and the text itself is never stored. Hover a placeholder to see what it replaced. A value redacted in one place but left in another, like a password repeated without its key or a name inside a longer word, is outlined in red, and **Copy** asks first. In the *Map*, click a placeholder to jump to it in the text, click again for the next occurrence; click a column header to sort. Then click **Copy**.
2. **Next text**: whatever you copy out joins the *conversation* (counted next to *Map*), so the next text you paste keeps the same placeholders for the same values, and new values continue the numbering. **New conversation** starts from 1. The conversation lives only in this browser tab.
3. **Restore the answer**: on the **Restore** page, paste what the LLM answered. Every placeholder of the conversation gets its real value back, also when the LLM changed the case, escaped the brackets (`\[HOST_1\]`) or used part of a domain on its own (`[DOMAIN_1]`). Placeholders it can't restore stay as they are and are outlined; hover one to see why. Restoring happens in your browser only.
4. **Something is not redacted?** Add the word to *Organization → Names and codenames*, or add a regex rule on the **Rules** page (one tab per ruleset). The preview uses your edits right away. Put a few lines of your text under a rule's patterns (*Sample lines*) to see what that rule alone does with them. A rule that probably redacts too much or too little gets a warning, and its tab a `!`.
5. **Profiles**: switch rulesets and rules on or off next to the text, then pick **Save switches as…** above *Rulesets* to keep that combination under a name. Picking a profile later applies it in one click. Profiles are saved in this browser.
6. **Save** stores the rules on the server, for the CLI too. **Discard** goes back to the last save. **Restore preset** resets one ruleset, **Reset all** resets everything, **Export** downloads the saved rules as YAML; the **Export** in a ruleset's tab downloads only that ruleset. **Import…** lists the rulesets in a YAML or JSON file: tick the ones you want, then **Add** (one with the same name replaces yours) or **Replace all**. Nothing is saved until you click **Save**.
7. **Report a wrong redaction**: click **Debug** in the *Map*. Pick a reason next to a wrong placeholder (false positive, too much, too little, wrong type, wrong number, other) and add a description if you like. **New reason…** saves your own reason in this browser for next time. To report text that was not redacted, select it in *Redacted* and click **Add selection**. Then click **Copy report**. The report lists each item with its rule and the lines it occurs in. Only the values you marked are restored; everything else stays redacted.

**Restore**: the LLM's answer on the left, the same answer with the real values on the right; `[HOST_9]` is not in the map, so it stays and is outlined.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/screenshots/restore-dark.png">
  <img alt="The Restore page: an answer with placeholders such as [HOST_1].[DOMAIN_1].[TLD_1] and [IP_1] on the left, the same answer with the real host name, IP address and email on the right, and [HOST_9] outlined as not restored" src="docs/screenshots/restore-light.png">
</picture>

**Rules page**: one tab per ruleset; words and regexes, placeholders, *Keep* exceptions, *Number each part*, and sample lines that show what each rule does. "How rules work" and a regex cheat sheet open at the top.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/screenshots/rules-dark.png">
  <img alt="The Rules page: the Organization ruleset with a regex rule for hosts and its sample line 'ssh admin@web01.prod.domain.tld' shown as 'ssh admin@[HOST_1].domain.tld', a words rule for the company domain with Number each part ticked, and an empty Names and codenames rule" src="docs/screenshots/rules-light.png">
</picture>

**Debug mode**: a placeholder marked as a false positive with a description, and the report ready to copy.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/screenshots/debug-dark.png">
  <img alt="Debug mode: the map with a Report column, [PHONE_1] marked false positive with the description 'our public support line, fine to keep', and the generated debug report below" src="docs/screenshots/debug-light.png" width="640">
</picture>

From the command line:

```bash
memento-redact-cli redact < app.log > redacted.txt
memento-redact-cli redact --conversation chat.json < app.log    # numbering continues across texts
memento-redact-cli restore --conversation chat.json < answer.txt
memento-redact-cli rules check                                  # rule warnings and sample lines
```

The CLI covers the Rules page and profiles too: export and import (all, some, or added to yours), reset one preset, check rules, apply a profile. See [docs/api.md](docs/api.md#cli).

## Limits

- It finds only what the rules describe. Names have no pattern and must be added by hand. Secrets without a key name or a known token format are not found. Read the redacted text before you send it.
- There is no login. Keep it on localhost, or put it behind a reverse proxy with authentication.

## Documentation

- [CONTRIBUTING.md](CONTRIBUTING.md): how to set up, test and send changes
- [docs/rules.md](docs/rules.md): the rule format, how matching and numbering work, what the presets cover, and how to write your own rules
- [docs/api.md](docs/api.md): the HTTP API and the CLI
- [PRODUCTION.md](PRODUCTION.md): running, settings, what is stored, security, backup, troubleshooting, tests, CI
- [docs/agentic/README.md](docs/agentic/README.md): code layout and conventions for maintainers and AI assistants

## Development

```bash
go run ./cmd/server                 # API on :8080; creates data/rulesets.yml from the presets
cd web && npm ci && npm run dev     # UI on :5173; sends API calls on to :8080
go test -race ./... && (cd web && npm test && npm run build)
```

The defaults work without any configuration. To change settings, see [PRODUCTION.md](PRODUCTION.md#settings). Contributions are welcome: see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
