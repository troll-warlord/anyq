// Package detector sniffs file format from extension or content.
package detector

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// Format represents a supported data format.
type Format string

const (
	FormatJSON    Format = "json"
	FormatYAML    Format = "yaml"
	FormatTOML    Format = "toml"
	FormatUnknown Format = "unknown"
)

// FromPath detects format from a file path, falling back to content sniffing.
func FromPath(path string) (Format, error) {
	// 1. Try extension first.
	if f := fromExtension(path); f != FormatUnknown {
		return f, nil
	}

	// 2. Read first 512 bytes and sniff content.
	file, err := os.Open(path)
	if err != nil {
		return FormatUnknown, err
	}
	defer file.Close()

	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	return fromBytes(buf[:n]), nil
}

// FromBytes detects format purely from raw bytes (used for stdin).
func FromBytes(data []byte) Format {
	// Try extension-less sniff on raw content.
	return fromBytes(data)
}

func fromExtension(path string) Format {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return FormatJSON
	case ".yaml", ".yml":
		return FormatYAML
	case ".toml":
		return FormatTOML
	}
	return FormatUnknown
}

// fromBytes inspects the first bytes to guess format.
// Order matters: JSON is unambiguous (starts with { or [), TOML uses = assignments,
// YAML is the fallback for anything that looks like key: value or starts with ---.
func fromBytes(data []byte) Format {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return FormatUnknown
	}

	first := trimmed[0]

	// JSON always starts with { or [
	if first == '{' || first == '[' {
		return FormatJSON
	}

	// TOML characteristic: lines like [section] or key = value
	if isTOML(trimmed) {
		return FormatTOML
	}

	// YAML: --- document marker, or key: value patterns
	if isYAML(trimmed) {
		return FormatYAML
	}

	return FormatUnknown
}

func isTOML(data []byte) bool {
	lines := bytes.SplitN(data, []byte("\n"), 20)
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		// TOML table header: [section]
		if line[0] == '[' && line[len(line)-1] == ']' {
			return true
		}
		// TOML key = value (not YAML key: value)
		if bytes.Contains(line, []byte(" = ")) || bytes.Contains(line, []byte("=")) {
			if !bytes.Contains(line, []byte(":")) {
				return true
			}
		}
	}
	return false
}

func isYAML(data []byte) bool {
	// YAML document separator
	if bytes.HasPrefix(data, []byte("---")) {
		return true
	}
	lines := bytes.SplitN(data, []byte("\n"), 10)
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		// key: value
		if bytes.Contains(line, []byte(": ")) {
			return true
		}
		// bare key: (with nothing after)
		if bytes.HasSuffix(line, []byte(":")) {
			return true
		}
	}
	return false
}
