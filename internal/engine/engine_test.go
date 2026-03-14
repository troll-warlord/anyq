package engine

import (
	"bytes"
	"strings"
	"testing"

	"github.com/troll-warlord/anyq/internal/detector"
)

// helpers

func runJSON(t *testing.T, query, input string, opts Options) string {
	t.Helper()
	opts.InputFormat = detector.FormatJSON
	if opts.OutputFormat == detector.FormatUnknown || opts.OutputFormat == "" {
		opts.OutputFormat = detector.FormatJSON
	}
	var buf bytes.Buffer
	if err := Run(nil, &buf, query, []byte(input), opts); err != nil {
		t.Fatalf("Run(%q): %v", query, err)
	}
	return strings.TrimRight(buf.String(), "\n")
}

// Parse

func TestParse_JSON(t *testing.T) {
	v, err := Parse([]byte(`{"a":1}`), detector.FormatJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	if m["a"] == nil {
		t.Error("expected key 'a'")
	}
}

func TestParse_YAML(t *testing.T) {
	v, err := Parse([]byte("a: 1\nb: hello"), detector.FormatYAML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	if m["b"] == nil {
		t.Error("expected key 'b'")
	}
}

func TestParse_TOML(t *testing.T) {
	v, err := Parse([]byte("a = 1\nb = \"hello\""), detector.FormatTOML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	if m["b"] == nil {
		t.Error("expected key 'b'")
	}
}

func TestParse_StripsBOM(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	data := append(bom, []byte(`{"key":"value"}`)...)
	_, err := Parse(data, detector.FormatJSON)
	if err != nil {
		t.Fatalf("BOM not stripped: %v", err)
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`{bad`), detector.FormatJSON)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	_, err := Parse([]byte("key: [unclosed"), detector.FormatYAML)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParse_UnsupportedFormat(t *testing.T) {
	_, err := Parse([]byte("data"), detector.FormatUnknown)
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

// Run — query execution

func TestRun_Identity(t *testing.T) {
	got := runJSON(t, ".", `{"name":"anyq"}`, Options{Pretty: false})
	if !strings.Contains(got, "anyq") {
		t.Errorf("expected 'anyq' in output, got %q", got)
	}
}

func TestRun_FieldAccess(t *testing.T) {
	got := runJSON(t, ".name", `{"name":"anyq","version":"1.0"}`, Options{})
	if !strings.Contains(got, "anyq") {
		t.Errorf("expected 'anyq' in output, got %q", got)
	}
}

func TestRun_Filter(t *testing.T) {
	got := runJSON(t, ".[] | select(.age > 30)", `[{"name":"alice","age":25},{"name":"bob","age":35}]`, Options{})
	if !strings.Contains(got, "bob") {
		t.Errorf("expected 'bob' in output, got %q", got)
	}
	if strings.Contains(got, "alice") {
		t.Errorf("did not expect 'alice' in output, got %q", got)
	}
}

func TestRun_RawOutput(t *testing.T) {
	got := runJSON(t, ".name", `{"name":"anyq"}`, Options{RawOutput: true})
	if got != "anyq" {
		t.Errorf("raw output: got %q, want %q", got, "anyq")
	}
}

func TestRun_CompactJSON(t *testing.T) {
	got := runJSON(t, ".", `{"a":1,"b":2}`, Options{Compact: true})
	if strings.Contains(got, "\n  ") {
		t.Errorf("compact output should not have indentation, got %q", got)
	}
}

func TestRun_NullInput(t *testing.T) {
	opts := Options{
		InputFormat:  detector.FormatJSON,
		OutputFormat: detector.FormatJSON,
		NullInput:    true,
	}
	var buf bytes.Buffer
	if err := Run(nil, &buf, "1+1", nil, opts); err != nil {
		t.Fatalf("null input: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "2" {
		t.Errorf("1+1 with null input = %q, want %q", buf.String(), "2")
	}
}

func TestRun_ExitStatus_False(t *testing.T) {
	opts := Options{
		InputFormat:  detector.FormatJSON,
		OutputFormat: detector.FormatJSON,
		ExitStatus:   true,
	}
	var buf bytes.Buffer
	err := Run(nil, &buf, "false", []byte("null"), opts)
	if err != ErrExitStatus {
		t.Errorf("expected ErrExitStatus, got %v", err)
	}
}

func TestRun_ExitStatus_Null(t *testing.T) {
	opts := Options{
		InputFormat:  detector.FormatJSON,
		OutputFormat: detector.FormatJSON,
		ExitStatus:   true,
	}
	var buf bytes.Buffer
	err := Run(nil, &buf, "null", []byte("null"), opts)
	if err != ErrExitStatus {
		t.Errorf("expected ErrExitStatus, got %v", err)
	}
}

func TestRun_ExitStatus_True(t *testing.T) {
	opts := Options{
		InputFormat:  detector.FormatJSON,
		OutputFormat: detector.FormatJSON,
		ExitStatus:   true,
	}
	var buf bytes.Buffer
	err := Run(nil, &buf, "true", []byte("null"), opts)
	if err != nil {
		t.Errorf("expected no error for true with exit-status, got %v", err)
	}
}

func TestRun_InvalidJQExpression(t *testing.T) {
	opts := Options{InputFormat: detector.FormatJSON, OutputFormat: detector.FormatJSON}
	var buf bytes.Buffer
	err := Run(nil, &buf, ".[invalid", []byte(`{}`), opts)
	if err == nil {
		t.Error("expected error for invalid jq expression")
	}
}

func TestRun_AutoDetect_JSON(t *testing.T) {
	opts := Options{} // FormatUnknown — auto-detect
	var buf bytes.Buffer
	if err := Run(nil, &buf, ".key", []byte(`{"key":"detected"}`), opts); err != nil {
		t.Fatalf("auto-detect: %v", err)
	}
	if !strings.Contains(buf.String(), "detected") {
		t.Errorf("auto-detect: expected 'detected' in output, got %q", buf.String())
	}
}

// Format conversion

func TestRun_JSONToYAML(t *testing.T) {
	opts := Options{
		InputFormat:  detector.FormatJSON,
		OutputFormat: detector.FormatYAML,
	}
	var buf bytes.Buffer
	if err := Run(nil, &buf, ".", []byte(`{"name":"anyq"}`), opts); err != nil {
		t.Fatalf("JSON→YAML: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "name:") || !strings.Contains(out, "anyq") {
		t.Errorf("JSON→YAML: unexpected output %q", out)
	}
}

func TestRun_YAMLToJSON(t *testing.T) {
	opts := Options{
		InputFormat:  detector.FormatYAML,
		OutputFormat: detector.FormatJSON,
		Compact:      true,
	}
	var buf bytes.Buffer
	if err := Run(nil, &buf, ".", []byte("name: anyq"), opts); err != nil {
		t.Fatalf("YAML→JSON: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"name"`) || !strings.Contains(out, "anyq") {
		t.Errorf("YAML→JSON: unexpected output %q", out)
	}
}
