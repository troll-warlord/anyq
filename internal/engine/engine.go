// Package engine provides a unified parse → query → serialize pipeline.
// It accepts any supported format as input, runs a jq expression against it,
// and serializes the result to the requested output format.
package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/itchyny/gojq"
	"github.com/pelletier/go-toml/v2"
	"github.com/troll-warlord/anyq/internal/detector"
	"github.com/troll-warlord/anyq/internal/highlight"
)

// Options controls engine behaviour.
type Options struct {
	InputFormat  detector.Format // FormatUnknown means auto-detect from content
	OutputFormat detector.Format // FormatUnknown means same as input
	Pretty       bool            // pretty-print output (JSON only; YAML/TOML are always pretty)
	RawOutput    bool            // -r: print strings without quotes
	Compact      bool            // -c: compact JSON output
	NullInput    bool            // -n: use null as input (no parsing)
	ExitStatus   bool            // -e: exit 1 if last value is false/null
	Color        bool            // enable ANSI syntax highlighting on output
}

// Run executes the full pipeline: read → detect → parse → query → serialize → write.
func Run(r io.Reader, w io.Writer, query string, data []byte, opts Options) error {
	inputFmt := opts.InputFormat

	// Auto-detect if not specified.
	if inputFmt == detector.FormatUnknown || inputFmt == "" {
		inputFmt = detector.FromBytes(data)
		if inputFmt == detector.FormatUnknown {
			// Default to JSON (mirrors jq behaviour)
			inputFmt = detector.FormatJSON
		}
	}

	outputFmt := opts.OutputFormat
	if outputFmt == detector.FormatUnknown || outputFmt == "" {
		outputFmt = inputFmt
	}

	// Parse input into a generic Go value.
	var input interface{}
	var err error

	if opts.NullInput {
		input = nil
	} else {
		input, err = parse(data, inputFmt)
		if err != nil {
			return err
		}
	}

	// Compile and run the jq query.
	results, err := execQuery(query, input)
	if err != nil {
		return err
	}

	return writeResults(w, results, outputFmt, opts)
}

// ErrExitStatus is returned when --exit-status is set and the last value is false/null.
var ErrExitStatus = fmt.Errorf("exit status 1")

// writeResults serializes and writes every query result to w.
func writeResults(w io.Writer, results []interface{}, outputFmt detector.Format, opts Options) error {
	for i, result := range results {
		out, err := serialize(result, outputFmt, opts)
		if err != nil {
			return fmt.Errorf("serialize result %d: %w", i, err)
		}
		if opts.Color && !opts.RawOutput {
			if err := highlight.Write(w, out, outputFmt); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprint(w, out); err != nil {
				return err
			}
		}
	}
	if opts.ExitStatus && len(results) > 0 {
		last := results[len(results)-1]
		if last == nil || last == false {
			return ErrExitStatus
		}
	}
	return nil
}

// RunValues executes query against a pre-parsed slice of values.
// The slice is passed directly as the jq input — used by slurp mode where
// all documents are combined into a single array before querying.
func RunValues(w io.Writer, query string, inputs []interface{}, opts Options) error {
	outputFmt := opts.OutputFormat
	if outputFmt == detector.FormatUnknown || outputFmt == "" {
		outputFmt = opts.InputFormat
		if outputFmt == detector.FormatUnknown || outputFmt == "" {
			outputFmt = detector.FormatJSON
		}
	}
	results, err := execQuery(query, inputs)
	if err != nil {
		return err
	}
	return writeResults(w, results, outputFmt, opts)
}

// Parse converts raw bytes in the given format to a generic Go value.
// It is the exported counterpart of the internal parse function, intended
// for use by sibling packages (diff, validator) that need parsed data
// without running a jq query.
func Parse(data []byte, fmt_ detector.Format) (interface{}, error) {
	return parse(data, fmt_)
}

// ParseMulti parses all documents from data and returns them as a slice.
// For JSON it uses a streaming decoder so concatenated objects (e.g. the
// output of `go list -json ./...`) are each returned as a separate element.
// For YAML it splits on `---` document separators.
// For TOML there is only ever one document, so the slice has length 1.
func ParseMulti(data []byte, fmt_ detector.Format) ([]interface{}, error) {
	data = stripBOM(data)
	switch fmt_ {
	case detector.FormatJSON:
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		var docs []interface{}
		for {
			var v interface{}
			if err := dec.Decode(&v); err != nil {
				if err.Error() == "EOF" {
					break
				}
				return nil, fmt.Errorf("invalid JSON: %w", err)
			}
			norm, err := normalise(v)
			if err != nil {
				return nil, err
			}
			docs = append(docs, norm)
		}
		return docs, nil
	case detector.FormatYAML:
		// Split on YAML document separators.
		parts := bytes.Split(data, []byte("\n---\n"))
		var docs []interface{}
		for _, part := range parts {
			part = bytes.TrimSpace(part)
			if len(part) == 0 {
				continue
			}
			v, err := parseYAML(part)
			if err != nil {
				return nil, err
			}
			docs = append(docs, v)
		}
		return docs, nil
	default:
		v, err := parse(data, fmt_)
		if err != nil {
			return nil, err
		}
		return []interface{}{v}, nil
	}
}

