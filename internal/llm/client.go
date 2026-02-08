package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
)

// Client wraps the Anthropic Claude API for documentation operations.
type Client struct {
	api   anthropic.Client
	model anthropic.Model
}

// NewClient creates a new LLM client with the given API key and model.
func NewClient(ctx context.Context, apiKey, model string) (*Client, error) {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	return &Client{
		api:   client,
		model: anthropic.Model(model),
	}, nil
}

// ClassifyChange asks the LLM whether the given code warrants a documentation update.
// Returns true if the docs should be updated, along with the reasoning.
func (c *Client) ClassifyChange(ctx context.Context, codeContent string) (bool, string, error) {
	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 256,
		System: []anthropic.TextBlockParam{
			{Text: ClassifierSystemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(ClassifierUserPrompt(codeContent))),
		},
		Temperature: param.Opt[float64]{Value: 0.1},
	}

	resp, err := c.api.Messages.New(ctx, params)
	if err != nil {
		return false, "", fmt.Errorf("LLM classification request failed: %w", err)
	}

	if len(resp.Content) == 0 {
		return false, "", fmt.Errorf("LLM returned no content")
	}

	// Extract text from the first content block
	var answer string
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			answer = block.Text
			break
		}
	}

	if answer == "" {
		return false, "", fmt.Errorf("LLM response did not contain text content")
	}

	needsUpdate := strings.HasPrefix(strings.TrimSpace(strings.ToUpper(answer)), "YES")

	return needsUpdate, answer, nil
}

// UpdateDocumentation asks the LLM to update the documentation based on the code.
// Returns the updated documentation content.
func (c *Client) UpdateDocumentation(ctx context.Context, codeContent, docContent string) (string, error) {
	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 4096,
		System: []anthropic.TextBlockParam{
			{Text: UpdaterSystemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(UpdaterUserPrompt(codeContent, docContent))),
		},
		Temperature: param.Opt[float64]{Value: 0.2},
	}

	resp, err := c.api.Messages.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("LLM update request failed: %w", err)
	}

	if len(resp.Content) == 0 {
		return "", fmt.Errorf("LLM returned no content")
	}

	// Extract text from all content blocks
	var content strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			content.WriteString(block.Text)
		}
	}

	updated, err := extractUpdatedDoc(content.String())
	if err != nil {
		return "", err
	}

	return updated, nil
}

// extractUpdatedDoc parses the <updated_doc> tags from the LLM response.
func extractUpdatedDoc(content string) (string, error) {
	const openTag = "<updated_doc>"
	const closeTag = "</updated_doc>"

	startIdx := strings.Index(content, openTag)
	if startIdx == -1 {
		// If no tags found, assume the entire response is the updated doc
		return strings.TrimSpace(content), nil
	}

	startIdx += len(openTag)
	endIdx := strings.Index(content[startIdx:], closeTag)
	if endIdx == -1 {
		// Opening tag found but no closing tag; take everything after open tag
		return strings.TrimSpace(content[startIdx:]), nil
	}

	return strings.TrimSpace(content[startIdx : startIdx+endIdx]), nil
}
