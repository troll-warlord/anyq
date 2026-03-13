# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | ✅        |
| < 0.1.0 | ❌        |

## Reporting a Vulnerability

**Please do not open a public GitHub Issue for security vulnerabilities.**

Report security issues privately via [GitHub's private vulnerability reporting](https://github.com/troll-warlord/anyq/security/advisories/new).

Include:
- A description of the vulnerability and its potential impact
- Steps to reproduce
- Affected versions
- Any suggested fix (optional)

You will receive a response within **72 hours**. If a vulnerability is confirmed, a fix will be released and you will be credited in the release notes (unless you prefer to remain anonymous).

## Security Model

`anyq` is a local CLI tool — it reads files from disk or stdin and writes to stdout. It does not:
- Open network connections
- Execute arbitrary code from input files
- Require elevated privileges

The primary attack surface is **malformed input files** (e.g., YAML bombs / billion laughs). If you discover a denial-of-service via crafted input, please report it.
