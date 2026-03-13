# anyq

> **Query JSON, YAML, and TOML with one tool, full jq syntax.**

[![CI](https://github.com/troll-warlord/anyq/actions/workflows/ci.yml/badge.svg)](https://github.com/troll-warlord/anyq/actions/workflows/ci.yml)
[![Release](https://github.com/troll-warlord/anyq/actions/workflows/release.yml/badge.svg)](https://github.com/troll-warlord/anyq/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/troll-warlord/anyq)](https://goreportcard.com/report/github.com/troll-warlord/anyq)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

`anyq` is a single static binary that replaces `jq`, `yq`, and `tomlq`. It auto-detects your input format and lets you query and transform data using the full **jq expression language** you already know.

---

## Features

| Feature | Details |
|---|---|
| **Auto-detection** | Identifies JSON, YAML, TOML by file extension or content sniffing |
| **Full jq syntax** | Pipes, `select()`, `map()`, `walk()`, `env`, `@base64`, `@csv`, `@tsv` — everything |
| **Format conversion** | Read YAML → output JSON, read TOML → output YAML, etc. |
| **Streaming stdin** | `cat config.yaml \| anyq .database` just works |
| **Zero dependencies** | Single static binary, no runtime required |
| **Human-readable errors** | `Error: invalid YAML at line 14, column 3` |

---

## Installation

### Pre-built binary (recommended)

```bash
# Linux / macOS — replace VERSION and ARCH as needed
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
docker run --rm ghcr.io/troll-warlord/anyq:latest '.database.host' < config.yaml
```

---

## Usage

```
anyq [flags] <jq-expression> [file...]
```

### Flags

| Flag | Short | Description |
|---|---|---|
| `--input-format` | `-f` | Force input format: `json`, `yaml`, `toml` |
| `--output-format` | `-o` | Output format: `json`, `yaml`, `toml` (default: same as input) |
| `--pretty` | | Pretty-print JSON output |
| `--raw-output` | `-r` | Print strings without surrounding quotes |
| `--compact` | `-c` | Compact JSON output (no whitespace) |
| `--null-input` | `-n` | Use `null` as input; no file is read |
| `--exit-status` | `-e` | Exit 1 if the last output is `false` or `null` |
| `--write-output` | `-w` | Write output to a file instead of stdout |

---

## Examples

### Basic field access

```bash
anyq '.database.host' config.yaml
```

### Array operations (jq full syntax)

```bash
# Select users older than 30
anyq '.users[] | select(.age > 30)' users.json

# Extract just the names
anyq '[.users[] | .name]' users.json

# Count items
anyq '.items | length' data.yaml
```

### Format conversion

```bash
# YAML → JSON
anyq -o json '.' config.yaml

# TOML → YAML
anyq -o yaml '.' Cargo.toml

# JSON → TOML
anyq -o toml '.' package.json
```

### Piping from stdin

```bash
cat config.yaml | anyq '.database'
kubectl get pods -o json | anyq '[.items[] | .metadata.name]'
```

### TOML queries

```bash
anyq '.dependencies | keys' Cargo.toml
anyq '.tool.poetry.version' pyproject.toml
```

### Semantic diff

```bash
# Compare two files — key order and format don't matter
anyq diff old.yaml new.yaml

# Cross-format diff (YAML vs JSON)
anyq diff config.yaml config.json

# Exit 1 if files differ — useful in CI to detect config drift
anyq diff --exit-status baseline.yaml current.yaml
```

Output uses jq-style paths so every change is immediately queryable:
```
-  .app.debug                                   true
+  .app.replicas                                3
~  .database.host                               "localhost"  →  "production.db.internal"
```

### Schema validation

```bash
# Validate a YAML/TOML/JSON file against a JSON Schema
anyq validate --schema pod.schema.json deployment.yaml

# Also accepts shorthand -s
anyq validate -s openapi.json api-config.json

# Non-zero exit on failure — perfect as a CI gatekeeper
anyq validate --schema schema.json data.toml && echo "safe to deploy"
```

Errors point directly to the failing field:
```
✗ Validation failed: 2 error(s)

  • .: missing properties: 'version'
  • .port: must be <= 65535 but found 99999
```

### Build pipeline usage

```bash
# Extract a version from a config file and use it in a script
VERSION=$(anyq -r '.version' package.yaml)
echo "Building version $VERSION"
```

### Null input (generate data)

```bash
anyq -n '{name: "anyq", version: "0.1.0"}' | anyq -o yaml '.'
```

---

## GitHub Actions

Use `anyq` as a step in your GitHub workflows:

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

## Roadmap

- [x] **Semantic diff** — `anyq diff old.yaml new.yaml` — format-agnostic, key-order-insensitive
- [x] **Schema validation** — `anyq validate --schema k8s.json deploy.yaml` — JSON Schema drafts 4–2020-12
- [ ] **In-place editing** (`-i` flag, like `sed -i`) — modify files while preserving YAML comments
- [ ] **AI-assisted querying** — describe what you want in natural language, get the jq expression
- [ ] **Interactive TUI** — `anyq explore file.yaml` — navigable tree view with path extraction
- [ ] **XML support** — extend auto-detection and engine to XML
- [ ] **Multiple document streams** — `---` separated YAML documents
- [ ] **Shell completion** — `anyq completion bash|zsh|fish|powershell`

---

## How It Works

```
Input (file or stdin)
        │
        ▼
  ┌─────────────┐
  │  Detector   │  ← extension sniff → content sniff (first 512 bytes)
  └──────┬──────┘
         │ Format
         ▼
  ┌─────────────┐
  │   Parser    │  ← goccy/go-yaml (JSON+YAML) or pelletier/go-toml
  └──────┬──────┘
         │ interface{}
         ▼
  ┌─────────────┐
  │  jq Engine  │  ← itchyny/gojq (full jq spec)
  └──────┬──────┘
         │ results
         ▼
  ┌─────────────┐
  │ Serializer  │  ← JSON / YAML / TOML
  └──────┬──────┘
         │
         ▼
      Output
```

---

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md). Issues and PRs are welcome.

---

## License

[MIT](./LICENSE) © 2026 troll-warlord
