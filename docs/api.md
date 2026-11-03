# API and CLI

The web UI and the CLI use the same HTTP API and the same redaction code. Anything the UI does, a script can do too.

## Basics

- Base URL: `http://localhost:8080`.
- There is no login. See [PRODUCTION.md](../PRODUCTION.md#security) before you expose the port.
- Request bodies over `MAX_PAYLOAD_SIZE` (default 1 MiB) get `413 payload too large`. They are never cut short.
- Errors come back as plain text with a 4xx or 5xx status.
- `POST` and `PUT` requests that a browser sends from another website get `403`. Scripts, curl and the CLI send no browser headers, so this check does not affect them.
- Requests addressed to a host name other than `localhost` get `403 host … is not allowed`, unless the name is listed in `ALLOWED_HOSTS`. Requests to IP addresses (`127.0.0.1`, `192.168.1.20`) always pass. This blocks DNS rebinding, where a website points its own domain at your machine.
- The server never stores or logs the text you send. It logs one line per request: method, path and duration.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | Liveness check: `{"status":"ok"}` |
| POST | `/v1/preview` | Redact text and return the map |
| GET | `/v1/rulesets` | Saved rulesets as JSON, or YAML with `?format=yaml` |
| PUT | `/v1/rulesets` | Replace all saved rulesets |
| GET | `/v1/rulesets/defaults` | Factory presets (read only) |
| POST | `/v1/rulesets/reset` | Replace all saved rulesets with the factory presets |

Unknown paths and wrong methods get `404` or `405`.

### POST /v1/preview

The request has two forms:

- **Any content type except `application/json`**: the body is the text, redacted with the saved rules.
- **`application/json`**: `{"text": "...", "rulesets": [...]}`. The given rulesets are checked and used but not saved. This is how the UI previews unsaved edits. If `rulesets` is missing or `null`, the saved rules are used. An empty list `[]` means no rules, so nothing is redacted.

```bash
curl -X POST --data 'ip 8.8.8.8, mail jane@corp.com' localhost:8080/v1/preview
```

```json
{
  "redacted": "ip [IP_1], mail [EMAIL_1]",
  "map": [
    {"token": "[IP_1]", "original": "8.8.8.8", "ruleset": "Network", "rule": "Public IPv4", "count": 1},
    {"token": "[EMAIL_1]", "original": "jane@corp.com", "ruleset": "Personal data", "rule": "Email", "count": 1}
  ]
}
```

- `map` has one entry per distinct value, in order of first appearance. `count` says how often the value occurs.
- `ruleset` and `rule` name the rule that matched first.
- Two entries can share a token. This happens with fixed placeholders and with *Number each part*, where `acme.sk` and `ACME.SK` both become `[DOMAIN_1].[TLD_1]`.
- `400`: invalid JSON, or an invalid rule. The message names the ruleset and the rule, e.g. `ruleset "S", rule "R": invalid regex "(": …`.
- If the client disconnects, the server stops redacting and sends nothing. This is how the UI's live preview drops runs it no longer needs.

### GET /v1/rulesets

Returns the saved rulesets as a JSON array; the format is described in [rules.md](rules.md#format). With `?format=yaml` it returns YAML, the same file that **Export** downloads.

### PUT /v1/rulesets

This replaces **all** saved rulesets. The body can be JSON or YAML; the content type is not checked. Every rule is checked first, and on any error you get `400` and nothing is saved. On success, the rulesets are written to `RULES_PATH`, and the response returns them as JSON in their cleaned-up form: words trimmed, empty patterns dropped, and `[REDACTED_#]` filled in where the placeholder was missing. You get `500` if the file cannot be written.

```bash
curl -s 'localhost:8080/v1/rulesets?format=yaml' > rules.yml
curl -X PUT --data-binary @rules.yml localhost:8080/v1/rulesets
```

### GET /v1/rulesets/defaults

Returns the factory presets, each stamped with its `preset` fingerprint. It changes nothing. The UI uses it for **Restore preset**, **Add preset…** and **Update available**.

### POST /v1/rulesets/reset

Replaces all saved rulesets, including your own, with the factory presets, then returns them. Export first if you want to keep a copy.

## CLI

`memento-redact-cli` is a small client for the API. It keeps no rules of its own.

```text
memento-redact-cli <command> [--api URL]

  health                          check the API
  redact  [--input TEXT]          print the redacted text (reads stdin without --input)
  preview [--input TEXT]          print redacted text and the token map as JSON
  rules list                      show rulesets and rules with their on/off state
  rules export                    print saved rulesets as YAML
  rules import [FILE|-]           replace all rulesets with a YAML/JSON file (or stdin)
  rules reset                     restore factory default rulesets
  rules enable|disable NAME       toggle a ruleset ("Network") or rule ("Network/Public IPv4")
```

- API URL: `--api`, else the `API_URL` environment variable, else `http://localhost:8080`.
- On an error it prints the server's message to stderr and exits with code 1. Requests time out after 20 seconds.
- `redact` prints only the redacted text, with no extra newline, so it works in pipes.
- `rules enable|disable` ignores case in names. It reads all saved rules, flips one switch and saves them all back. If the Rules page is open with unsaved edits, its next **Save** overwrites this change.

```bash
go build -o bin/memento-redact-cli ./cmd/cli
export API_URL=http://localhost:8080

memento-redact-cli redact < app.log > redacted.txt
memento-redact-cli preview --input "mail jane@corp.com"
memento-redact-cli rules disable "Network/Public IPv4"
memento-redact-cli rules export > rules.yml && $EDITOR rules.yml && memento-redact-cli rules import rules.yml
```

The Docker image includes the CLI:

```bash
docker compose exec memento-redact memento-redact-cli rules list
docker compose exec -T memento-redact memento-redact-cli redact < get-vm.txt
```
