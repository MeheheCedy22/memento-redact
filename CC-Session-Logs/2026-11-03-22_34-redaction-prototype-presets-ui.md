# Session Log: 2026-11-03 22:34 - redaction-prototype-presets-ui

## Quick Reference (for AI scanning)
**Confidence keywords:** memento-redact, MeheheCedy22, Go, React, Vite, redaction, rulesets, presets, placeholders, per_part, rules.Sync, preset fingerprint, keep regex, gitleaks, detect-secrets, GitGuardian, JIRA_PASS, shell expansion false positive, CrossOriginProtection, Docker, GHCR, puppeteer-core e2e
**Projects:** memento-redact (github.com/MeheheCedy22/memento-redact)
**Outcome:** Built a working self-hosted LLM redaction proxy prototype (Go API + CLI + React UI, one Docker image) with research-based presets, auto-upgrading presets, reference-aware rules, per-part domain numbering and a simple light/dark UI; all tests pass.

## Decisions Made
- Single container: Go server serves API and built UI (`STATIC_DIR`); nginx + separate frontend image removed (simpler, light HW).
- Config from env only (`config.Load()`), `config.yml` deleted; real env vars win over `.env`.
- Rules model: rulesets (bundles) of rules; `type: words|regex`, `patterns[]`, `keep` regex, `placeholder` with `#` numbering, `per_part`, ruleset `preset` fingerprint.
- Redaction: all rules run on original text, longest overlapping match wins (ties: earlier rule), first capture group is the redacted part, same value -> same token, numbering by appearance.
- Presets auto-upgrade on start (`rules.Sync`): unedited presets replaced by new factory version keeping on/off switches; edited/unstamped left alone, UI shows "Update available".
- Values that only reference something (`$VAR`, `${VAR}`, `$(cmd)`, `` `cmd` ``, `%VAR%`, `$env:VAR`, `{{ }}`) are kept via shared `keep: &refs` YAML anchor; `${VAR:-literal}` and `$(echo literal)` still redacted.
- UI (user's answers): keep React+Vite, drop Monaco and react-grid-layout, fixed layout, auto-growing fields, no diff view (highlight placeholders, hover shows original), light/dark switch, plain non-"AI look" style.
- Original/Redacted pair is one shared scroll area (75vh); rulesets and map scroll (55vh); map placeholders jump to text.
- Input text never stored/logged; port bound to 127.0.0.1 (no auth); `http.CrossOriginProtection` against CSRF.
- Go module renamed to `github.com/MeheheCedy22/memento-redact`; never put user's real domains in repo (use acme.sk / contoso.com).

## Solutions & Fixes
- `JIRA_PASS=...` not redacted: keyword list lacked `pass`; then still not redacted because saved rules on volume were an old copy of presets -> added fingerprint stamp + `rules.Sync` auto-upgrade.
- `DB_USER=$(jq ...)` redacted as `[USER_1]`: added refs keep regex to all value-capturing rules.
- XML config secrets not caught: patterns for `<entry key="dbPassword" value="..."/>`, `key="..">x</entry>`, `<password>x</password>`.
- Private-key regex merged consecutive key blocks: label class `[ A-Z0-9_-]` -> `[ A-Z0-9_]`.
- YAML parse error from unquoted `: ` in a description: quote it.
- @monaco-editor/react DiffEditor "TextModel got disposed" on unmount (before Monaco was removed): fixed model paths + keepCurrent*Model.
- Silent truncation of large bodies: `http.MaxBytesReader` -> 413.
- Broken rules file meant "redact nothing": server now refuses to start.
- Domain distinction: `per_part` numbers each dot part (`acme.sk` -> `[DOMAIN_1].[TLD_1]`, `acme.com` -> `[DOMAIN_1].[TLD_2]`).

## Files Modified
- `internal/rules/rules.go`: Ruleset/Rule model, Compile/Find, Parse (YAML+JSON), atomic Save, LoadDefaults, Sync, fingerprint, PerPart field.
- `internal/redactor/redactor.go`: span selection, token numbering, `partNumbering`, map entries.
- `internal/server/server.go`: endpoints `/health`, `/v1/preview` (text or JSON with draft rulesets), `/v1/proxy`, `/v1/rulesets` (GET ?format=yaml, PUT), `/v1/rulesets/defaults`, `/v1/rulesets/reset`, static UI, CSRF protection.
- `cmd/server/main.go`, `cmd/cli/main.go` (`health|redact|preview|proxy|rules list|export|import|reset|enable|disable`), `config/config.go`, `internal/dotenv/dotenv.go`.
- `config/default-rulesets.yml`: presets Organization, Network, Personal data, Secrets (research-based); tests in `internal/rules/rules_test.go` (`TestDefaultPresets` requires a case per preset rule).
- `web/src/`: `App.tsx`, `Workspace.tsx`, `RulesEditor.tsx`, `AutoTextarea.tsx`, `theme.ts`, `api.ts`, `styles.css`, `main.tsx`; removed `monaco.ts`, web Dockerfile/nginx.
- `Dockerfile` (multi-stage, cross-compile amd64/arm64, uid 65534), `docker-compose.yml`, `docker-compose.test.yml`, `.dockerignore`, `.github/workflows/ci.yml`, `Makefile`, `.env.example`, `.gitignore`, `README.md`, `PRODUCTION.md`, `docs/agentic/README.md`, `docs/agentic/rulesets.md`.

## Pending Tasks
- [ ] Commit current working tree (per_part + map jump) with the last provided message (user commits manually; never commit for them).
- [ ] User: `cp .env.example .env` (old `.env` points to deleted `config/rules.yml`; overwrite was blocked).
- [ ] User: `docker compose up -d --build`, then Restore preset on unedited presets; tick "Number each part" on Company domain and Save.
- [ ] Not built: authentication, de-redaction/restore of LLM answers via map, numbering consistency across separate pastes, proper automatic proxy (`/v1/proxy` is naive passthrough), original-side highlight for map jump.
- [ ] Local machine has no `docker compose` plugin (tested with plain `docker run`).

---

## Custom Notes
None

---

## Quick Resume Context
memento-redact is a working prototype: Go server (API + serves `web/dist`), Go CLI, React+Vite UI, presets in `config/default-rulesets.yml` synced to `RULES_PATH` by `rules.Sync` with fingerprint-based auto-upgrade. Last commit is `c070346`; uncommitted work adds `per_part` domain/TLD numbering and map-to-text jump links (commit message already drafted). Verify with `go test -race ./...`, `cd web && npm run build`, and the puppeteer e2e script in the session scratchpad; the user commits manually and wants plain, uncluttered UI and no real company names in the repo.
