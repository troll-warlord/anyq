# Contributing to anyq

Thank you for considering a contribution! Here's everything you need to get started.

---

## Development Setup

**Prerequisites:** Go 1.21+, Git, optionally [golangci-lint](https://golangci-lint.run/usage/install/).

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

```
anyq/
├── main.go                    # Entry point
├── cmd/
│   └── root.go                # Cobra CLI definition and flag wiring
├── internal/
│   ├── detector/
│   │   └── detector.go        # Format auto-detection (extension + content sniffing)
│   └── engine/
│       └── engine.go          # Parse → jq query → serialize pipeline
├── .goreleaser.yaml           # Release automation
├── .github/
│   └── workflows/
│       ├── ci.yml             # CI: lint + test + snapshot build
│       └── release.yml        # Release: triggered on tag push
├── Dockerfile                 # Multi-stage minimal image
└── action.yml                 # GitHub Marketplace action metadata
```

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

---

## Versioning

anyq follows [Semantic Versioning](https://semver.org/). Releases are automated via GoReleaser triggered by a tag push. Maintainers handle tagging.
