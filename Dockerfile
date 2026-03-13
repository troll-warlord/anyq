# syntax=docker/dockerfile:1
#
# anyq – unified jq/yq/tomlq CLI
#
# This Dockerfile is used by GoReleaser, which pre-builds the binary and
# places it in the Docker build context alongside this file.  There is no
# compile step here; the image simply packages the pre-built binary into the
# smallest possible container.
#
# Build context provided by GoReleaser:
#   anyq                  – pre-built static binary
#   ca-certificates.crt   – CA bundle copied from the CI runner

# ── Runtime image ──────────────────────────────────────────────────────────────
# scratch has zero OS overhead (~0 B base).  The binary is fully static
# (CGO_ENABLED=0) so no libc or shell is needed.
FROM scratch

# OCI image labels (values are injected by GoReleaser via --label flags or
# can be overridden at build time with --build-arg).
LABEL org.opencontainers.image.title="anyq" \
      org.opencontainers.image.description="Unified jq-syntax query tool for JSON, YAML, and TOML" \
      org.opencontainers.image.url="https://github.com/troll-warlord/anyq" \
      org.opencontainers.image.source="https://github.com/troll-warlord/anyq" \
      org.opencontainers.image.licenses="MIT"

# The pre-built static binary
COPY anyq /anyq

ENTRYPOINT ["/anyq"]

