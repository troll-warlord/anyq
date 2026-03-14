package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/troll-warlord/anyq/internal/highlight"
	"github.com/troll-warlord/anyq/internal/validator"
)

var validateSchema string

var validateCmd = &cobra.Command{
	Use:   "validate --schema <schema.json> <file>",
	Short: "Validate a JSON/YAML/TOML file against a JSON Schema",
	Long: `Validate a data file against a local JSON Schema file.
Supports JSON Schema drafts 4, 6, 7, 2019-09, and 2020-12
(auto-detected from the schema's "$schema" field).

Exit codes:
  0  valid
  1  validation failed (errors printed to stderr)
  2  usage or I/O error

Perfect as a CI gatekeeper — a non-zero exit from this command will
fail a GitHub Actions step and block a merge.

Examples:
  anyq validate --schema pod.schema.json deployment.yaml
  anyq validate -s openapi-schema.json api-config.json
  anyq validate --schema schema.json data.toml`,

	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runValidate,
}

func init() {
	validateCmd.Flags().StringVarP(&validateSchema, "schema", "s", "", "path to JSON Schema file (required)")
	_ = validateCmd.MarkFlagRequired("schema")
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	parsed, err := parseFile(args[0])
	if err != nil {
		return err
	}

	result, err := validator.Validate(parsed, validateSchema)
	if err != nil {
		return err
	}

	useColor := colorEnabled(false)

	if result.Valid {
		if useColor {
			fmt.Fprintf(cmd.OutOrStdout(), highlight.Green+"✓ Valid"+highlight.Reset+"\n")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "✓ Valid")
		}
		return nil
	}

	// Print structured errors to stderr and exit 1.
	// We print ourselves rather than returning an error so cobra does not
	// add its own "Error: ..." prefix on top of our formatted output.
	if useColor {
		fmt.Fprintf(os.Stderr, highlight.Red+"✗ Validation failed: %d error(s)"+highlight.Reset+"\n\n", len(result.Errors))
	} else {
		fmt.Fprintf(os.Stderr, "✗ Validation failed: %d error(s)\n\n", len(result.Errors))
	}
	for _, e := range result.Errors {
		fmt.Fprintf(os.Stderr, "  • %s\n", e)
	}
	os.Exit(1)
	return nil
}
