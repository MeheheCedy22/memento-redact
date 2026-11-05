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
| GET | `/v1/rulesets` | Saved rulesets as JSON, or YAML with `?format=yaml`; one ruleset with `?name=` |
| PUT | `/v1/rulesets` | Replace all saved rulesets |
| POST | `/v1/rulesets/parse` | Check a rules file and return it as JSON, without saving |
| GET | `/v1/rulesets/defaults` | Factory presets (read only) |
| POST | `/v1/rulesets/reset` | Replace all saved rulesets with the factory presets |

Unknown paths and wrong methods get `404` or `405`.

### POST /v1/preview

The request has two forms:

- **Any content type except `application/json`**: the body is the text, redacted with the saved rules.
- **`application/json`**: `{"text": "...", "rulesets": [...], "known": [...]}`. The given rulesets are checked and used but not saved. This is how the UI previews unsaved edits. If `rulesets` is missing or `null`, the saved rules are used. An empty list `[]` means no rules, so nothing is redacted.
- `known` (optional) is the `map` of earlier texts in the same conversation. Their values keep their tokens, and new values are numbered after them. The UI sends the map of every text you copied; the server keeps nothing between requests.

```bash
curl -X POST --data 'ip 8.8.8.8, mail jane@corp.com' localhost:8080/v1/preview
```

```json
{
  "redacted": "ip [IP_1], mail [EMAIL_1]",
  "map": [
    {"token": "[IP_1]", "original": "8.8.8.8", "ruleset": "Network", "rule": "Public IPv4", "count": 1, "placeholder": "[IP_#]", "numbers": [1]},
    {"token": "[EMAIL_1]", "original": "jane@corp.com", "ruleset": "Personal data", "rule": "Email", "count": 1, "placeholder": "[EMAIL_#]", "numbers": [1]}
  ]
}
```

- `map` has one entry per distinct value, in order of first appearance. `count` says how often the value occurs.
- `ruleset` and `rule` name the rule that matched first.
- `placeholder` is the rule's placeholder, and `numbers` the numbers put into it (one per part with *Number each part*, none for a fixed placeholder). They let `known` continue the numbering.
- Two entries can share a token. This happens with fixed placeholders (no `#`), where every value gets the same text.
- `400`: invalid JSON, or an invalid rule. The message names the ruleset and the rule, e.g. `ruleset "S", rule "R": invalid regex "(": …`.
- If the client disconnects, the server stops redacting and sends nothing. This is how the UI's live preview drops runs it no longer needs.

### GET /v1/rulesets

Returns the saved rulesets as a JSON array; the format is described in [rules.md](rules.md#format). With `?format=yaml` it returns YAML, the same file that **Export** downloads. With `?name=Network` it returns only that ruleset (any case), still as a list, so the file can be imported; `404` if there is none. That is the file a ruleset's own **Export** downloads.

### PUT /v1/rulesets

This replaces **all** saved rulesets. The body can be JSON or YAML; the content type is not checked. Every rule is checked first, and on any error you get `400` and nothing is saved. On success, the rulesets are written to `RULES_PATH`, and the response returns them as JSON in their cleaned-up form: words trimmed, empty patterns dropped, and `[REDACTED_#]` filled in where the placeholder was missing. You get `500` if the file cannot be written.

```bash
curl -s 'localhost:8080/v1/rulesets?format=yaml' > rules.yml
curl -X PUT --data-binary @rules.yml localhost:8080/v1/rulesets
```

### POST /v1/rulesets/parse

Checks a rules file (JSON or YAML, the content type is not checked) exactly like `PUT /v1/rulesets`, and returns its rulesets as JSON in their cleaned-up form. Nothing is saved. The UI's **Import…** uses it to list a file's rulesets before you pick from them. `400` names the first invalid ruleset and rule.

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
          [--rules FILE]          redact with a rules file instead of the saved rules
          [--conversation FILE]   keep the placeholders of earlier texts, add this text's values
  preview [same flags]            print redacted text and the token map as JSON
  restore --conversation FILE     put the real values back into an LLM answer (stdin or --input)
  rules list                      show rulesets and rules with their on/off state and updates
  rules export [NAME]             print saved rulesets, or only ruleset NAME, as YAML
  rules import [--add] [FILE|-]   replace all rulesets with a YAML/JSON file (or stdin);
                                  --add adds them instead (same name replaces yours)
  rules reset [NAME]              restore all factory presets, or only preset NAME
  rules check [FILE]              show each rule's sample lines redacted by that rule alone
  rules enable|disable NAME       toggle a ruleset ("Network") or rule ("Network/Public IPv4")
```

- API URL: `--api`, else the `API_URL` environment variable, else `http://localhost:8080`.
- On an error it prints the server's message to stderr and exits with code 1. Requests time out after 20 seconds.
- `redact` prints only the redacted text, with no extra newline, so it works in pipes.
- `--rules FILE` tries an edited rules file before you import it; the saved rules are not used.
- `--conversation FILE` is the CLI's conversation: the first `redact` creates the file, later ones keep its placeholders (`web01` stays `[HOST_1]`) and add their new values. `restore` reads it. The file holds the real values and is written with mode `0600`; delete it when the conversation is over.
- `restore` works like the Restore page and runs locally: the answer is never sent to the server. It prints the restored answer; placeholders it leaves as they are (one that stands for several values, or a numbered one not in the file) are listed on stderr.
- `rules list` marks an edited preset whose factory version changed with *(update available)*, like the Rules page.
- `rules export NAME` prints one ruleset (any case). `rules import` replaces **all** rulesets; `rules import --add` adds the file's rulesets instead, and one with the same name replaces yours (Import… → Add).
- `rules reset NAME` replaces one ruleset with its factory version, or adds it back if you deleted it (Restore preset, Add preset…). Your edits in it, sample lines included, are lost.
- `rules check` shows the sample lines of every rule that has them, each redacted by that rule alone, and marks lines it left as they were with *(not redacted)*. With a FILE it checks that file instead of the saved rules.
- `rules enable|disable` ignores case in names. It reads all saved rules, flips one switch and saves them all back. If the Rules page is open with unsaved edits, its next **Save** overwrites this change.

```bash
go build -o bin/memento-redact-cli ./cmd/cli
export API_URL=http://localhost:8080

memento-redact-cli redact < app.log > redacted.txt
memento-redact-cli preview --input "mail jane@corp.com"
memento-redact-cli rules disable "Network/Public IPv4"
memento-redact-cli rules export > rules.yml && $EDITOR rules.yml && memento-redact-cli rules check rules.yml && memento-redact-cli rules import rules.yml

# a conversation: numbering continues across texts, then the LLM's answer gets the real values back
memento-redact-cli redact --conversation chat.json < app.log > redacted.txt
memento-redact-cli redact --conversation chat.json < config.yml > redacted-config.txt
memento-redact-cli restore --conversation chat.json < answer.txt
rm chat.json
```

The Docker image includes the CLI:

```bash
docker compose exec memento-redact memento-redact-cli rules list
docker compose exec -T memento-redact memento-redact-cli redact < get-vm.txt
```
