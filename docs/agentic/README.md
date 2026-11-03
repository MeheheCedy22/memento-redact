# Agentic Notes

Non-sensitive operating notes for AI-assisted maintenance of `memento-redact`. Never store secrets, tokens, or real rule contents here.

## Layout

- `cmd/server`: HTTP server; serves the API and the built UI (`STATIC_DIR`).
- `cmd/cli`: API client (`redact`, `preview`, `proxy`, `rules list|export|import|reset|enable|disable`).
- `internal/rules`: Ruleset/Rule model, validation (`Compile`), matching (`Find`), YAML load/save (atomic writes), seeding.
- `internal/redactor`: runs all enabled rules on the original text, longest match wins on overlaps, numbers tokens per placeholder, builds the map.
- `internal/server`: handlers; refuses to start on an invalid rules file.
- `config/default-rulesets.yml`: factory presets, copied to `RULES_PATH` on first start. Every preset rule must have a case in `TestDefaultPresets`.
- `web/`: Vite + React + TypeScript. `Workspace.tsx` (react-grid-layout panels, Monaco editors bundled locally), `RulesEditor.tsx`, `App.tsx` (draft vs saved rules, live preview).
- Config: environment variables only (`.env.example`); real env wins over `.env`.

## Validation

- `gofmt -l .` (must be empty), `go vet ./...`, `go test ./...`
- `cd web && npm run build` (runs `tsc --noEmit`)
- `docker build -t memento-redact:local .` and a smoke test as in `.github/workflows/ci.yml`

## Conventions

- Keep the UI same-origin; no CDN assets (Monaco is bundled, never loaded from a CDN).
- Input text is never persisted or logged; only rules are stored.
- Rule changes must keep the preview/CLI/proxy behaviour identical: all three use `redactor.Redact`.
