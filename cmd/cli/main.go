package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yashkhare/living-docs/internal/analyzer"
	"github.com/yashkhare/living-docs/internal/config"
	"github.com/yashkhare/living-docs/internal/llm"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "docs-agent",
	Short: "Living Documentation Agent - keeps your docs in sync with code",
	Long: `docs-agent is a CLI tool that reads your source code and documentation,
detects discrepancies, and generates updated documentation using AI.

It acts as a "docs janitor" — fixing broken docs so they stay in sync with code.`,
}

var (
	codeFile   string
	docFile    string
	outputFile string
	writeBack  bool
	skipCheck  bool
)

var fixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Analyze code and update documentation to match",
	Long: `The fix command reads a code file and a documentation file, uses an LLM to
determine if the documentation is outdated, and generates an updated version.

Examples:
  docs-agent fix --code=main.go --doc=README.md
  docs-agent fix --code=handler.go --doc=docs/api.md --write
  docs-agent fix --code=main.go --doc=README.md --output=updated-readme.md`,
	RunE: runFix,
}

var findDocsCmd = &cobra.Command{
	Use:   "find-docs",
	Short: "Find documentation files linked to a code file",
	Long: `The find-docs command uses heuristics to locate documentation files
that are likely related to the given code file.

Example:
  docs-agent find-docs --code=internal/handler.go --root=.`,
	RunE: runFindDocs,
}

var (
	repoRoot string
)

func init() {
	fixCmd.Flags().StringVar(&codeFile, "code", "", "Path to the code file to analyze (required)")
	fixCmd.Flags().StringVar(&docFile, "doc", "", "Path to the documentation file to update (required)")
	fixCmd.Flags().StringVar(&outputFile, "output", "", "Path to write the updated documentation (default: stdout)")
	fixCmd.Flags().BoolVar(&writeBack, "write", false, "Overwrite the original doc file in-place")
	fixCmd.Flags().BoolVar(&skipCheck, "skip-check", false, "Skip the classification step and always update")
	_ = fixCmd.MarkFlagRequired("code")
	_ = fixCmd.MarkFlagRequired("doc")

	findDocsCmd.Flags().StringVar(&codeFile, "code", "", "Path to the code file (required)")
	findDocsCmd.Flags().StringVar(&repoRoot, "root", ".", "Repository root directory")
	_ = findDocsCmd.MarkFlagRequired("code")

	rootCmd.AddCommand(fixCmd)
	rootCmd.AddCommand(findDocsCmd)
}

func runFix(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.MustLoadForCLI()
	if err != nil {
		return err
	}

	// Read the code file
	codeContent, err := os.ReadFile(codeFile)
	if err != nil {
		return fmt.Errorf("failed to read code file %s: %w", codeFile, err)
	}

	// Read the doc file
	docContent, err := os.ReadFile(docFile)
	if err != nil {
		return fmt.Errorf("failed to read doc file %s: %w", docFile, err)
	}

	// Initialize LLM client
	llmClient := llm.NewClient(cfg.OpenAIAPIKey, cfg.LLMModel)

	// Step 1: Classify whether docs need updating (unless skipped)
	if !skipCheck {
		fmt.Fprintf(os.Stderr, "🔍 Analyzing code for documentation-relevant changes...\n")
		needsUpdate, reason, err := llmClient.ClassifyChange(ctx, string(codeContent))
		if err != nil {
			return fmt.Errorf("classification failed: %w", err)
		}

		fmt.Fprintf(os.Stderr, "📋 Classification: %s\n", reason)

		if !needsUpdate {
			fmt.Fprintf(os.Stderr, "✅ Documentation appears to be up-to-date. No changes needed.\n")
			return nil
		}
	}

	// Step 2: Generate updated documentation
	fmt.Fprintf(os.Stderr, "📝 Generating updated documentation...\n")
	updatedDoc, err := llmClient.UpdateDocumentation(ctx, string(codeContent), string(docContent))
	if err != nil {
		return fmt.Errorf("documentation update failed: %w", err)
	}

	// Ensure the doc ends with a newline
	if !strings.HasSuffix(updatedDoc, "\n") {
		updatedDoc += "\n"
	}

	// Step 3: Output the result
	if writeBack {
		if err := os.WriteFile(docFile, []byte(updatedDoc), 0644); err != nil {
			return fmt.Errorf("failed to write updated doc to %s: %w", docFile, err)
		}
		fmt.Fprintf(os.Stderr, "✅ Updated documentation written to %s\n", docFile)
	} else if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(updatedDoc), 0644); err != nil {
			return fmt.Errorf("failed to write updated doc to %s: %w", outputFile, err)
		}
		fmt.Fprintf(os.Stderr, "✅ Updated documentation written to %s\n", outputFile)
	} else {
		fmt.Print(updatedDoc)
	}

	return nil
}

func runFindDocs(cmd *cobra.Command, args []string) error {
	changedFiles := analyzer.ParseChangedFiles([]string{codeFile})
	candidates := analyzer.FindLinkedDocsForFiles(changedFiles, repoRoot)

	if len(candidates) == 0 {
		fmt.Println("No documentation files found linked to the code file.")
		return nil
	}

	fmt.Printf("Found %d documentation candidate(s):\n\n", len(candidates))
	for i, c := range candidates {
		fmt.Printf("  %d. %s\n     Priority: %d | Reason: %s\n\n", i+1, c.Path, c.Priority, c.Reason)
	}

	return nil
}
