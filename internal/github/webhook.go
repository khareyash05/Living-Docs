package github

import (
	"context"
	"fmt"
	"log"

	"github.com/khareyash05/living-docs/internal/analyzer"
	"github.com/khareyash05/living-docs/internal/llm"
)

// WebhookHandler processes GitHub webhook events for documentation updates.
type WebhookHandler struct {
	llmClient *llm.Client
}

// NewWebhookHandler creates a new webhook event handler.
func NewWebhookHandler(llmClient *llm.Client) *WebhookHandler {
	return &WebhookHandler{
		llmClient: llmClient,
	}
}

// HandleMergedPR processes a merged pull request event.
// It fetches the diff, finds linked docs, and creates update PRs if needed.
func (h *WebhookHandler) HandleMergedPR(ctx context.Context, ghClient *Client, prNumber int, baseBranch string) error {
	log.Printf("[webhook] Processing merged PR #%d", prNumber)

	// Step 1: Fetch the PR diff
	diff, err := ghClient.GetPRDiff(ctx, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get PR diff: %w", err)
	}

	log.Printf("[webhook] PR #%d has %d changed files", prNumber, len(diff.Files))

	// Step 2: Identify code files that changed
	filenames := make([]string, 0, len(diff.Files))
	for _, f := range diff.Files {
		filenames = append(filenames, f.Filename)
	}
	changedFiles := analyzer.ParseChangedFiles(filenames)
	codeFiles := analyzer.FilterCodeFiles(changedFiles)

	if len(codeFiles) == 0 {
		log.Printf("[webhook] PR #%d has no code file changes, skipping", prNumber)
		return nil
	}

	log.Printf("[webhook] PR #%d has %d code file changes", prNumber, len(codeFiles))

	// Step 3: Find linked documentation files
	// NOTE: In a full implementation, we'd use the GitHub API to list repo files
	// and apply the doc-finder heuristic. For now, we check common locations.
	docPaths := findCommonDocPaths(codeFiles)

	if len(docPaths) == 0 {
		log.Printf("[webhook] No documentation files found for PR #%d", prNumber)
		return nil
	}

	// Step 4: For each doc file, check if it needs updating
	for _, docPath := range docPaths {
		if err := h.processDocFile(ctx, ghClient, prNumber, baseBranch, diff, docPath); err != nil {
			log.Printf("[webhook] Error processing doc %s for PR #%d: %v", docPath, prNumber, err)
			// Continue with other doc files
		}
	}

	return nil
}

// processDocFile checks if a single documentation file needs updating and creates a PR if so.
func (h *WebhookHandler) processDocFile(ctx context.Context, ghClient *Client, prNumber int, baseBranch string, diff *PRDiff, docPath string) error {
	log.Printf("[webhook] Checking doc file: %s", docPath)

	// Fetch the current doc content
	docContent, err := ghClient.GetFileContent(ctx, docPath, baseBranch)
	if err != nil {
		return fmt.Errorf("failed to get doc content: %w", err)
	}

	// Build a combined code context from all changed files
	var codeContext string
	for _, f := range diff.Files {
		if analyzer.IsCodeFile(analyzer.ParseChangedFiles([]string{f.Filename})[0].Extension) {
			codeContext += fmt.Sprintf("--- File: %s ---\n%s\n\n", f.Filename, f.Patch)
		}
	}

	// Classify whether the changes warrant a doc update
	needsUpdate, reason, err := h.llmClient.ClassifyChange(ctx, codeContext)
	if err != nil {
		return fmt.Errorf("classification failed: %w", err)
	}

	log.Printf("[webhook] Classification for %s: %s", docPath, reason)

	if !needsUpdate {
		log.Printf("[webhook] Doc %s does not need updating", docPath)
		return nil
	}

	// Generate updated documentation
	updatedDoc, err := h.llmClient.UpdateDocumentation(ctx, codeContext, docContent)
	if err != nil {
		return fmt.Errorf("doc update generation failed: %w", err)
	}

	// Create a PR with the updated docs
	pr, err := ghClient.CreateDocUpdatePR(ctx, baseBranch, docPath, updatedDoc, prNumber)
	if err != nil {
		return fmt.Errorf("failed to create doc update PR: %w", err)
	}

	log.Printf("[webhook] Created doc update PR #%d for %s", pr.GetNumber(), docPath)
	return nil
}

// findCommonDocPaths returns common documentation file paths to check.
// In a full implementation, this would use the GitHub API to discover files.
func findCommonDocPaths(codeFiles []analyzer.ChangedFile) []string {
	paths := []string{
		"README.md",
		"docs/README.md",
	}

	// Add OpenAPI specs
	specFiles := []string{
		"openapi.yaml", "openapi.yml", "openapi.json",
		"swagger.yaml", "swagger.yml", "swagger.json",
	}
	paths = append(paths, specFiles...)

	// TODO: Use GitHub API tree listing to discover actual doc files
	// and apply the full doc-finder heuristic from analyzer package.

	return paths
}
