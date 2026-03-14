package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSchema writes a JSON Schema to a temp file and returns the path.
func writeSchema(t *testing.T, schema string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(path, []byte(schema), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

const simpleSchema = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["name", "version"],
  "properties": {
    "name":    {"type": "string"},
    "version": {"type": "string"},
    "port":    {"type": "integer", "minimum": 1, "maximum": 65535}
  }
}`

// Validate — valid data

func TestValidate_Valid(t *testing.T) {
	schema := writeSchema(t, simpleSchema)
	data := map[string]interface{}{
		"name":    "anyq",
		"version": "1.0.0",
	}
	result, err := Validate(data, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got: %v", result.Errors)
	}
}

func TestValidate_ValidWithOptionalField(t *testing.T) {
	schema := writeSchema(t, simpleSchema)
	data := map[string]interface{}{
		"name":    "anyq",
		"version": "1.0.0",
		"port":    float64(8080),
	}
	result, err := Validate(data, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}
}

// Validate — invalid data

func TestValidate_MissingRequired(t *testing.T) {
	schema := writeSchema(t, simpleSchema)
	data := map[string]interface{}{
		"name": "anyq",
		// missing "version"
	}
	result, err := Validate(data, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid, got valid")
	}
	if len(result.Errors) == 0 {
		t.Error("expected at least one error")
	}
}

func TestValidate_WrongType(t *testing.T) {
	schema := writeSchema(t, simpleSchema)
	data := map[string]interface{}{
		"name":    "anyq",
		"version": 123, // should be string
	}
	result, err := Validate(data, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid due to wrong type")
	}
}

func TestValidate_OutOfRange(t *testing.T) {
	schema := writeSchema(t, simpleSchema)
	data := map[string]interface{}{
		"name":    "anyq",
		"version": "1.0.0",
		"port":    float64(99999), // exceeds maximum
	}
	result, err := Validate(data, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid due to port out of range")
	}
}

// Validate — I/O errors

func TestValidate_MissingSchemaFile(t *testing.T) {
	_, err := Validate(map[string]interface{}{}, "/nonexistent/schema.json")
	if err == nil {
		t.Error("expected error for missing schema file")
	}
}

func TestValidate_InvalidSchema(t *testing.T) {
	schema := writeSchema(t, `{bad json`)
	_, err := Validate(map[string]interface{}{}, schema)
	if err == nil {
		t.Error("expected error for invalid schema JSON")
	}
}

func TestValidate_BOMStrippedSchema(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	content := append(bom, []byte(simpleSchema)...)
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	data := map[string]interface{}{"name": "anyq", "version": "1.0.0"}
	result, err := Validate(data, path)
	if err != nil {
		t.Fatalf("BOM schema: unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("BOM schema: expected valid, got errors: %v", result.Errors)
	}
}

// jsonPtrToJQ

func TestJSONPtrToJQ(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "."},
		{"/", "."},
		{"/name", ".name"},
		{"/spec/replicas", ".spec.replicas"},
		{"/items/0/name", ".items[0].name"},
		{"/a~1b", ".a/b"}, // ~1 → /
		{"/a~0b", ".a~b"}, // ~0 → ~
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := jsonPtrToJQ(tt.input)
			if got != tt.want {
				t.Errorf("jsonPtrToJQ(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// Error messages contain jq paths

func TestValidate_ErrorContainsPath(t *testing.T) {
	schema := writeSchema(t, simpleSchema)
	data := map[string]interface{}{
		"name":    "anyq",
		"version": 123,
	}
	result, err := Validate(data, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid")
	}
	// At least one error should mention the field path
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e, "version") || strings.Contains(e, ".") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error to contain field path, got: %v", result.Errors)
	}
}
