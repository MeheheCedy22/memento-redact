# memento-redact

Self-hosted LLM privacy proxy. Intercept, redact PII, and secure your prompts before they hit the cloud.

## Quick Start

1. Copy `.env.example` to `.env` and adjust values as needed.
2. Run the server:

```bash
go run ./cmd/server
```

3. Preview redaction through the API or CLI:

```bash
go run ./cmd/cli preview --input "contact alice@example.com"
```

4. Start the web UI:

```bash
cd web
npm install
npm run dev
```

## Runtime Settings

The proxy reads `.env` first, then `config/config.yml` for defaults. Editable rules live in `config/rules.yml`. The web UI reads `web/.env` or `web/.env.example` for `VITE_API_BASE_URL`.

## Endpoints

- `GET /health`
- `POST /v1/preview`
- `POST /v1/proxy`
- `GET /v1/rules`
- `PUT /v1/rules`
