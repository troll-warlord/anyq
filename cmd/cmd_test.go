package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// executeCmd runs the root command with the given args and captures stdout/stderr.
func executeCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	// Reset all flag values between tests.
	inputFormat = ""
	outputFormat = ""
	pretty = true
	rawOutput = false
	compact = false
	nullInput = false
	exitStatus = false
	inputFile = ""
	outputFile = ""
	noColor = true // always disable color in tests
	slurp = false

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs(args)

	err = rootCmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// writeTempFile writes content to a temp file with the given extension.
func writeTempFile(t *testing.T, ext, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test"+ext)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Root command tests

func TestRoot_FieldAccessJSON(t *testing.T) {
	f := writeTempFile(t, ".json", `{"name":"anyq","version":"1.0"}`)
	out, _, err := executeCmd(t, ".name", f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "anyq") {
		t.Errorf("expected 'anyq' in output, got %q", out)
	}
}

func TestRoot_FieldAccessYAML(t *testing.T) {
	f := writeTempFile(t, ".yaml", "name: anyq\nversion: \"1.0\"")
	out, _, err := executeCmd(t, ".name", f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "anyq") {
		t.Errorf("expected 'anyq' in output, got %q", out)
	}
}

func TestRoot_FieldAccessTOML(t *testing.T) {
	f := writeTempFile(t, ".toml", "name = \"anyq\"\nversion = \"1.0\"")
	out, _, err := executeCmd(t, ".name", f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "anyq") {
		t.Errorf("expected 'anyq' in output, got %q", out)
	}
}

func TestRoot_NullInput(t *testing.T) {
	out, _, err := executeCmd(t, "--null-input", "1+1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "2" {
		t.Errorf("1+1 null input = %q, want %q", strings.TrimSpace(out), "2")
	}
}

func TestRoot_RawOutput(t *testing.T) {
	f := writeTempFile(t, ".json", `{"name":"anyq"}`)
	out, _, err := executeCmd(t, "-r", ".name", f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "anyq" {
		t.Errorf("raw output = %q, want %q", strings.TrimSpace(out), "anyq")
	}
}

func TestRoot_OutputFormatConversion(t *testing.T) {
	f := writeTempFile(t, ".json", `{"name":"anyq"}`)
	out, _, err := executeCmd(t, "-o", "yaml", ".", f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "name:") {
		t.Errorf("expected YAML output with 'name:', got %q", out)
	}
}

func TestRoot_WriteOutput(t *testing.T) {
	f := writeTempFile(t, ".json", `{"name":"anyq"}`)
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.json")
	_, _, err := executeCmd(t, "-w", outPath, ".", f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if !strings.Contains(string(data), "anyq") {
		t.Errorf("output file content unexpected: %q", string(data))
	}
}

func TestRoot_PrettyAndCompactMutuallyExclusive(t *testing.T) {
	f := writeTempFile(t, ".json", `{"name":"anyq"}`)
	_, _, err := executeCmd(t, "--pretty", "--compact", ".", f)
	if err == nil {
		t.Error("expected error for --pretty and --compact together")
	}
}

func TestRoot_InvalidJQExpression(t *testing.T) {
	f := writeTempFile(t, ".json", `{"name":"anyq"}`)
	_, _, err := executeCmd(t, ".[invalid", f)
	if err == nil {
		t.Error("expected error for invalid jq expression")
	}
}

func TestRoot_MissingFile(t *testing.T) {
	_, _, err := executeCmd(t, ".", "/nonexistent/file.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// Diff command tests

func TestDiff_NoDifference(t *testing.T) {
	f1 := writeTempFile(t, ".json", `{"name":"anyq","version":"1.0"}`)
	f2 := writeTempFile(t, ".json", `{"name":"anyq","version":"1.0"}`)
	out, _, err := executeCmd(t, "diff", f1, f2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No differences") {
		t.Errorf("expected 'No differences', got %q", out)
	}
}

func TestDiff_WithDifferences(t *testing.T) {
	f1 := writeTempFile(t, ".json", `{"host":"localhost"}`)
	f2 := writeTempFile(t, ".json", `{"host":"production"}`)
	out, _, err := executeCmd(t, "diff", f1, f2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "~") {
		t.Errorf("expected '~' update in output, got %q", out)
	}
}

func TestDiff_CrossFormat(t *testing.T) {
	f1 := writeTempFile(t, ".json", `{"name":"anyq"}`)
	f2 := writeTempFile(t, ".yaml", "name: anyq")
	out, _, err := executeCmd(t, "diff", f1, f2)
	if err != nil {
		t.Fatalf("cross-format diff: %v", err)
	}
	if !strings.Contains(out, "No differences") {
		t.Errorf("cross-format diff: expected no differences, got %q", out)
	}
}

// Validate command tests

func TestValidate_Valid(t *testing.T) {
	schema := writeTempFile(t, ".json", `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"required": ["name"],
		"properties": {"name": {"type": "string"}}
	}`)
	data := writeTempFile(t, ".json", `{"name":"anyq"}`)
	out, _, err := executeCmd(t, "validate", "--schema", schema, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Valid") {
		t.Errorf("expected 'Valid' in output, got %q", out)
	}
}

func TestValidate_MissingSchemaFlag(t *testing.T) {
	data := writeTempFile(t, ".json", `{"name":"anyq"}`)
	_, _, err := executeCmd(t, "validate", data)
	if err == nil {
		t.Error("expected error for missing --schema flag")
	}
}

// parseFormat helper

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"json", "json"},
		{"JSON", "json"},
		{"yaml", "yaml"},
		{"yml", "yaml"},
		{"toml", "toml"},
		{"", "unknown"},
		{"xml", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseFormat(tt.input)
			if string(got) != tt.want {
				t.Errorf("parseFormat(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// Ensure rootCmd is re-initialized for each test to avoid flag pollution.
func init() {
	// Prevent cobra from calling os.Exit on flag errors in tests.
	rootCmd.FParseErrWhitelist = cobra.FParseErrWhitelist{UnknownFlags: false}
}

// ---------------------------------------------------------------------------
// Slurp mode tests
// ---------------------------------------------------------------------------

func TestSlurp_SingleFile(t *testing.T) {
	f := writeTempFile(t, ".json", `{"x":1}`)
	stdout, _, err := executeCmd(t, "--slurp", ".", f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Result should be an array wrapping the single document.
	if !strings.Contains(stdout, `"x"`) {
		t.Errorf("expected key x in output, got: %s", stdout)
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout), "[") {
		t.Errorf("expected array output, got: %s", stdout)
	}
}

func TestSlurp_MultipleFiles(t *testing.T) {
	f1 := writeTempFile(t, ".json", `{"name":"alice"}`)
	f2 := writeTempFile(t, ".json", `{"name":"bob"}`)
	stdout, _, err := executeCmd(t, "--slurp", "length", f1, f2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Two documents slurped → array of length 2.
	if strings.TrimSpace(stdout) != "2" {
		t.Errorf("expected 2, got: %s", stdout)
	}
}

func TestSlurp_ExtractField(t *testing.T) {
	f1 := writeTempFile(t, ".json", `{"v":10}`)
	f2 := writeTempFile(t, ".json", `{"v":20}`)
	// Sum all .v values across documents.
	stdout, _, err := executeCmd(t, "--slurp", "[.[].v] | add", f1, f2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(stdout) != "30" {
		t.Errorf("expected 30, got: %s", stdout)
	}
}

func TestSlurp_YAML(t *testing.T) {
	f := writeTempFile(t, ".yaml", "name: carol\nage: 25\n")
	stdout, _, err := executeCmd(t, "--slurp", ".[0].name", f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "carol") {
		t.Errorf("expected carol in output, got: %s", stdout)
	}
}

func TestSlurp_ShortFlag(t *testing.T) {
	f1 := writeTempFile(t, ".json", `{"n":1}`)
	f2 := writeTempFile(t, ".json", `{"n":2}`)
	stdout, _, err := executeCmd(t, "--slurp", "length", f1, f2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(stdout) != "2" {
		t.Errorf("expected 2, got: %s", stdout)
	}
}
