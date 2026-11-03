# Agentic Notes

Non-sensitive operating notes for AI-assisted maintenance of `memento-redact`. Never store secrets, tokens, or real rule contents here.

## Layout

- `cmd/server`: HTTP server; serves the API and the built UI (`STATIC_DIR`).
- `cmd/cli`: API client (`redact`, `preview`, `proxy`, `rules list|export|import|reset|enable|disable`).
- `internal/rules`: Ruleset/Rule model, validation (`Compile`), matching (`Find`), YAML load/save (atomic writes), `Sync` (seed on first start, auto-upgrade unedited presets via the `preset` fingerprint).
- `internal/redactor`: runs all enabled rules on the original text, longest match wins on overlaps, numbers tokens per placeholder, builds the map.
- `internal/server`: handlers; refuses to start on an invalid rules file.
- `config/default-rulesets.yml`: factory presets, copied to `RULES_PATH` on first start. Every preset rule must have a case in `TestDefaultPresets`.
- `web/`: Vite + React + TypeScript, no other runtime dependencies. `Workspace.tsx` (Redact page), `RulesEditor.tsx` (Rules page), `App.tsx` (draft vs saved rules, live preview, header), `AutoTextarea.tsx` (fields grow with content), `theme.ts` + CSS variables (light/dark).
- Config: environment variables only (`.env.example`); real env wins over `.env`.

## Validation

- `gofmt -l .` (must be empty), `go vet ./...`, `go test ./...`
- `cd web && npm run build` (runs `tsc --noEmit`)
- `docker build -t memento-redact:local .` and a smoke test as in `.github/workflows/ci.yml`

## Conventions

- Keep the UI same-origin with no CDN assets, and keep dependencies minimal (React only).
- UI style: plain and quiet (system font, 1px borders, one accent colour, no gradients); hints short and exact; form fields grow with their content; big views (the original/redacted pair, rulesets, map) scroll as one area each.
- Input text is never persisted or logged; only rules are stored.
- Rule changes must keep the preview/CLI/proxy behaviour identical: all three use `redactor.Redact`.
