// Package ai translates natural language queries into jq expressions.
// It supports multiple LLM backends, selected via the ANYQ_AI_PROVIDER env var.
//
// Supported providers:
//   - openai   (default) — requires OPENAI_API_KEY
//   - anthropic           — requires ANTHROPIC_API_KEY
//   - gemini              — requires GEMINI_API_KEY
//   - ollama              — requires a running Ollama instance (OLLAMA_BASE_URL, default http://localhost:11434)
//
// The model used by each provider can be overridden with ANYQ_AI_MODEL.
package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/troll-warlord/anyq/internal/detector"
	"github.com/troll-warlord/anyq/internal/engine"
)

// Provider translates a natural-language request into a jq expression.
type Provider interface {
	Translate(schema, request string) (string, error)
}

// TranslateWithValidation calls Translate and validates the result using the
// provided compile function (typically gojq.Parse). If the expression fails
// to compile, it retries up to maxRetries times, feeding the error back to
// the model so it can self-correct.
func TranslateWithValidation(p Provider, schema, request string, validate func(string) error, maxRetries int) (string, error) {
	expr, err := p.Translate(schema, request)
	if err != nil {
		return "", err
	}

	for i := 0; i < maxRetries; i++ {
		if err := validate(expr); err == nil {
			return expr, nil
		} else {
			// Ask the model to fix its own mistake.
			fixRequest := fmt.Sprintf(
				"The jq expression you produced was INVALID:\n  expression: %s\n  error: %s\n\nFix it. Output ONLY the corrected raw jq expression, nothing else.",
				expr, err.Error(),
			)
			expr, err = p.Translate(schema, fixRequest)
			if err != nil {
				return "", err
			}
		}
	}

	// Final validation — return the expression and the compile error together
	// so the caller can decide what to do.
	if err := validate(expr); err != nil {
		return expr, fmt.Errorf("generated query failed to compile after %d retries: %w", maxRetries, err)
	}
	return expr, nil
}

// New returns a Provider based on the ANYQ_AI_PROVIDER environment variable.
// Falls back to "openai" when the variable is unset.
func New() (Provider, error) {
	provider := os.Getenv("ANYQ_AI_PROVIDER")
	if provider == "" {
		provider = "openai"
	}

	model := os.Getenv("ANYQ_AI_MODEL")

	switch provider {
	case "openai":
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY is not set (required for ANYQ_AI_PROVIDER=openai)")
		}
		if model == "" {
			model = "gpt-4o-mini"
		}
		return &openAIProvider{apiKey: key, model: model}, nil

	case "anthropic":
		key := os.Getenv("ANTHROPIC_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set (required for ANYQ_AI_PROVIDER=anthropic)")
		}
		if model == "" {
			model = "claude-3-5-haiku-20241022"
		}
		return &anthropicProvider{apiKey: key, model: model}, nil

	case "gemini":
		key := os.Getenv("GEMINI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY is not set (required for ANYQ_AI_PROVIDER=gemini)")
		}
		if model == "" {
			model = "gemini-2.0-flash"
		}
		return &geminiProvider{apiKey: key, model: model}, nil

	case "ollama":
		baseURL := os.Getenv("OLLAMA_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		if model == "" {
			model = "qwen2.5-coder"
		}
		return &ollamaProvider{baseURL: baseURL, model: model}, nil

	default:
		return nil, fmt.Errorf("unsupported ANYQ_AI_PROVIDER %q; choose: openai, anthropic, gemini, ollama", provider)
	}
}

// SystemPrompt is the shared system role instruction sent to every provider.
const SystemPrompt = `You are a "Natural Language to JQ" translator for the anyq CLI tool.
Your task is to analyze a JSON sample and a user's intent to produce a functional jq filter.

CRITICAL CONSTRAINTS:
- Output ONLY the raw jq expression. No backticks, no code blocks, no explanations, no markdown.
- jq is NOT JavaScript. The following are INVALID jq and must NEVER be used:
    .map()  .filter()  .forEach()  .reduce()  .find()  =>  function()
- Use jq-native idioms:
    Filter array:   .array[] | select(.field == "value")
    Pluck field:    .object.field
    Array of vals:  [.array[] | select(.flag) | .name]
    Count:          .array | length
    Keys:           keys[]  or  to_entries[]
- Use field names EXACTLY as they appear in the provided JSON sample.
- If the request is unclear or cannot be expressed as jq, return . (the identity filter).

EXAMPLES (do not output the comments, only the expression):
  Request: "names of all admins"
  Data has: {"users":[{"name":"alice","role":"admin"}]}
  Output: [.users[] | select(.role == "admin") | .name]

  Request: "count of items"
  Data has: {"items":[1,2,3]}
  Output: .items | length

  Request: "all enabled features"
  Data has: {"features":[{"name":"x","enabled":true}]}
  Output: [.features[] | select(.enabled) | .name]`

