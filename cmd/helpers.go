package cmd

import (
	"fmt"
	"os"

	"github.com/troll-warlord/anyq/internal/detector"
	"github.com/troll-warlord/anyq/internal/engine"
)

// parseFile reads a file, auto-detects its format, and returns a parsed interface{}.
// Used by the diff and validate subcommands.
func parseFile(path string) (interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %q: %w", path, err)
	}

	fmt_, _ := detector.FromPath(path)
	if fmt_ == detector.FormatUnknown {
		fmt_ = detector.FromBytes(data)
	}
	if fmt_ == detector.FormatUnknown {
		fmt_ = detector.FormatJSON
	}

	return engine.Parse(data, fmt_)
}
