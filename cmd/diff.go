package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/troll-warlord/anyq/internal/diff"
)

var diffExitStatus bool

var diffCmd = &cobra.Command{
	Use:   "diff <file1> <file2>",
	Short: "Semantically diff two JSON/YAML/TOML files",
	Long: `Compare two files semantically — key order, indentation, and format
differences are ignored. Only actual data changes are reported.

Files can be in any combination of formats (e.g. diff a YAML file against
a JSON file). The output uses jq-style paths so every change is directly
usable in an anyq query.

Symbols in output:
  +  key or value was added
  -  key or value was removed
  ~  value was changed

Examples:
  anyq diff old.yaml new.yaml
  anyq diff config.json config.yaml          # cross-format diff
  anyq diff --exit-status before.toml after.toml`,

	Args:         cobra.ExactArgs(2),
	SilenceUsage: true,
	RunE:         runDiff,
}

func init() {
	diffCmd.Flags().BoolVarP(&diffExitStatus, "exit-status", "e", false, "exit 1 if the files differ (useful in CI)")
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	a, err := parseFile(args[0])
	if err != nil {
		return fmt.Errorf("file 1: %w", err)
	}
	b, err := parseFile(args[1])
	if err != nil {
		return fmt.Errorf("file 2: %w", err)
	}

	changes, err := diff.Compare(a, b)
	if err != nil {
		return err
	}

	diff.Print(cmd.OutOrStdout(), changes, colorEnabled(false))

	if diffExitStatus && len(changes) > 0 {
		os.Exit(1)
	}
	return nil
}
