# memento-redact

Self-hosted privacy proxy for LLMs. Paste text (logs, PowerCLI output, configs, emails), get a redacted copy that is safe to give to ChatGPT/Gemini/Claude, plus a map of what every placeholder stands for.

```
web01.prod.domain.tld  10.20.30.41  203.0.113.17  -Password 'S3cr3t!'  jane@domain.tld
[HOST_1].[DOMAIN].[TLD] 10.20.30.41 [IP_1]        -Password [SECRET_1] [EMAIL_1]
```

- **Identifiable placeholders**: each distinct value gets its own numbered token per kind (`[HOST_1]`, `[HOST_2]`, `[EMAIL_1]`); a value repeated in the text keeps its token, so the LLM can still reason about which host is which. Fixed placeholders such as `[DOMAIN].[TLD]` are possible too.
- **Rulesets with presets**: bundles (Organization, Network, Personal data, Secrets) you switch on/off as a whole or rule by rule. Rules are word/phrase lists or regexes, with an optional *keep* regex (e.g. private IPs stay as they are). The presets are based on what actually leaks; see [docs/agentic/rulesets.md](docs/agentic/rulesets.md).
- **Simple UI**: original and redacted text side by side in one scroll area, placeholders highlighted (hover shows the original), ruleset switches, the redaction map, and a Rules page. Light and dark mode.
- **Same features in CLI and API**; rules are stored on a persistent volume. Presets you have not edited update themselves when you upgrade.
- One small container (about 50 MB, amd64 and arm64), no database, runs on a Raspberry Pi.

## Quick start (Docker)

```bash
docker compose up -d --build
# open http://localhost:8080
```

Rules live in the `memento-data` volume and survive restarts and image upgrades. The port is bound to `127.0.0.1` because there is no login and your rules contain your sensitive names. See [PRODUCTION.md](PRODUCTION.md) to expose it.

## Using it

1. **Redact**: paste text into *Original*. *Redacted* updates as you type. Hover a highlighted placeholder to see what it replaced, then **Copy**.
2. Something is not redacted? On the **Rules** page put words or phrases one per line in *Organization → Names and codenames*, or add a regex rule. The preview applies your edits immediately.
3. **Save** stores the rules on the server; **Discard** goes back to the last save. **Restore preset** resets one ruleset, **Reset all** resets everything, **Export/Import** gives you a YAML backup.

First thing to do: on the **Rules** page, replace `domain.tld` in the *Organization* ruleset with your real domain (in both the *Hosts in our domain* regex and the *Company domain* words).

### Rule reference

| Field | Meaning |
|---|---|
| `type: words` | literal words/phrases, one per line, case-insensitive, whole words only |
| `type: regex` | Go RE2 regexes (no lookarounds). With a capture group, only the group is redacted: `password=(\S+)` gives `password=[SECRET_1]` |
| `placeholder` | `#` becomes a number per distinct value (`[HOST_#]`). Without `#`, every value gets the same fixed text |
| `keep` | optional regex; matched values that also match it are left untouched |

All rules run against the original text. When matches overlap, the longest wins. Values that only reference something (`$VAR`, `${VAR}`, `$(command)`, `%VAR%`, `{{ template }}`) are kept by the presets' *keep* regex, since the secret is not in the text; a default (`${VAR:-literal}`) or a literal echoed inside `$(echo …)` is still redacted. Factory presets: [config/default-rulesets.yml](config/default-rulesets.yml).

Presets are copied to your rules on first start. On every start, presets you have not edited are updated to the current factory version (your on/off switches are kept). Edited presets stay as they are and show **Update available** on the Rules page; **Restore preset** takes the new version.

## CLI

```bash
go build -o bin/memento-redact-cli ./cmd/cli     # also inside the container: docker compose exec memento-redact memento-redact-cli ...
export API_URL=http://localhost:8080

memento-redact-cli redact < get-vm.txt | wl-copy              # redacted text only, pipe-friendly
memento-redact-cli preview --input "mail jane@corp.com"       # JSON with redacted text and map
memento-redact-cli rules list
memento-redact-cli rules disable "Network/Public IPv4"        # or a whole ruleset: "Network"
memento-redact-cli rules export > rules.yml && $EDITOR rules.yml && memento-redact-cli rules import rules.yml
memento-redact-cli rules reset
```

## API

| Method | Path | |
|---|---|---|
| GET | `/health` | liveness |
| POST | `/v1/preview` | `text/plain` body uses saved rules; JSON `{"text": "...", "rulesets": [...]}` tests unsaved rules. Returns `{"redacted", "map": [{token, original, ruleset, rule, count}]}` |
| POST | `/v1/proxy` | redacts the body and forwards it to `PROVIDER_URL` (automatic mode, experimental) |
| GET | `/v1/rulesets` | saved rulesets as JSON, or YAML with `?format=yaml` |
| PUT | `/v1/rulesets` | replace all rulesets (JSON or YAML body); validated, then saved |
| GET | `/v1/rulesets/defaults` | factory presets |
| POST | `/v1/rulesets/reset` | restore factory presets |

## Development

```bash
cp .env.example .env
go run ./cmd/server                 # API on :8080, seeds data/rulesets.yml
cd web && npm ci && npm run dev     # UI on :5173, proxies the API to :8080
go test ./... && (cd web && npm run build)
```

Configuration is environment-only (`.env` is read at startup; real environment variables win). See [.env.example](.env.example).
