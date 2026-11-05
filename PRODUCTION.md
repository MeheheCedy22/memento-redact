# Running memento-redact

## Run

With Docker Compose, from the repository:

```bash
docker compose up -d --build
```

With the published image (linux/amd64 and linux/arm64):

```bash
docker run -d --name memento-redact --restart unless-stopped \
  -p 127.0.0.1:8080:8080 -v memento-data:/app/data \
  ghcr.io/mehehecedy22/memento-redact:latest
```

Without Docker, from the repository root:

```bash
make build web && bin/memento-redact
```

Then open `http://localhost:8080`. One process serves the UI and the API. It needs no database and no other services. The container runs as an unprivileged user (uid 65534) and has a healthcheck on `/health`.

## What is stored where

| Data | Where | Kept |
|---|---|---|
| Your rules | `RULES_PATH`: `/app/data/rulesets.yml` on the `memento-data` volume | Until you change them. The file is written atomically (temp file, then rename), with mode `0600`. |
| Factory presets | `/app/config/default-rulesets.yml` inside the image | Replaced with every image upgrade |
| Text you redact | Server memory, while the request runs; the open browser tab | Never written to disk or logs. Gone when you close or reload the tab. |
| Answers you restore, and the conversation (the map of texts you copied) | The open browser tab only. The conversation goes to the server with each preview so numbering continues, and is not kept there. | Gone when you close or reload the tab, or click **New conversation**. |
| Theme choice, profiles, your own debug reasons | Browser `localStorage` | Until you clear site data |
| A CLI conversation (`--conversation FILE`) | The file you name, on the machine running the CLI, mode `0600`. It holds the real values. | Until you delete it |

The log has one line per request: method, path and duration. It never contains text or rules.

Your rules list exactly the names you want to hide, so treat the volume and every export like a secret.

## Upgrade

```bash
git pull && docker compose up -d --build              # from the repository
docker pull ghcr.io/mehehecedy22/memento-redact:latest   # published image, then recreate the container
```

On start, presets you have not edited are updated to the new factory version, and your on/off switches are kept. Presets you edited and your own rulesets are never overwritten. Edited presets show **Update available** on the Rules page. Details: [docs/rules.md](docs/rules.md#upgrades).


## Settings

All settings are environment variables. Values that are empty or invalid fall back to the default.

| Variable | Default (binary) | In the image | Format |
|---|---|---|---|
| `LISTEN_ADDR` | `:8080` | `:8080` | `host:port` |
| `RULES_PATH` | `data/rulesets.yml` | `/app/data/rulesets.yml` | file path; keep it on persistent storage |
| `DEFAULT_RULES_PATH` | `config/default-rulesets.yml` | `/app/config/default-rulesets.yml` | file path |
| `STATIC_DIR` | `web/dist` | `/app/web` | directory; if it is missing, only the API runs |
| `MAX_PAYLOAD_SIZE` | `1048576` | same | a whole number of bytes; anything else (`10MB`) logs a warning and uses the default |
| `ALLOWED_HOSTS` | empty | same | host names besides `localhost` you open the UI with, comma-separated: `redact.example.com,myhost.lan` |

- Docker Compose passes `MAX_PAYLOAD_SIZE` and `ALLOWED_HOSTS` from your shell, or from a `.env` file next to `docker-compose.yml` (see [.env.example](.env.example)). The paths are fixed in the image.
- When you run the binary directly, set variables in the shell: `LISTEN_ADDR=:9090 bin/memento-redact`.
- `MAX_PAYLOAD_SIZE` limits every request body: text to redact, the UI's preview (text plus your rules as JSON), and rule imports.

## Security

- **There is no login.** Compose binds the port to `127.0.0.1`, so only your own machine can reach it. Everyone who can reach the port can read and change your rules.
- **Other websites cannot change anything.** Cross-site `POST`/`PUT` requests from a browser are refused with `403` (Go's `http.CrossOriginProtection`).
- **Other websites cannot read your rules either.** The server answers only requests addressed to `localhost`, an IP address, or a name in `ALLOWED_HOSTS`. This blocks DNS rebinding, where a malicious site points its own domain at `127.0.0.1` and then reads the server as if it were its own.
- **Exposing it**: put it behind a reverse proxy with authentication and TLS before you change the port mapping, and add the name you open it with to `ALLOWED_HOSTS`. The **Copy** button needs HTTPS or localhost. Over plain HTTP, select the redacted text and press Ctrl+C.
- **Redaction finds only what the rules describe.** Names of people, customers and projects must be added by hand, and secrets without a key name or a known format are not found. Always read the redacted text before you send it anywhere.

## Backup and restore

To back up, use **Rules → Export**, or run on the host:

```bash
docker compose exec memento-redact memento-redact-cli rules export > backup.yml
```

To restore, use **Rules → Import…**, tick all rulesets, click **Replace all** and then **Save**. Or:

```bash
docker compose exec -T memento-redact memento-redact-cli rules import < backup.yml
```

## Troubleshooting

| Symptom | Cause and fix |
|---|---|
| Server exits at start with `rulesets …: …; fix the file, or delete it` | The rules file or the presets file is broken or missing. The server refuses to run without rules, because then it would pass text through unredacted. Fix the named line. Deleting the file brings back the presets but loses your rules. |
| `payload too large` | The text (in the UI: text plus rules) is over `MAX_PAYLOAD_SIZE`. Raise the limit, as a whole number of bytes. If the log says `is not a whole number of bytes`, your value was ignored. |
| `host … is not allowed; add it to ALLOWED_HOSTS` | You opened the UI by a name other than `localhost`, e.g. `http://myhost.lan:8080`. Add that name to `ALLOWED_HOSTS`, or use the IP address. |
| Log says `web UI not found` | `STATIC_DIR` has no built UI. Run `cd web && npm run build`, or use the image. |
| **Copy** says it needs HTTPS | The page is on plain HTTP and not on localhost. Use HTTPS or localhost, or select the text and press Ctrl+C. |
| A script gets `403 cross-origin request detected` | The script sends an `Origin` or `Sec-Fetch-Site` header from another site. Drop those headers. |
| Something is not redacted | Add a word or a rule; see [docs/rules.md](docs/rules.md#writing-your-own-rules). |
| Large pastes are slow | With the presets, the server needs about 2 seconds per MiB on a desktop CPU. The UI asks again when you pause typing, and the server stops the run it no longer needs. |

## Test

```bash
go test -race ./...                 # rules, every factory preset, redaction, API
cd web && npm ci && npm test        # debug report builder
npm run build                       # type-check and build the UI
make docker                         # build the production image
```

Without Go or Node on the host:

```bash
make test-docker                    # docker compose -f docker-compose.test.yml run --rm backend-test / frontend-test
```

Smoke test a running instance:

```bash
curl localhost:8080/health
curl -X POST --data "ip 8.8.8.8, mail jane@corp.com" localhost:8080/v1/preview
```

## CI/CD

[.github/workflows/ci.yml](.github/workflows/ci.yml) runs on every push and pull request. It checks `gofmt`, runs `go vet` and `go test -race`, and type-checks and builds the frontend. Then it builds the production image and smoke-tests it: health, redaction, UI and CLI.

On `main` and on `v*` tags, the image is pushed to `ghcr.io/mehehecedy22/memento-redact` for amd64 and arm64, with these tags:

- `latest` from `main`
- the version from `v*` tags (`v1.2.3` → `1.2.3`)
- `sha-<commit>`
