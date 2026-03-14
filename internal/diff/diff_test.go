package diff

import (
	"strings"
	"testing"
)

// helpers

func mustCompare(t *testing.T, a, b interface{}) []Change {
	t.Helper()
	changes, err := Compare(a, b)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	return changes
}

// Compare

func TestCompare_NoDifference(t *testing.T) {
	a := map[string]interface{}{"key": "value", "num": float64(42)}
	b := map[string]interface{}{"key": "value", "num": float64(42)}
	changes := mustCompare(t, a, b)
	if len(changes) != 0 {
		t.Errorf("expected 0 changes, got %d: %+v", len(changes), changes)
	}
}

func TestCompare_AddedKey(t *testing.T) {
	a := map[string]interface{}{"key": "value"}
	b := map[string]interface{}{"key": "value", "new": "added"}
	changes := mustCompare(t, a, b)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != "create" {
		t.Errorf("expected type 'create', got %q", changes[0].Type)
	}
	if !strings.Contains(changes[0].Path, "new") {
		t.Errorf("expected path to contain 'new', got %q", changes[0].Path)
	}
}

func TestCompare_DeletedKey(t *testing.T) {
	a := map[string]interface{}{"key": "value", "old": "removed"}
	b := map[string]interface{}{"key": "value"}
	changes := mustCompare(t, a, b)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != "delete" {
		t.Errorf("expected type 'delete', got %q", changes[0].Type)
	}
}

func TestCompare_UpdatedValue(t *testing.T) {
	a := map[string]interface{}{"host": "localhost"}
	b := map[string]interface{}{"host": "production.example.com"}
	changes := mustCompare(t, a, b)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != "update" {
		t.Errorf("expected type 'update', got %q", changes[0].Type)
	}
	if changes[0].From != "localhost" {
		t.Errorf("expected From='localhost', got %v", changes[0].From)
	}
	if changes[0].To != "production.example.com" {
		t.Errorf("expected To='production.example.com', got %v", changes[0].To)
	}
}

func TestCompare_NestedPath(t *testing.T) {
	a := map[string]interface{}{"database": map[string]interface{}{"host": "localhost"}}
	b := map[string]interface{}{"database": map[string]interface{}{"host": "remote"}}
	changes := mustCompare(t, a, b)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if !strings.Contains(changes[0].Path, "database") {
		t.Errorf("expected path to contain 'database', got %q", changes[0].Path)
	}
	if !strings.Contains(changes[0].Path, "host") {
		t.Errorf("expected path to contain 'host', got %q", changes[0].Path)
	}
}

// formatPath

func TestFormatPath_Empty(t *testing.T) {
	got := formatPath([]string{})
	if got != "." {
		t.Errorf("formatPath([]) = %q, want %q", got, ".")
	}
}

func TestFormatPath_SimpleKeys(t *testing.T) {
	got := formatPath([]string{"database", "host"})
	if got != ".database.host" {
		t.Errorf("formatPath = %q, want %q", got, ".database.host")
	}
}

func TestFormatPath_ArrayIndex(t *testing.T) {
	got := formatPath([]string{"items", "0", "name"})
	if got != ".items[0].name" {
		t.Errorf("formatPath = %q, want %q", got, ".items[0].name")
	}
}

// Print

func TestPrint_NoDiffs(t *testing.T) {
	var sb strings.Builder
	Print(&sb, []Change{}, false)
	if !strings.Contains(sb.String(), "No differences found") {
		t.Errorf("expected 'No differences found', got %q", sb.String())
	}
}

func TestPrint_Create(t *testing.T) {
	var sb strings.Builder
	changes := []Change{{Type: "create", Path: ".key", To: "value"}}
	Print(&sb, changes, false)
	out := sb.String()
	if !strings.HasPrefix(out, "+") {
		t.Errorf("create line should start with '+', got %q", out)
	}
	if !strings.Contains(out, ".key") {
		t.Errorf("expected '.key' in output, got %q", out)
	}
}

func TestPrint_Delete(t *testing.T) {
	var sb strings.Builder
	changes := []Change{{Type: "delete", Path: ".key", From: "value"}}
	Print(&sb, changes, false)
	out := sb.String()
	if !strings.HasPrefix(out, "-") {
		t.Errorf("delete line should start with '-', got %q", out)
	}
}

func TestPrint_Update(t *testing.T) {
	var sb strings.Builder
	changes := []Change{{Type: "update", Path: ".key", From: "old", To: "new"}}
	Print(&sb, changes, false)
	out := sb.String()
	if !strings.HasPrefix(out, "~") {
		t.Errorf("update line should start with '~', got %q", out)
	}
	if !strings.Contains(out, "→") {
		t.Errorf("update line should contain '→', got %q", out)
	}
}

func TestPrint_Color(t *testing.T) {
	var sb strings.Builder
	changes := []Change{
		{Type: "create", Path: ".a", To: "v"},
		{Type: "delete", Path: ".b", From: "v"},
		{Type: "update", Path: ".c", From: "x", To: "y"},
	}
	Print(&sb, changes, true)
	out := sb.String()
	// Color output should contain ANSI escape codes.
	if !strings.Contains(out, "\033[") {
		t.Errorf("color output should contain ANSI codes, got %q", out)
	}
}

// formatVal

func TestFormatVal_Nil(t *testing.T) {
	if got := formatVal(nil); got != "null" {
		t.Errorf("formatVal(nil) = %q, want %q", got, "null")
	}
}

func TestFormatVal_String(t *testing.T) {
	if got := formatVal("hello"); got != `"hello"` {
		t.Errorf("formatVal(string) = %q, want %q", got, `"hello"`)
	}
}

func TestFormatVal_Bool(t *testing.T) {
	if got := formatVal(true); got != "true" {
		t.Errorf("formatVal(true) = %q, want %q", got, "true")
	}
}
