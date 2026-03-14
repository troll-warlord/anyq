// Package cmd implements the anyq CLI using cobra.
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"
	"github.com/troll-warlord/anyq/internal/ai"
	"github.com/troll-warlord/anyq/internal/detector"
	"github.com/troll-warlord/anyq/internal/engine"
	"github.com/troll-warlord/anyq/internal/highlight"
	"golang.org/x/term"
)

// Version, Commit, and Date are set at build time via -ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var (
	inputFormat  string
	outputFormat string
	pretty       bool
	rawOutput    bool
	compact      bool
	nullInput    bool
	exitStatus   bool
	inputFile    string
	outputFile   string
	noColor      bool
	aiQuery      string
	showQuery    bool
)

var rootCmd = &cobra.Command{
	Use:   "anyq [flags] <jq-expression> [file...]",
	Short: "anyq — query JSON, YAML, and TOML with jq syntax",
	Long: `anyq is a unified command-line processor for JSON, YAML, and TOML.
It uses full jq expression syntax and auto-detects the input format.

Examples:
  anyq '.database.host' config.yaml
  anyq '.[] | select(.age > 30)' users.json
  cat config.toml | anyq '.server.port'
  anyq -o yaml '.database' config.json
  anyq --pretty '.' config.yaml
  anyq --ai "show all users older than 30" users.json`,

	Version:      Version,
	Args:         cobra.ArbitraryArgs,
	SilenceUsage: true,
	RunE:         run,
}

// Execute is the entry point called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// cobra already printed the error; just set exit code.
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&inputFormat, "input-format", "f", "", "input format: json|yaml|toml (auto-detected by default)")
	rootCmd.Flags().StringVarP(&outputFormat, "output-format", "o", "", "output format: json|yaml|toml (defaults to input format)")
	rootCmd.Flags().BoolVar(&pretty, "pretty", true, "pretty-print JSON output (always on for YAML/TOML)")
	rootCmd.Flags().BoolVarP(&rawOutput, "raw-output", "r", false, "output strings without JSON quotes")
	rootCmd.Flags().BoolVarP(&compact, "compact", "c", false, "compact JSON output (no whitespace)")
	rootCmd.Flags().BoolVarP(&nullInput, "null-input", "n", false, "use null as input; do not read any input")
	rootCmd.Flags().BoolVarP(&exitStatus, "exit-status", "e", false, "exit 1 if the last output is false or null")
	rootCmd.Flags().StringVarP(&inputFile, "input", "i", "", "input file (alternative to positional file argument)")
	rootCmd.Flags().StringVarP(&outputFile, "write-output", "w", "", "write output to file instead of stdout")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output (also honoured via NO_COLOR env var)")
	rootCmd.Flags().StringVar(&aiQuery, "ai", "", "natural language query translated to jq via AI (set ANYQ_AI_PROVIDER + API key)")
	rootCmd.Flags().BoolVar(&showQuery, "show-query", false, "print the AI-generated query and exit without running")
}

// colorEnabled reports whether ANSI color output should be used.
// Color is on by default when stdout is a TTY and NO_COLOR is not set.
// Writing to a file (--write-output) always disables color.
func colorEnabled(writingToFile bool) bool {
	if writingToFile || noColor {
		return false
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd())) // #nosec G115 -- fd is always a small non-negative integer
}

