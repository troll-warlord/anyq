# anyq

> **Query any config file in plain English. No jq required.**

[![CI](https://github.com/troll-warlord/anyq/actions/workflows/ci.yml/badge.svg)](https://github.com/troll-warlord/anyq/actions/workflows/ci.yml)
[![Release](https://github.com/troll-warlord/anyq/actions/workflows/release.yml/badge.svg)](https://github.com/troll-warlord/anyq/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/troll-warlord/anyq)](https://goreportcard.com/report/github.com/troll-warlord/anyq)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

`anyq` lets you query JSON, YAML, and TOML files using **natural language** — powered by your choice of AI provider. Describe what you want to extract; `anyq` writes the query and runs it. It also works as a conventional `jq`/`yq`/`tomlq` replacement for those who prefer writing expressions directly.

---

## AI-Powered Querying

The flagship feature. Instead of learning jq syntax, just describe what you want:

```bash
# Set your AI provider once
export ANYQ_AI_PROVIDER=openai   # or: anthropic, gemini, ollama
export OPENAI_API_KEY=sk-...

# Then query in plain English
anyq --ai "names of all admin users" users.json
anyq --ai "which services are disabled" config.yaml
anyq --ai "list all dev dependencies" package.json
anyq --ai "database host and port" app.toml
```

`anyq` extracts the **schema** of your file (key names and types only — no actual values leave your machine), asks the AI to write a jq expression, validates it compiles, then runs it locally against your real data.

```
your file ──► schema extraction ──► AI (keys + types only) ──► jq expression
                                                                      │
your file ◄────────────────────────────────────────────── run locally ┘
```

### Preview the generated query

Use `--show-query` to inspect (and optionally cancel) the generated expression before it runs:

```bash
anyq --ai "count of pending orders" --show-query orders.json
```
```
✦ Generated query: [.orders[] | select(.status == "pending")] | length
Run? [Y/n]:
```

### Supported AI providers

| Provider | Env var | Default model |
|---|---|---|
| `openai` (default) | `OPENAI_API_KEY` | `gpt-4o-mini` |
| `anthropic` | `ANTHROPIC_API_KEY` | `claude-3-5-haiku-20241022` |
| `gemini` | `GEMINI_API_KEY` | `gemini-2.0-flash` |
| `ollama` (local, free) | `OLLAMA_BASE_URL` (default: `http://localhost:11434`) | `qwen2.5-coder` |

Override the model for any provider with `ANYQ_AI_MODEL`:

```bash
ANYQ_AI_MODEL=gpt-4o anyq --ai "summarize the pipeline stages" .github/workflows/ci.yml
```

### Privacy

`anyq` never sends your file's values to the AI. It only sends a structural schema like:

```json
{
  "users": [{ "name": "string", "age": "number", "role": "string" }]
}
```

Sensitive field names (`password`, `token`, `secret`, `key`, etc.) are automatically replaced with `"<redacted>"`.

---

## Installation

### Pre-built binary (recommended)

```bash
# Linux / macOS
curl -sSfL \
  https://github.com/troll-warlord/anyq/releases/latest/download/anyq_linux_amd64.tar.gz \
  | tar -xz anyq && sudo mv anyq /usr/local/bin/
```

### Homebrew (macOS / Linux)

```bash
brew install troll-warlord/tap/anyq
```

### Go install

```bash
go install github.com/troll-warlord/anyq@latest
```

### Docker

```bash
docker run --rm ghcr.io/troll-warlord/anyq:latest --ai "database host" < config.yaml
```

---

## Usage

```
anyq [flags] [jq-expression] [file...]
anyq diff [flags] <file1> <file2>
anyq validate [flags] --schema <schema.json> <file>
```

### All flags

| Flag | Short | Description |
|---|---|---|
| `--ai` | | Natural language query — translated to jq by AI |
| `--show-query` | | Print the AI-generated query and prompt before running |
| `--input-format` | `-f` | Force input format: `json`, `yaml`, `toml` |
| `--output-format` | `-o` | Output format: `json`, `yaml`, `toml` (default: same as input) |
| `--pretty` | | Pretty-print output (default: `true`) |
| `--raw-output` | `-r` | Print strings without surrounding quotes |
| `--compact` | `-c` | Compact JSON output (no whitespace) |
| `--slurp` | | Read all inputs into an array, then run expression once |
| `--null-input` | `-n` | Use `null` as input; no file is read |
| `--exit-status` | `-e` | Exit 1 if the last output is `false` or `null` |
| `--write-output` | `-w` | Write output to a file instead of stdout |
| `--no-color` | | Disable syntax highlighting |

---

## More Examples

### Direct jq expressions (no AI needed)

If you already know jq, you can use `anyq` as a drop-in replacement for `jq`, `yq`, and `tomlq` — just point it at any file format:

```bash
# Works on JSON, YAML, and TOML without any flags
anyq '.database.host' config.yaml
anyq '.users[] | select(.age > 30)' users.json
anyq '.dependencies | keys' Cargo.toml
anyq '.tool.poetry.version' pyproject.toml
```

### Format conversion

```bash
anyq -o json '.' config.yaml      # YAML → JSON
anyq -o yaml '.' Cargo.toml       # TOML → YAML
anyq -o toml '.' package.json     # JSON → TOML
```

### Piping from stdin

```bash
cat config.yaml | anyq --ai "what is the log level"
kubectl get pods -o json | anyq --ai "names of pods not in Running state"
```

### Slurp mode — combine multiple documents

`--slurp` reads all input documents into a single array before running the expression. Essential for operations that need the full dataset: `group_by`, `unique_by`, aggregations across files, or the concatenated JSON that tools like `go list -json` produce.

```bash
# Count packages across the whole codebase
go list -json ./... | anyq --slurp 'length'

# Collect all unique imports across every package
go list -json ./... | anyq --slurp '[.[].Imports // [] | .[]] | unique | sort | .[]' -r

# Combine multiple JSON files and group by a field
anyq --slurp 'group_by(.status)' a.json b.json c.json

# Sum a field across files
anyq --slurp '[.[].score] | add' results-jan.json results-feb.json

# Multi-document YAML (--- separated)
anyq --slurp 'length' deployments.yaml
```

### Semantic diff

Compare two config files regardless of format or key order:

```bash
anyq diff old.yaml new.yaml
anyq diff config.yaml config.json          # cross-format
anyq diff --exit-status baseline.yaml current.yaml   # CI gatekeeper
```

```
-  .app.debug                                   true
+  .app.replicas                                3
~  .database.host                               "localhost"  →  "production.db.internal"
```

### Schema validation

```bash
anyq validate --schema pod.schema.json deployment.yaml
anyq validate -s openapi.json api-config.json
anyq validate --schema schema.json data.toml && echo "safe to deploy"
```

```
✗ Validation failed: 2 error(s)

  • .: missing properties: 'version'
  • .port: must be <= 65535 but found 99999
```

### GitHub Actions

```yaml
- name: Read config value
  uses: troll-warlord/anyq@v0.1.0
  id: read-config
  with:
    expression: '.deploy.region'
    file: config.yaml
    output-format: json

- name: Use the result
  run: echo "Deploying to ${{ steps.read-config.outputs.result }}"
```

---

## How It Works

```
Natural language request
        │
        ▼
  ┌─────────────────┐
  │  Schema Extract │  ← keys + types only, values never leave your machine
  └────────┬────────┘
           │ schema JSON
           ▼
  ┌─────────────────┐
  │   AI Provider   │  ← OpenAI / Anthropic / Gemini / Ollama
  └────────┬────────┘
           │ jq expression
           ▼
  ┌─────────────────┐
  │  gojq Validate  │  ← compile check; retries with error feedback if invalid
  └────────┬────────┘
           │ validated expression
           ▼
  ┌─────────────────┐        ┌──────────────┐
  │   jq Engine     │ ◄──────│  Real data   │  ← your actual file (local only)
  └────────┬────────┘        └──────────────┘
           │ results
           ▼
  ┌─────────────────┐
  │   Serializer    │  ← JSON / YAML / TOML, syntax-highlighted
  └────────┬────────┘
           │
           ▼
        Output
```

---

## Roadmap

- [x] **AI-assisted querying** — describe what you want in natural language, get results instantly
- [x] **Semantic diff** — `anyq diff old.yaml new.yaml` — format-agnostic, key-order-insensitive
- [x] **Schema validation** — `anyq validate --schema k8s.json deploy.yaml` — JSON Schema drafts 4–2020-12
- [x] **Syntax highlighting** — colorized output in the terminal
- [x] **Multiple document streams** — `---` separated YAML documents and concatenated JSON (e.g. `go list -json ./...`) via `--slurp`
- [ ] **In-place editing** (`-i` flag, like `sed -i`) — modify files while preserving YAML comments
- [ ] **Interactive TUI** — `anyq explore file.yaml` — navigable tree view with path extraction
- [ ] **XML support** — extend auto-detection and engine to XML
- [ ] **Shell completion** — `anyq completion bash|zsh|fish|powershell`

---

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md). Issues and PRs are welcome.

---

## License

[MIT](./LICENSE) © 2026 troll-warlord
