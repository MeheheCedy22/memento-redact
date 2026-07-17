# memento-redact Production and Test Guide

This guide covers how to test `memento-redact` without installing Node or npm on the host, and how to run it as a production-style container stack.

## Prerequisites

- Docker Desktop or Docker Engine with `docker compose`
- No local Go or Node installation is required for the Docker-based test flow

## Configuration

1. Copy `.env.example` to `.env` and adjust values for your environment.
2. Edit `config/config.yml` for default runtime settings if you want file-based overrides.
3. Edit `config/rules.yml` to change the redaction rules used by the proxy and UI.

## Test In Docker

Run the backend unit and integration-style tests in a container:

```bash
docker compose --project-directory C:\Marek\Coding\memento-redact -f C:\Marek\Coding\memento-redact\docker-compose.test.yml run --rm backend-test
```

Run the frontend build validation in a container:

```bash
docker compose --project-directory C:\Marek\Coding\memento-redact -f C:\Marek\Coding\memento-redact\docker-compose.test.yml run --rm frontend-test
```

Run both in sequence:

```bash
docker compose --project-directory C:\Marek\Coding\memento-redact -f C:\Marek\Coding\memento-redact\docker-compose.test.yml run --rm backend-test
docker compose --project-directory C:\Marek\Coding\memento-redact -f C:\Marek\Coding\memento-redact\docker-compose.test.yml run --rm frontend-test
```

## Run As Production

Bring up the backend and frontend stack:

```bash
docker compose --project-directory C:\Marek\Coding\memento-redact up --build
# or
docker compose up --build
```

After startup:

- Backend API: `http://localhost:8080`
- Frontend UI: `http://localhost:3000`
- The browser UI talks to the backend through the same-origin `/api` path, which nginx proxies to the backend container.
- The backend container runs from `/app`, where the mounted config and rules files are available.

## Health Checks

Backend health check:

```bash
curl http://localhost:8080/health
```

Preview a prompt with the API:

```bash
curl -X POST http://localhost:8080/v1/preview -H "Content-Type: text/plain; charset=utf-8" --data "contact alice@example.com"
```

## CLI Usage

Build the CLI binary:

```bash
go build -o bin/memento-redact-cli ./cmd/cli
```

Preview a prompt through the API with the CLI:

```bash
bin/memento-redact-cli preview --input "contact alice@example.com"
```

## Operational Notes

- The proxy forwards the redacted payload to the provider, not the raw input.
- The UI and CLI both use the same backend API, so rule changes apply everywhere.
- Keep redaction rules deterministic and review `config/rules.yml` before production use.

## Troubleshooting

- If Docker cannot find the compose file, pass `--project-directory` as shown above.
- If the frontend UI cannot reach the API, confirm `web/nginx.conf` still proxies `/api/` to `http://backend:8080/` and that the frontend was built with `VITE_API_BASE_URL=/api`.
- If the Rules editor shows an empty list, confirm the backend is starting in `/app` and that `/app/config/rules.yml` exists in the running backend container.
- If provider requests fail, set `PROVIDER_URL` in `.env` and confirm the upstream service is reachable.