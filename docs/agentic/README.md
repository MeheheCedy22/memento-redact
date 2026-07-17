# Agentic Notes

This directory contains non-sensitive operating notes for AI-assisted maintenance of `memento-redact`.

## Purpose

- Capture project conventions that are safe to share.
- Keep a short reference for build, test, and validation workflows.
- Document where rules, config, and production runtime settings live.

## Key Facts

- Backend: Go service in `cmd/server`.
- CLI: Go client in `cmd/cli`.
- Frontend: Vite + React app in `web`.
- Rules file: `config/rules.yml`.
- Runtime config: `.env` and `config/config.yml`.
- Production stack: `docker compose up --build` from the repo root.

## Validation Paths

- Go tests: `go test ./...`
- Backend container test: `docker compose --project-directory C:\Marek\Coding\memento-redact -f C:\Marek\Coding\memento-redact\docker-compose.test.yml run --rm backend-test`
- Frontend container test: `docker compose --project-directory C:\Marek\Coding\memento-redact -f C:\Marek\Coding\memento-redact\docker-compose.test.yml run --rm frontend-test`

## Ruleset Areas

- Generic identifiers and credentials.
- Java application launch flags.
- Tomcat and WildFly runtime secrets.
- Apache request headers and auth directives.
- MySQL, Oracle, LDAP, SSH, and PowerShell command-line secrets.

## Notes for Future Updates

- Keep any agent-specific notes non-sensitive.
- Never store secrets, tokens, or provider credentials here.
- Prefer short, explicit examples over large data dumps.