func run(cmd *cobra.Command, args []string) error {
	// Determine jq expression and file paths.
	// In --ai mode the jq expression is generated; all positional args are file paths.
	// In normal mode args[0] is the jq expression.
	var query string
	var filePaths []string

	if aiQuery != "" {
		filePaths = args
	} else {
		if len(args) == 0 {
			return fmt.Errorf("requires a jq expression as the first argument, or use --ai")
		}
		query = args[0]
		filePaths = args[1:]
	}

	// --input / -i flag overrides positional file argument.
	if inputFile != "" {
		filePaths = append([]string{inputFile}, filePaths...)
	}

	// Parse --input-format and --output-format flags.
	inFmt := parseFormat(inputFormat)
	outFmt := parseFormat(outputFormat)

	// Validate conflicting flags.
	if pretty && compact {
		return fmt.Errorf("--pretty and --compact are mutually exclusive")
	}

	opts := engine.Options{
		InputFormat:  inFmt,
		OutputFormat: outFmt,
		Pretty:       pretty || outFmt == detector.FormatJSON, // JSON defaults to pretty
		RawOutput:    rawOutput,
		Compact:      compact,
		NullInput:    nullInput,
		ExitStatus:   exitStatus,
		Color:        colorEnabled(outputFile != ""),
	}

	// Resolve output writer.
	out := cmd.OutOrStdout()
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("cannot open output file %q: %w", outputFile, err)
		}
		defer f.Close()
		out = f
	}

	// AI mode: sample data → extract schema → translate NL → jq expression.
	// When stdin is used as the data source it is buffered here so it can be
	// processed again after the query is generated.
	var bufferedStdin []byte
	if aiQuery != "" {
		var sampleData []byte
		if len(filePaths) > 0 {
			var err error
			sampleData, err = os.ReadFile(filePaths[0])
			if err != nil {
				return fmt.Errorf("cannot read file %q: %w", filePaths[0], err)
			}
		} else if !nullInput {
			var err error
			bufferedStdin, err = io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			sampleData = bufferedStdin
		}

		schema, err := ai.ExtractSchema(sampleData)
		if err != nil {
			return fmt.Errorf("schema extraction: %w", err)
		}

		provider, err := ai.New()
		if err != nil {
			return err
		}

		expr, err := ai.TranslateWithValidation(provider, schema, aiQuery, func(e string) error {
			_, err := gojq.Parse(e)
			return err
		}, 2)
		if err != nil {
			return fmt.Errorf("AI translation failed: %w", err)
		}

		// Safety check: always show the generated expression.
		useColor := colorEnabled(outputFile != "")
		if useColor {
			fmt.Fprintf(os.Stderr, highlight.Green+"✦ Generated query: "+highlight.Reset+"%s\n", expr)
		} else {
			fmt.Fprintf(os.Stderr, "✦ Generated query: %s\n", expr)
		}

		if showQuery {
			fmt.Fprintln(cmd.OutOrStdout(), expr)
			return nil
		}

		// Interactive confirmation when running in a terminal.
		if term.IsTerminal(int(os.Stdin.Fd())) { // #nosec G115
			fmt.Fprint(os.Stderr, "Run? [Y/n]: ")
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "" && response != "y" {
				return nil
			}
		}

		query = expr

		// If stdin was buffered, process it immediately and return.
		if bufferedStdin != nil {
			if inFmt == detector.FormatUnknown || inFmt == "" {
				opts.InputFormat = detector.FromBytes(bufferedStdin)
			}
			return runQuery(out, query, bufferedStdin, opts)
		}
	}

	// No file paths → read from stdin.
	if len(filePaths) == 0 && !nullInput {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
		// Auto-detect from content when reading stdin.
		if inFmt == detector.FormatUnknown {
			opts.InputFormat = detector.FromBytes(data)
		}
		return runQuery(out, query, data, opts)
	}

	// Null input mode: run once with no data.
	if nullInput {
		return runQuery(out, query, nil, opts)
	}

	// Process each file.
	for _, path := range filePaths {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("cannot read file %q: %w", path, err)
		}
		// Per-file format detection when not overridden.
		fileOpts := opts
		if inFmt == detector.FormatUnknown {
			detectedFmt, err := detector.FromPath(path)
			if err != nil {
				detectedFmt = detector.FromBytes(data)
			}
			fileOpts.InputFormat = detectedFmt
			// If output format was not explicitly set, track to input.
			if outFmt == detector.FormatUnknown {
				fileOpts.OutputFormat = detectedFmt
			}
		}
		if err := runQuery(out, query, data, fileOpts); err != nil {
			return err
		}
	}

	return nil
}

func runQuery(w io.Writer, query string, data []byte, opts engine.Options) error {
	err := engine.Run(nil, w, query, data, opts)
	if err == engine.ErrExitStatus {
		os.Exit(1)
	}
	return err
}

// parseFormat converts a user-supplied string to a detector.Format.
func parseFormat(s string) detector.Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return detector.FormatJSON
	case "yaml", "yml":
		return detector.FormatYAML
	case "toml":
		return detector.FormatTOML
	default:
		return detector.FormatUnknown
	}
}
