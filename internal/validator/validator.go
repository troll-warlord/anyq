// Package validator validates parsed documents against a JSON Schema.
// Supports JSON Schema drafts 4 through 2020-12, auto-detected from the
// schema's "$schema" field.
package validator

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Result is returned by Validate.
type Result struct {
	Valid  bool
	Errors []string // human-readable messages, one per failing constraint; empty when Valid
}

// Validate checks data (the output of engine.Parse) against the JSON Schema
// stored in schemaPath (a local file path).
//
// Returns a non-nil Result for both valid and invalid data.
// Returns a non-nil error only for I/O or schema compilation failures.
func Validate(data interface{}, schemaPath string) (*Result, error) {
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read schema %q: %w", schemaPath, err)
	}
	// Strip UTF-8 BOM — Windows tools (PowerShell, Notepad) often add it.
	if len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF {
		content = content[3:]
	}

	compiler := jsonschema.NewCompiler()

	// Register the schema under a stable ID; this allows $ref within the schema
	// to resolve relative to this identifier.
	const resourceID = "schema.json"
	if err := compiler.AddResource(resourceID, bytes.NewReader(content)); err != nil {
		return nil, fmt.Errorf("invalid schema %q: %w", schemaPath, err)
	}

	schema, err := compiler.Compile(resourceID)
	if err != nil {
		return nil, fmt.Errorf("cannot compile schema %q: %w", schemaPath, err)
	}

	if err := schema.Validate(data); err != nil {
		verr, ok := err.(*jsonschema.ValidationError)
		if !ok {
			return nil, fmt.Errorf("validation: %w", err)
		}
		return &Result{Valid: false, Errors: flattenErrors(verr)}, nil
	}

	return &Result{Valid: true}, nil
}

// flattenErrors walks the ValidationError tree and collects all leaf messages.
// Only leaf nodes (no Causes) carry distinct constraint failures; parent nodes
// are structural groupings and are skipped.
func flattenErrors(err *jsonschema.ValidationError) []string {
	if len(err.Causes) == 0 {
		loc := jsonPtrToJQ(err.InstanceLocation)
		return []string{fmt.Sprintf("%s: %s", loc, err.Message)}
	}
	var msgs []string
	for _, cause := range err.Causes {
		msgs = append(msgs, flattenErrors(cause)...)
	}
	return msgs
}

// jsonPtrToJQ converts a JSON Pointer ("/spec/replicas") to a jq path (".spec.replicas").
// Array indices become [n] accessors: "/items/0/name" → ".items[0].name".
func jsonPtrToJQ(pointer string) string {
	if pointer == "" || pointer == "/" {
		return "."
	}
	parts := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	var b strings.Builder
	for _, part := range parts {
		// Unescape JSON Pointer escaped characters.
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")
		if isDigitString(part) {
			fmt.Fprintf(&b, "[%s]", part)
		} else {
			fmt.Fprintf(&b, ".%s", part)
		}
	}
	return b.String()
}

func isDigitString(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
