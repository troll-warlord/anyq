package ai

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// CleanExpr
// ---------------------------------------------------------------------------

func TestCleanExpr(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain expression unchanged", `.users[] | select(.active)`, `.users[] | select(.active)`},
		{"single backtick wrapped", "`.foo`", `.foo`},
		{"triple backtick plain", "```\n.foo\n```", `.foo`},
		{"triple backtick jq fence", "```jq\n.foo\n```", `.foo`},
		{"triple backtick json fence", "```json\n.foo\n```", `.foo`},
		{"leading/trailing whitespace", "   .foo   ", `.foo`},
		{"fence with whitespace inside", "```jq\n  .foo  \n```", `.foo`},
		{"already clean dot", ".", "."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CleanExpr(tc.input)
			if got != tc.want {
				t.Errorf("CleanExpr(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// fixCommonMistakes
// ---------------------------------------------------------------------------

func TestFixCommonMistakes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"pipe inside .[] is rewritten to array constructor",
			`.[.users[] | select(.admin) | .name]`,
			`[.users[] | select(.admin) | .name]`,
		},
		{
			"simple index .[] without pipe is left alone",
			`.[0]`,
			`.[0]`,
		},
		{
			"normal array constructor unchanged",
			`[.users[] | .name]`,
			`[.users[] | .name]`,
		},
		{
			"plain path unchanged",
			`.users[].name`,
			`.users[].name`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fixCommonMistakes(tc.input)
			if got != tc.want {
				t.Errorf("fixCommonMistakes(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BuildUserPrompt
// ---------------------------------------------------------------------------

func TestBuildUserPrompt(t *testing.T) {
	schema := `{"users":[{"name":"string","role":"string"}]}`
	request := "list all admin names"

	prompt, err := BuildUserPrompt(schema, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, schema) {
		t.Error("prompt should contain the schema")
	}
	if !strings.Contains(prompt, request) {
		t.Error("prompt should contain the request")
	}
}

// ---------------------------------------------------------------------------
// ExtractSchema
// ---------------------------------------------------------------------------

func TestExtractSchema_JSON(t *testing.T) {
	input := []byte(`{
		"name": "alice",
		"age": 30,
		"active": true,
		"password": "s3cr3t",
		"roles": ["admin", "user", "viewer"]
	}`)

	schema, err := ExtractSchema(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &out); err != nil {
		t.Fatalf("schema is not valid JSON: %v\nschema: %s", err, schema)
	}

	// Leaf string values become "string"
	if out["name"] != "string" {
		t.Errorf("name: got %v, want \"string\"", out["name"])
	}
	// Leaf numbers become "number"
	if out["age"] != "number" {
		t.Errorf("age: got %v, want \"number\"", out["age"])
	}
	// Leaf booleans become "boolean"
	if out["active"] != "boolean" {
		t.Errorf("active: got %v, want \"boolean\"", out["active"])
	}
	// Sensitive field names are redacted
	if out["password"] != "<redacted>" {
		t.Errorf("password: got %v, want \"<redacted>\"", out["password"])
	}
	// Arrays are sampled (3 items) and each element is type-replaced
	roles, ok := out["roles"].([]interface{})
	if !ok {
		t.Fatalf("roles: expected []interface{}, got %T", out["roles"])
	}
	if len(roles) != 3 {
		t.Errorf("roles length: got %d, want 3", len(roles))
	}
	for _, r := range roles {
		if r != "string" {
			t.Errorf("roles element: got %v, want \"string\"", r)
		}
	}
}

func TestExtractSchema_ArraySampledToThree(t *testing.T) {
	input := []byte(`{"items":[1,2,3,4,5,6,7,8,9,10]}`)

	schema, err := ExtractSchema(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &out); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	items, ok := out["items"].([]interface{})
	if !ok {
		t.Fatalf("items: expected []interface{}, got %T", out["items"])
	}
	if len(items) != 3 {
		t.Errorf("items length: got %d, want 3 (capped)", len(items))
	}
}

func TestExtractSchema_EmptyInput(t *testing.T) {
	schema, err := ExtractSchema([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if schema != "{}" {
		t.Errorf("got %q, want \"{}\"", schema)
	}
}

func TestExtractSchema_YAML(t *testing.T) {
	input := []byte(`name: bob
age: 25
active: false
token: abc123
`)

	schema, err := ExtractSchema(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &out); err != nil {
		t.Fatalf("schema is not valid JSON: %v\nschema: %s", err, schema)
	}

	if out["name"] != "string" {
		t.Errorf("name: got %v, want \"string\"", out["name"])
	}
	if out["age"] != "number" {
		t.Errorf("age: got %v, want \"number\"", out["age"])
	}
	if out["active"] != "boolean" {
		t.Errorf("active: got %v, want \"boolean\"", out["active"])
	}
	if out["token"] != "<redacted>" {
		t.Errorf("token: got %v, want \"<redacted>\"", out["token"])
	}
}

func TestExtractSchema_NestedObject(t *testing.T) {
	input := []byte(`{"user":{"id":1,"name":"carol","address":{"city":"NYC"}}}`)

	schema, err := ExtractSchema(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &out); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	user, ok := out["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("user: expected map, got %T", out["user"])
	}
	if user["name"] != "string" {
		t.Errorf("user.name: got %v, want \"string\"", user["name"])
	}
	if user["id"] != "number" {
		t.Errorf("user.id: got %v, want \"number\"", user["id"])
	}

	address, ok := user["address"].(map[string]interface{})
	if !ok {
		t.Fatalf("user.address: expected map, got %T", user["address"])
	}
	if address["city"] != "string" {
		t.Errorf("user.address.city: got %v, want \"string\"", address["city"])
	}
}

// ---------------------------------------------------------------------------
// TranslateWithValidation (uses a mock provider — no HTTP needed)
// ---------------------------------------------------------------------------

// mockProvider is a test double that returns pre-configured responses.
type mockProvider struct {
	responses []string // popped in order; last value is repeated if list exhausted
	calls     int
}

func (m *mockProvider) Translate(_, _ string) (string, error) {
	idx := m.calls
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}
	m.calls++
	return m.responses[idx], nil
}

// alwaysValid accepts any expression.
func alwaysValid(_ string) error { return nil }

// alwaysInvalid always reports a compile error.
func alwaysInvalid(_ string) error { return errors.New("parse error") }

// validAfterN rejects the first n expressions, then accepts.
func validAfterN(n int) func(string) error {
	count := 0
	return func(_ string) error {
		if count < n {
			count++
			return errors.New("parse error")
		}
		return nil
	}
}

func TestTranslateWithValidation_SuccessFirstTry(t *testing.T) {
	p := &mockProvider{responses: []string{`.foo`}}
	expr, err := TranslateWithValidation(p, "", "get foo", alwaysValid, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expr != `.foo` {
		t.Errorf("got %q, want \".foo\"", expr)
	}
	if p.calls != 1 {
		t.Errorf("Translate called %d times, want 1", p.calls)
	}
}

func TestTranslateWithValidation_SuccessAfterRetry(t *testing.T) {
	// First response is invalid jq, second is valid.
	p := &mockProvider{responses: []string{`.bad expression |||`, `.foo`}}
	expr, err := TranslateWithValidation(p, "", "get foo", validAfterN(1), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expr != `.foo` {
		t.Errorf("got %q, want \".foo\"", expr)
	}
	if p.calls != 2 {
		t.Errorf("Translate called %d times, want 2", p.calls)
	}
}

func TestTranslateWithValidation_ExhaustsRetries(t *testing.T) {
	p := &mockProvider{responses: []string{`.bad`}}
	_, err := TranslateWithValidation(p, "", "get foo", alwaysInvalid, 2)
	if err == nil {
		t.Fatal("expected error after all retries exhausted, got nil")
	}
	if !strings.Contains(err.Error(), "generated query failed to compile") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestTranslateWithValidation_ZeroRetries(t *testing.T) {
	// With maxRetries=0 the retry loop is skipped; final validation must pass.
	p := &mockProvider{responses: []string{`.foo`}}
	expr, err := TranslateWithValidation(p, "", "get foo", alwaysValid, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expr != `.foo` {
		t.Errorf("got %q, want \".foo\"", expr)
	}
}
