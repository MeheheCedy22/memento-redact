# Agentic Notes

These are operating notes for AI-assisted maintenance of `memento-redact`. Never store secrets, tokens, or real rule contents here.

## Layout

- `cmd/server`: the HTTP server. It serves the API and the built UI (`STATIC_DIR`). On start it runs `rules.Sync`, and it exits if the rules do not load.
- `cmd/cli`: the API client (`health`, `redact`, `preview`, `restore`, `rules list|export|import|reset|check|profile|enable|disable`). `parse` lets flags come after arguments. `restore.go` is a Go copy of `web/src/restore.ts` (restoring never goes through the server): change both, and their tests, together. Profiles (`profileOf`, `applyProfile`) use the same JSON as `web/src/rulesets.ts`.
- `config`: `config.Load()` reads settings from environment variables only; `default-rulesets.yml` holds the factory presets.
- `internal/rules`: the Ruleset/Rule model, validation (`Compile`), matching (`Find`), YAML load and atomic save, and `Sync`. `Sync` seeds the rules on first start and upgrades unedited presets through the `preset` fingerprint; on/off switches and `samples` are not part of the fingerprint. `lint.go`: `Lint`, the rule warnings (greedy `.*`, missing `\b`, redacts ordinary text). `random.go`: `looksRandom` for rules with `random: true`.
- `internal/redactor`: `Redact` runs all enabled rules on the original text. The longest match wins on overlaps. It numbers tokens per placeholder, ignoring case (`partNumbering` for `per_part`), and builds the map. `known` entries (an earlier map) keep their tokens and numbering continues after them; each entry carries its `placeholder` and `numbers` for that. It stops with an error when the request context ends. `leaks` finds redacted values still left in the text, through one suffix array over the unredacted parts (a scan per value took seconds on large logs).
- `internal/server`: the handlers (including `POST /v1/rulesets/parse`, which checks a file without saving; `/v1/preview` also returns `warnings` and `leaks`), the body size limit, `securityHeaders` (CSP, no framing), `allowHosts` (blocks DNS rebinding) and `http.CrossOriginProtection` (blocks CSRF).
- `web/`: Vite, React and TypeScript, with no other runtime dependencies.
  - `App.tsx`: draft and saved rules, the live preview, the header.
  - `Workspace.tsx`: the Redact page, and the Restore page (same layout; the text pair is `Restore.tsx`).
  - `restore.ts`: puts real values back into an LLM answer, and `remember` adds a copied text's map to the conversation (kept in `App.tsx`, memory only, sent as `known` with each preview).
  - `Numbered.tsx`: line numbers for the text pair and the Patterns fields (an invisible copy of the textarea text carries the numbers, so wrapped lines keep theirs), and `highlight`, which marks placeholders and outlines leaks.
  - `mapsort.ts`: sorting the map table by a column.
  - `clipboard.ts`: copying with a visible note on failure.
  - `RulesEditor.tsx`: the Rules page, with `Samples` (each rule's sample lines, redacted by that rule alone through `/v1/preview`).
  - `debug.ts`: builds the debug report from the marked map entries and missed texts.
  - `rulesets.ts`: merging an import into the rules, and profiles (saved on/off switches, matched by name).
  - `storage.ts`: the browser-only settings (saved debug reasons, profiles, text box height) in `localStorage`.
  - `*.test.ts`: tests for the pure modules above (`npm test`, Node's built-in runner, excluded from `tsc`). Tested modules import each other with the `.ts` extension, because Node needs it (`allowImportingTsExtensions` in `tsconfig.json`).
  - `AutoTextarea.tsx`: form fields that grow with their content.
  - `theme.ts` and the CSS variables: light and dark mode.

## Docs

- `README.md`: why to use it, features, quick start, limits.
- `CONTRIBUTING.md`: setup, checks and rules for changes; keep it in line with the conventions below.
- `docs/rules.md`: rule format and semantics, presets. Update it when matching or numbering changes.
- `docs/api.md`: endpoints and the CLI. Update it when a handler or a command changes.
- `PRODUCTION.md`: running, settings table, storage, security, troubleshooting. Update the settings table when `config.Load` changes.
- `docs/screenshots/`: README images, each in light and dark. Take them from a fresh instance (factory presets, only the built-in **Example** text, 1400 px wide) and retake them after visible UI changes. Never screenshot an instance with real rules.

## Validation

- `gofmt -l .` (must print nothing), `go vet ./...`, `go test -race ./...`
- `cd web && npm test && npm run build` (the build runs `tsc --noEmit`)
- `docker build -t memento-redact:local .`, then a smoke test as in `.github/workflows/ci.yml`

## Conventions

- Every factory preset rule needs a case in `TestDefaultPresets`, and the presets must have no rule warnings (`TestDefaultPresetsHaveNoWarnings`).
- A UI feature gets a CLI counterpart when it makes sense outside a browser (see `docs/api.md`).
- The UI and the CLI must redact the same way: both go through `/v1/preview` and `redactor.Redact`.
- Never return a partly redacted text: `Redact` returns an error instead.
- The server never stores or logs input text. Only rules are stored.
- Never use real domains, company names or internal system names in code, tests or docs. Use `domain.tld`, `acme.sk` and `contoso.com`.
- Keep the UI same-origin, with no CDN assets and minimal dependencies (React only).
- Keep the UI style plain and quiet: system font, 1px borders, one accent colour, no gradients. Keep hints short and exact. Form fields grow with their content. Each big view (the original/redacted pair, the rulesets, the map) scrolls as one area.
