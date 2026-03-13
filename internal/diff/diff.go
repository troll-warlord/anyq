// Package diff implements semantic comparison of two parsed documents.
// It wraps r3labs/diff to produce jq-style path annotations so the output
// is immediately actionable in anyq queries.
package diff

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	r3diff "github.com/r3labs/diff/v3"
)

// Change represents a single semantic difference between two documents.
type Change struct {
	Type string      // "create", "update", or "delete"
	Path string      // jq-style path, e.g. .database.host or .items[0].name
	From interface{} // previous value (nil for "create")
	To   interface{} // new value (nil for "delete")
}

// Compare semantically diffs two parsed documents (output of engine.Parse)
// and returns an ordered list of changes. Key order and whitespace are ignored;
// only actual data differences are reported.
func Compare(a, b interface{}) ([]Change, error) {
	changelog, err := r3diff.Diff(a, b)
	if err != nil {
		return nil, fmt.Errorf("diff: %w", err)
	}

	changes := make([]Change, 0, len(changelog))
	for _, c := range changelog {
		changes = append(changes, Change{
			Type: string(c.Type),
			Path: formatPath(c.Path),
			From: c.From,
			To:   c.To,
		})
	}
	return changes, nil
}

// Print writes a human-readable diff to w.
//
// Output format:
//
//   - .key  "new value"          (key was added)
//   - .key  "old value"          (key was removed)
//     ~  .key  "old"  →  "new"      (value was changed)
func Print(w io.Writer, changes []Change) {
	if len(changes) == 0 {
		fmt.Fprintln(w, "No differences found.")
		return
	}
	for _, c := range changes {
		switch c.Type {
		case "create":
			fmt.Fprintf(w, "+  %-44s %s\n", c.Path, formatVal(c.To))
		case "delete":
			fmt.Fprintf(w, "-  %-44s %s\n", c.Path, formatVal(c.From))
		case "update":
			fmt.Fprintf(w, "~  %-44s %s  →  %s\n", c.Path, formatVal(c.From), formatVal(c.To))
		}
	}
}

// formatPath converts a diff path slice into a jq-compatible accessor string.
// String segments become .key and numeric segments become [n].
// Examples: ["database","host"] → ".database.host"
//
//	["items","0","name"] → ".items[0].name"
func formatPath(path []string) string {
	if len(path) == 0 {
		return "."
	}
	var b strings.Builder
	for _, seg := range path {
		if _, err := strconv.Atoi(seg); err == nil {
			fmt.Fprintf(&b, "[%s]", seg)
		} else {
			fmt.Fprintf(&b, ".%s", seg)
		}
	}
	return b.String()
}

// formatVal produces a compact, human-readable representation of a value.
func formatVal(v interface{}) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case json.Number:
		return val.String()
	case string:
		return fmt.Sprintf("%q", val)
	case bool:
		return fmt.Sprintf("%v", val)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}
