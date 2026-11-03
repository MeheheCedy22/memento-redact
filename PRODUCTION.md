# memento-redact: running and testing

## Run

```bash
docker compose up -d --build        # or: docker build -t memento-redact . && docker run -d -p 127.0.0.1:8080:8080 -v memento-data:/app/data memento-redact
```

- UI and API on `http://localhost:8080` (one container; the Go server also serves the web UI).
- Rulesets: `/app/data/rulesets.yml` on the `memento-data` volume. It is seeded once from the factory presets in the image (`/app/config/default-rulesets.yml`), so image upgrades never overwrite your rules. New presets from an upgrade appear in **Rules → + Add factory preset…** (only if you have no ruleset with that name) or via **Reset all to factory defaults**.
- The container runs as an unprivileged user (uid 65534), has a healthcheck on `/health`, and needs no other services.
- Published images: `ghcr.io/<owner>/memento-redact:latest` (linux/amd64 and linux/arm64), built by CI from `main` and `v*` tags.

### Settings

Set in the shell or in `.env` next to `docker-compose.yml` (compose substitutes them):

| Variable | Default | |
|---|---|---|
| `PROVIDER_URL` | empty | upstream for `POST /v1/proxy` |
| `MAX_PAYLOAD_SIZE` | `1048576` | bytes; larger requests get HTTP 413, never silently truncated |
| `REQUEST_TIMEOUT_SECONDS` | `15s` | proxy upstream timeout (Go duration) |

Paths (`RULES_PATH`, `DEFAULT_RULES_PATH`, `STATIC_DIR`, `LISTEN_ADDR`) are fixed by the image; override them only when running the binary directly.

### Exposing it beyond localhost

There is no authentication, and the rules list exactly the names you want to hide. Keep the default `127.0.0.1` binding, or put the container behind a reverse proxy with authentication and TLS (Caddy, Traefik, nginx) before changing the port mapping. The clipboard button needs HTTPS or localhost; over plain HTTP use Ctrl+A, Ctrl+C in the redacted panel.

### Backup

Use **Rules → Export saved (YAML)**, or from the host: `docker compose exec memento-redact memento-redact-cli rules export > backup.yml`. Restore with **Import file…** or `docker compose exec -T memento-redact memento-redact-cli rules import < backup.yml`.

If the rules file is broken (e.g. edited by hand), the server refuses to start and names the problem, because running with no rules would pass text through unredacted. Fix the file or delete it to re-seed the presets.

## Test

```bash
go test ./...                       # rules, every factory preset, redaction, API
cd web && npm ci && npm run build   # type-check and build the UI
make docker                         # build the production image
```

Without Go or Node on the host:

```bash
docker compose -f docker-compose.test.yml run --rm backend-test
docker compose -f docker-compose.test.yml run --rm frontend-test
```

Smoke test a running instance:

```bash
curl localhost:8080/health
curl -X POST --data "ip 8.8.8.8, mail jane@corp.com" localhost:8080/v1/preview
```

## CI/CD

`.github/workflows/ci.yml`: gofmt, `go vet`, `go test -race`, frontend type-check and build, then the production image is built and smoke-tested (health, redaction, UI, CLI). On `main` and `v*` tags the image is pushed to GHCR for amd64 and arm64.