var userPromptTmpl = template.Must(template.New("prompt").Parse(
	`JSON SAMPLE (values sanitized to protect sensitive data):
{{.Schema}}

USER REQUEST:
{{.Request}}`))

type promptData struct {
	Schema  string
	Request string
}

// BuildUserPrompt renders the user-turn prompt from a schema and request.
func BuildUserPrompt(schema, request string) (string, error) {
	var buf bytes.Buffer
	if err := userPromptTmpl.Execute(&buf, promptData{Schema: schema, Request: request}); err != nil {
		return "", fmt.Errorf("prompt template: %w", err)
	}
	return buf.String(), nil
}

// ExtractSchema parses data, sanitises sensitive leaf values, and returns a
// compact JSON string safe to send as AI context.
// Short, enum-like string values (roles, statuses, types) are kept as-is because
// the model needs to see them to write correct select() filters.
// Long strings, emails, UUIDs, and token-like values are redacted.
// If the data is not valid JSON, a raw truncated snippet is returned instead —
// this covers YAML and TOML inputs which the AI can still understand structurally.
func ExtractSchema(data []byte) (string, error) {
	if len(data) == 0 {
		return "{}", nil
	}

	// Detect format and parse through the engine so YAML and TOML are
	// normalised to map[string]interface{} — the same structure JSON produces.
	fmt_ := detector.FromBytes(data)
	if fmt_ == detector.FormatUnknown {
		fmt_ = detector.FormatJSON
	}

	var v interface{}
	var err error
	v, err = engine.Parse(data, fmt_)
	if err != nil {
		// Unparseable — send a safe raw snippet so the model has at least some context.
		snippet := data
		if len(snippet) > 800 {
			snippet = snippet[:800]
		}
		return string(snippet), nil
	}

	sanitised := sanitise(v, 0)
	b, err := json.MarshalIndent(sanitised, "", "  ")
	if err != nil {
		return "", err
	}
	// Cap at ~3000 chars to stay within typical context limits.
	if len(b) > 3000 {
		b = b[:3000]
	}
	return string(b), nil
}

// sensitivePattern matches field names that likely hold secrets.
var sensitivePattern = regexp.MustCompile(
	`(?i)(password|secret|token|key|auth|bearer|credential|private)`,
)

// sanitise recursively builds a schema-only representation:
// every leaf value is replaced with its Go type name ("string", "number", "boolean").
// Arrays are sampled to at most 3 elements so the model sees structural variety.
// Sensitive field names (password, token, etc.) are replaced with "<redacted>".
func sanitise(v interface{}, depth int) interface{} {
	if depth > 8 {
		return "..."
	}
	switch val := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, vv := range val {
			if sensitivePattern.MatchString(k) {
				out[k] = "<redacted>"
			} else {
				out[k] = sanitise(vv, depth+1)
			}
		}
		return out
	case []interface{}:
		if len(val) == 0 {
			return []interface{}{}
		}
		max := 3
		if len(val) < max {
			max = len(val)
		}
		out := make([]interface{}, max)
		for i := 0; i < max; i++ {
			out[i] = sanitise(val[i], depth+1)
		}
		return out
	case string:
		return "string"
	case float64:
		return "number"
	case json.Number:
		return "number"
	case bool:
		return "boolean"
	default:
		return "null"
	}
}

// CleanExpr strips common markdown wrappers that models sometimes add
// despite being instructed not to: triple-backtick fences and single backticks.
func CleanExpr(expr string) string {
	expr = strings.TrimSpace(expr)
	// Strip triple-backtick opening fence (```jq, ```json, ```)
	for _, fence := range []string{"```jq", "```json", "```"} {
		if strings.HasPrefix(expr, fence) {
			expr = strings.TrimPrefix(expr, fence)
			break
		}
	}
	// Strip triple-backtick closing fence
	expr = strings.TrimSuffix(expr, "```")
	expr = strings.TrimSpace(expr)
	// Strip single backtick wrapping: `expr`
	if strings.HasPrefix(expr, "`") && strings.HasSuffix(expr, "`") && len(expr) > 1 {
		expr = expr[1 : len(expr)-1]
	}
	expr = strings.TrimSpace(expr)
	return fixCommonMistakes(expr)
}

// fixCommonMistakes corrects well-known model errors that still parse as valid jq
// but produce wrong results at runtime.
//
// Pattern: .[<expr containing |>]  →  [<expr>]
// Models frequently wrap array-building pipes in .[...] (object index syntax)
// instead of [...] (array constructor syntax).
func fixCommonMistakes(expr string) string {
	// .[...] where inner expression contains a pipe → array constructor
	if strings.HasPrefix(expr, ".[") && strings.HasSuffix(expr, "]") {
		inner := expr[2 : len(expr)-1]
		if strings.Contains(inner, "|") {
			return "[" + inner + "]"
		}
	}
	return expr
}
