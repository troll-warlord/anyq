package detector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFromExtension(t *testing.T) {
	tests := []struct {
		path string
		want Format
	}{
		{"config.json", FormatJSON},
		{"config.JSON", FormatJSON},
		{"data.yaml", FormatYAML},
		{"data.yml", FormatYAML},
		{"data.YAML", FormatYAML},
		{"config.toml", FormatTOML},
		{"config.TOML", FormatTOML},
		{"config.txt", FormatUnknown},
		{"config", FormatUnknown},
		{"config.xml", FormatUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := fromExtension(tt.path)
			if got != tt.want {
				t.Errorf("fromExtension(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestFromBytes_JSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Format
	}{
		{"object", `{"key":"value"}`, FormatJSON},
		{"array", `[1,2,3]`, FormatJSON},
		{"pretty object", "{\n  \"key\": \"value\"\n}", FormatJSON},
		{"with leading whitespace", "  {\"key\":\"value\"}", FormatJSON},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromBytes([]byte(tt.input))
			if got != tt.want {
				t.Errorf("FromBytes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFromBytes_YAML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Format
	}{
		{"document marker", "---\nkey: value", FormatYAML},
		{"key value", "key: value\nother: 123", FormatYAML},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromBytes([]byte(tt.input))
			if got != tt.want {
				t.Errorf("FromBytes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFromBytes_TOML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Format
	}{
		{"section header", "[database]\nhost = \"localhost\"", FormatTOML},
		{"key = value", "host = \"localhost\"\nport = 5432", FormatTOML},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromBytes([]byte(tt.input))
			if got != tt.want {
				t.Errorf("FromBytes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFromBytes_Unknown(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"whitespace only", "   \n\t  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromBytes([]byte(tt.input))
			if got != FormatUnknown {
				t.Errorf("FromBytes(%q) = %q, want %q", tt.input, got, FormatUnknown)
			}
		})
	}
}

func TestFromPath_ByExtension(t *testing.T) {
	tests := []struct {
		filename string
		content  string
		want     Format
	}{
		{"test.json", `{"key":"value"}`, FormatJSON},
		{"test.yaml", "key: value", FormatYAML},
		{"test.toml", "key = \"value\"", FormatTOML},
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.filename)
			if err := os.WriteFile(path, []byte(tt.content), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := FromPath(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("FromPath(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestFromPath_ContentFallback(t *testing.T) {
	// File with no recognizable extension — should fall back to content sniffing.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.conf")
	if err := os.WriteFile(path, []byte(`{"key":"value"}`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := FromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != FormatJSON {
		t.Errorf("FromPath content fallback = %q, want %q", got, FormatJSON)
	}
}

func TestFromPath_MissingFile(t *testing.T) {
	_, err := FromPath("/nonexistent/path/file.conf")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
