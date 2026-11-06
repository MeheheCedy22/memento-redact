# Contributing

Thanks for helping. Bug reports, new preset rules, fixes and features are all welcome.

## Report

- **A bug or an idea**: open an issue. For a wrong redaction, use **Debug** in the *Map* and paste the report, after replacing every real value in it with an invented one.
- **A security problem**: do not open a public issue. Use *Security → Report a vulnerability* on GitHub.

## Set up

You need Go (the version in [go.mod](go.mod)) and Node 24.

```bash
go run ./cmd/server                 # API on :8080; creates data/rulesets.yml from the presets
cd web && npm ci && npm run dev     # UI on :5173; sends API calls on to :8080
```

[docs/agentic/README.md](docs/agentic/README.md) explains the code layout.

## Check

Run these before you send a change; CI runs the same:

```bash
gofmt -l .                          # must print nothing
go vet ./... && go test -race ./...
cd web && npm test && npm run build
```

No Go or Node on your machine: `make test-docker`.

## Rules for changes

- **No real data, ever.** Tests, docs, examples and screenshots use invented values only: `domain.tld`, `acme.sk`, `contoso.com`, `jane@example.com`, `8.8.8.8`, `hunter2`. Never copy values from a real log, a bug report or a screenshot.
- **Preset rules** ([config/default-rulesets.yml](config/default-rulesets.yml)): every rule needs a case in `TestDefaultPresets`, and the presets must have no [rule warnings](docs/rules.md#warnings) (`TestDefaultPresetsHaveNoWarnings`). Say where a pattern comes from when it is adapted from another project.
- **The UI and the CLI do the same things.** A new feature in the UI gets a CLI counterpart when it makes sense outside a browser, and the other way round. Redaction always goes through `/v1/preview`.
- **Restoring answers** exists twice, in [web/src/restore.ts](web/src/restore.ts) and [cmd/cli/restore.go](cmd/cli/restore.go). Change both, and both tests.
- **Small and plain.** No new runtime dependencies without a strong reason. The UI stays quiet: short texts, no decoration.
- **Docs in the same change.** Update [docs/rules.md](docs/rules.md) for rule behaviour, [docs/api.md](docs/api.md) for endpoints and CLI commands, [PRODUCTION.md](PRODUCTION.md) for settings and storage, and the README for visible features. Retake the [screenshots](docs/screenshots) after visible UI changes, from a fresh instance with the factory presets and the built-in **Example** text, in light and dark.

## Send

Fork, make a branch, and open a pull request against `main` that says what changed and why. Keep one topic per pull request. Contributions are under the [MIT License](LICENSE).
