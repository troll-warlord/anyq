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

// ---------------------------------------------------------------------------
// RunValues (slurp mode)
// ---------------------------------------------------------------------------

func TestRunValues_SingleDoc(t *testing.T) {
	v, err := Parse([]byte(`{"x":1}`), detector.FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	opts := Options{InputFormat: detector.FormatJSON, OutputFormat: detector.FormatJSON, Compact: true}
	if err := RunValues(&buf, ".", []interface{}{v}, opts); err != nil {
		t.Fatalf("RunValues: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	// Input is wrapped in a slice, so output should be an array.
	if !strings.HasPrefix(out, "[") {
		t.Errorf("expected array output, got: %s", out)
	}
	if !strings.Contains(out, `"x"`) {
		t.Errorf("expected key x, got: %s", out)
	}
}

func TestRunValues_MultipleDocsCombined(t *testing.T) {
	a, _ := Parse([]byte(`{"n":1}`), detector.FormatJSON)
	b, _ := Parse([]byte(`{"n":2}`), detector.FormatJSON)
	var buf bytes.Buffer
	opts := Options{InputFormat: detector.FormatJSON, OutputFormat: detector.FormatJSON, Compact: true}
	if err := RunValues(&buf, "length", []interface{}{a, b}, opts); err != nil {
		t.Fatalf("RunValues: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "2" {
		t.Errorf("expected 2, got: %s", buf.String())
	}
}

func TestRunValues_SumField(t *testing.T) {
	a, _ := Parse([]byte(`{"v":10}`), detector.FormatJSON)
	b, _ := Parse([]byte(`{"v":20}`), detector.FormatJSON)
	var buf bytes.Buffer
	opts := Options{InputFormat: detector.FormatJSON, OutputFormat: detector.FormatJSON, Compact: true}
	if err := RunValues(&buf, "[.[].v] | add", []interface{}{a, b}, opts); err != nil {
		t.Fatalf("RunValues: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "30" {
		t.Errorf("expected 30, got: %s", buf.String())
	}
}

func TestRunValues_EmptySlice(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{InputFormat: detector.FormatJSON, OutputFormat: detector.FormatJSON, Compact: true}
	if err := RunValues(&buf, "length", []interface{}{}, opts); err != nil {
		t.Fatalf("RunValues: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "0" {
		t.Errorf("expected 0, got: %s", buf.String())
	}
}

func TestRunValues_DefaultsOutputFormatToJSON(t *testing.T) {
	v, _ := Parse([]byte(`{"k":"v"}`), detector.FormatJSON)
	var buf bytes.Buffer
	// No output format set — should default to JSON.
	opts := Options{Compact: true}
	if err := RunValues(&buf, ".", []interface{}{v}, opts); err != nil {
		t.Fatalf("RunValues: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "[") {
		t.Errorf("expected JSON array, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// ParseMulti
// ---------------------------------------------------------------------------

func TestParseMulti_SingleJSON(t *testing.T) {
	docs, err := ParseMulti([]byte(`{"a":1}`), detector.FormatJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
}

func TestParseMulti_ConcatenatedJSON(t *testing.T) {
	// Simulates `go list -json ./...` output: multiple objects with no separator.
	input := `{"Name":"a"}` + "\n" + `{"Name":"b"}` + "\n" + `{"Name":"c"}`
	docs, err := ParseMulti([]byte(input), detector.FormatJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected 3 docs, got %d", len(docs))
	}
	// Verify each doc is independently accessible.
	for i, d := range docs {
		m, ok := d.(map[string]interface{})
		if !ok {
			t.Fatalf("doc %d: expected map, got %T", i, d)
		}
		if m["Name"] == nil {
			t.Errorf("doc %d: missing Name field", i)
		}
	}
}

func TestParseMulti_EmptyJSON(t *testing.T) {
	docs, err := ParseMulti([]byte(`   `), detector.FormatJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %d", len(docs))
	}
}

func TestParseMulti_InvalidJSON(t *testing.T) {
	_, err := ParseMulti([]byte(`{"a": INVALID}`), detector.FormatJSON)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParseMulti_SingleYAML(t *testing.T) {
	docs, err := ParseMulti([]byte("name: alice\nage: 30"), detector.FormatYAML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
}

func TestParseMulti_MultiDocYAML(t *testing.T) {
	input := "name: alice\n---\nname: bob\n---\nname: carol"
	docs, err := ParseMulti([]byte(input), detector.FormatYAML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected 3 docs, got %d", len(docs))
	}
}

func TestParseMulti_TOML(t *testing.T) {
	// TOML always returns one document.
	docs, err := ParseMulti([]byte("[server]\nhost = \"localhost\""), detector.FormatTOML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
}

func TestParseMulti_JSONNumberPrecision(t *testing.T) {
	// Numbers should be preserved via UseNumber, not converted to float64.
	docs, err := ParseMulti([]byte(`{"id":12345678901234567}`), detector.FormatJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := docs[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", docs[0])
	}
	if _, isNum := m["id"].(interface{ String() string }); !isNum {
		// json.Number implements String(); float64 does not.
		t.Errorf("expected json.Number for large integer, got %T", m["id"])
	}
}
