# memento-redact

Self-hosted privacy proxy for LLMs. Paste text such as logs, PowerCLI output, configs or emails. You get a redacted copy that is safe to give to ChatGPT, Gemini or Claude, plus a map of what every placeholder stands for.

```
web01.prod.domain.tld        10.20.30.41  203.0.113.17  -Password 'S3cr3t!'   jane@domain.tld
[HOST_1].[DOMAIN_1].[TLD_1]  10.20.30.41  [IP_1]        -Password [SECRET_1]  [EMAIL_1]
```

- **Placeholders you can tell apart**: every distinct value gets its own numbered token (`[HOST_1]`, `[HOST_2]`), and a repeated value keeps its token, so the LLM can still tell which host is which. Private IPs, key names and `$VARIABLES` stay readable.
- **Rulesets with presets**: Organization, Network, Personal data and Secrets, based on what actually leaks. You switch them on and off as a whole or rule by rule, and add your own words and regexes.
- **Simple UI**: the original and redacted text side by side, highlighted placeholders (hover shows the original), the map, a Rules page, light and dark mode.
- **CLI and HTTP API** with the same features, for pipes and scripts.
- **One small container** (about 35 MB, amd64 and arm64). No database. It runs on a Raspberry Pi.

## Quick start

```bash
docker compose up -d --build
# open http://localhost:8080
```

Rules live in the `memento-data` volume and survive restarts and upgrades. The port is bound to `127.0.0.1` because there is no login and your rules list your sensitive names.

**First step**: on the **Rules** page, replace `domain.tld` with your real domain in *Organization* (*Hosts in our domain* and *Company domain*), add names of people, customers and projects to *Names and codenames*, and click **Save**.

## Using it

1. **Redact**: paste text into *Original*. *Redacted* updates as you type, and the text itself is never stored. Hover a placeholder to see what it replaced. In the *Map*, click a placeholder to jump to it in the text; click again for the next occurrence. Then click **Copy**.
2. **Something is not redacted?** Add the word to *Organization → Names and codenames*, or add a regex rule on the **Rules** page. The preview uses your edits right away.
3. **Save** stores the rules on the server, for the CLI too. **Discard** goes back to the last save. **Restore preset** resets one ruleset, **Reset all** resets everything, and **Export** / **Import…** move the rules as YAML.

```bash
kubectl get pods -o wide | memento-redact-cli redact | wl-copy
```

## Limits

- It finds only what the rules describe. Names have no pattern and must be added by hand. Secrets without a key name or a known token format are not found. Read the redacted text before you send it.
- Answers from the LLM are not translated back. Use the map.
- Numbering starts again for every text. `[HOST_1]` in two separate pastes can be two different hosts.
- There is no login. Keep it on localhost, or put it behind a reverse proxy with authentication.
- It does not talk to any LLM itself. You copy the redacted text yourself.

## Documentation

- [docs/rules.md](docs/rules.md): the rule format, how matching and numbering work, what the presets cover, and how to write your own rules
- [docs/api.md](docs/api.md): the HTTP API and the CLI
- [PRODUCTION.md](PRODUCTION.md): running, settings, what is stored, security, backup, troubleshooting, tests, CI
- [docs/agentic/README.md](docs/agentic/README.md): code layout and conventions for maintainers and AI assistants

## Development

```bash
go run ./cmd/server                 # API on :8080; creates data/rulesets.yml from the presets
cd web && npm ci && npm run dev     # UI on :5173; sends API calls on to :8080
go test -race ./... && (cd web && npm run build)
```

The defaults work without any configuration. To change settings, see [PRODUCTION.md](PRODUCTION.md#settings).

MIT License.
