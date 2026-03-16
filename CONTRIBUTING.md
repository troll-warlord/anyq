# Contributing to anyq

Thank you for considering a contribution! Here's everything you need to get started.

---

## Development Setup

**Prerequisites:** Go 1.25+, Git, optionally [golangci-lint](https://golangci-lint.run/usage/install/).

```bash
# Clone the repo
git clone https://github.com/troll-warlord/anyq.git
cd anyq

# Download dependencies
go mod download

# Build
go build -o anyq .

# Run tests
go test ./...

# Run with race detector (recommended before opening a PR)
go test -race ./...
```

---

## Project Structure

```bash
anyq/
├── main.go                    # Entry point
├── cmd/
│   ├── root.go                # Cobra CLI definition, flag wiring, AI pipeline
│   ├── diff.go                # anyq diff subcommand
│   ├── validate.go            # anyq validate subcommand
│   └── helpers.go             # Shared CLI utilities
├── internal/
│   ├── detector/
│   │   └── detector.go        # Format auto-detection (extension + content sniffing)
│   ├── engine/
│   │   └── engine.go          # Parse → jq query → serialize pipeline; ParseMulti for slurp
│   ├── diff/
│   │   └── diff.go            # Semantic diff between two parsed documents
│   ├── validator/
│   │   └── validator.go       # JSON Schema validation (drafts 4–2020-12)
│   ├── highlight/
│   │   └── highlight.go       # Syntax highlighting (chroma, github-dark theme) + ANSI constants
│   └── ai/
│       ├── ai.go              # NL→jq translation: schema extraction, prompt building, retry loop
│       ├── openai.go          # OpenAI provider
│       ├── anthropic.go       # Anthropic provider
│       ├── gemini.go          # Gemini provider
│       └── ollama.go          # Ollama (local) provider
├── .goreleaser.yaml           # Release automation
├── .github/
│   ├── release.yml            # GitHub-native changelog category config
│   └── workflows/
│       ├── ci.yml             # CI: lint + test + snapshot build
│       └── release.yml        # Release: triggered on tag push
├── Dockerfile                 # Multi-stage minimal image
├── install.sh                 # Curl-based installer script
└── action.yml                 # GitHub Marketplace action metadata
```

---

## AI Feature

The `--ai` flag translates natural language into a jq expression using an LLM provider. To work on or test it locally:

```bash
# OpenAI (default)
export ANYQ_AI_PROVIDER=openai
export OPENAI_API_KEY=sk-...

# Anthropic
export ANYQ_AI_PROVIDER=anthropic
export ANTHROPIC_API_KEY=...

# Gemini
export ANYQ_AI_PROVIDER=gemini
export GEMINI_API_KEY=...

# Ollama (local, free)
export ANYQ_AI_PROVIDER=ollama          # default base URL: http://localhost:11434
ollama pull qwen2.5-coder               # default model

# Override model for any provider
export ANYQ_AI_MODEL=gpt-4o

# Test it
echo '{"users":[{"name":"alice","role":"admin"}]}' | go run . --ai "names of all admins"
```

The AI pipeline never sends actual data values — only a structural schema (key names + types). See `internal/ai/ai.go` for details.

---

## Opening a Pull Request

1. **Fork** the repository and create a feature branch: `git checkout -b feat/my-feature`.
2. Make your changes with tests where appropriate.
3. Run `go test -race ./...` and `golangci-lint run` and ensure both pass.
4. Commit using [Conventional Commits](https://www.conventionalcommits.org/) — the changelog is generated from commit messages:
   - `feat: add XML support`
   - `fix: handle empty TOML files`
   - `perf: speed up YAML detection`
   - `docs: improve README examples`
5. Open a PR against `main`. Describe *what* and *why*.

---

## Reporting Bugs

Open a GitHub Issue with:

- `anyq --version` output
- The input file (or a minimal reproducer)
- The command you ran
- What you expected vs. what happened

---

## Code Style

- Standard Go formatting (`gofmt`). Run `gofmt -l .` — no output means you're clean.
- Keep packages small and focused. `internal/detector` only detects; `internal/engine` only processes.
- No CGO. The binary must remain a single static binary.
- HTTP requests must use `context.Context` (`http.NewRequestWithContext`) — enforced by the `noctx` linter.

---

## Versioning

anyq follows [Semantic Versioning](https://semver.org/). Releases are automated via GoReleaser triggered by a tag push. Maintainers handle tagging.
