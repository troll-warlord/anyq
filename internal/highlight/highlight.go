// Package highlight provides ANSI syntax-colorization for JSON, YAML, and TOML output.
// It wraps alecthomas/chroma and falls back to plain text on any error.
package highlight

import (
	"io"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/troll-warlord/anyq/internal/detector"
)

// ANSI color escape codes. Use Reset after any color code.
const (
	Green  = "\033[32m"
	Red    = "\033[31m"
	Yellow = "\033[33m"
	Reset  = "\033[0m"
)

// Write syntax-highlights content in the given format and writes it to w.
// Falls back to writing content as-is if chroma fails.
func Write(w io.Writer, content string, fmt_ detector.Format) error {
	if err := quick.Highlight(w, content, lexerFor(fmt_), "terminal256", "github-dark"); err != nil {
		_, writeErr := io.WriteString(w, content)
		return writeErr
	}
	return nil
}

func lexerFor(fmt_ detector.Format) string {
	switch fmt_ {
	case detector.FormatJSON:
		return "json"
	case detector.FormatYAML:
		return "yaml"
	case detector.FormatTOML:
		return "toml"
	default:
		return "text"
	}
}