// stripBOM removes a leading UTF-8 BOM (EF BB BF) if present.
// Windows tools (PowerShell Out-File, Notepad) frequently add BOMs.
func stripBOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return data[3:]
	}
	return data
}

// parse converts raw bytes into a generic interface{} value.
func parse(data []byte, fmt_ detector.Format) (interface{}, error) {
	data = stripBOM(data)
	switch fmt_ {
	case detector.FormatJSON:
		return parseJSON(data)
	case detector.FormatYAML:
		return parseYAML(data)
	case detector.FormatTOML:
		return parseTOML(data)
	default:
		return nil, fmt.Errorf("unsupported input format: %s", fmt_)
	}
}

func parseJSON(data []byte) (interface{}, error) {
	var v interface{}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // preserve numeric precision
	if err := dec.Decode(&v); err != nil {
		// Try to extract line information from JSON errors.
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return v, nil
}

func parseYAML(data []byte) (interface{}, error) {
	var v interface{}
	// Standard unmarshal produces map[string]interface{} which gojq understands directly.
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, formatYAMLError(err)
	}
	// Normalise to ensure all nested maps are map[string]interface{} (not map[interface{}]interface{}).
	return normalise(v)
}

func parseTOML(data []byte) (interface{}, error) {
	var v interface{}
	if err := toml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("invalid TOML: %w", err)
	}
	return normalise(v)
}

// normalise converts any non-standard map/slice types into plain
// map[string]interface{} / []interface{} that gojq understands.
func normalise(v interface{}) (interface{}, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("internal normalisation error: %w", err)
	}
	var out interface{}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("internal normalisation decode error: %w", err)
	}
	return out, nil
}

// execQuery compiles and runs a gojq expression, collecting all results.
func execQuery(expr string, input interface{}) ([]interface{}, error) {
	q, err := gojq.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid jq expression %q: %w", expr, err)
	}

	code, err := gojq.Compile(q)
	if err != nil {
		return nil, fmt.Errorf("compile jq expression: %w", err)
	}

	iter := code.Run(input)
	var results []interface{}
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, fmt.Errorf("jq runtime error: %w", err)
		}
		results = append(results, v)
	}
	return results, nil
}

// serialize converts a Go value to the target format string.
func serialize(v interface{}, fmt_ detector.Format, opts Options) (string, error) {
	// --raw-output always prints bare strings, regardless of format.
	if opts.RawOutput {
		if s, ok := v.(string); ok {
			return s + "\n", nil
		}
	}

	switch fmt_ {
	case detector.FormatJSON:
		return serializeJSON(v, opts)
	case detector.FormatYAML:
		return serializeYAML(v, opts)
	case detector.FormatTOML:
		return serializeTOML(v, opts)
	default:
		return "", fmt.Errorf("unsupported output format: %s", fmt_)
	}
}

func serializeJSON(v interface{}, opts Options) (string, error) {
	var b []byte
	var err error
	if opts.Pretty && !opts.Compact {
		b, err = json.MarshalIndent(v, "", "  ")
	} else {
		b, err = json.Marshal(v)
	}
	if err != nil {
		return "", fmt.Errorf("JSON serialization error: %w", err)
	}
	return string(b) + "\n", nil
}

func serializeYAML(v interface{}, _ Options) (string, error) {
	b, err := yaml.MarshalWithOptions(v, yaml.IndentSequence(true))
	if err != nil {
		return "", fmt.Errorf("YAML serialization error: %w", err)
	}
	return string(b), nil
}

func serializeTOML(v interface{}, _ Options) (string, error) {
	var buf strings.Builder
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return "", fmt.Errorf("TOML serialization error: %w", err)
	}
	return buf.String(), nil
}

// formatYAMLError extracts a human-readable message with line info.
func formatYAMLError(err error) error {
	// goccy/go-yaml embeds line/column in the error interface.
	type linerErr interface {
		GetLine() int
		GetColumn() int
	}
	if le, ok := err.(linerErr); ok {
		return fmt.Errorf("invalid YAML at line %d, column %d: %w", le.GetLine(), le.GetColumn(), err)
	}
	return fmt.Errorf("invalid YAML: %w", err)
}
