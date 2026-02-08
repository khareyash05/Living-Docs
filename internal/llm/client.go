package llm

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// Client wraps the OpenAI API for documentation operations.
type Client struct {
	api   *openai.Client
	model string
}

// NewClient creates a new LLM client with the given API key and model.
func NewClient(apiKey, model string) *Client {
	return &Client{
		api:   openai.NewClient(apiKey),
		model: model,
	}
}

// ClassifyChange asks the LLM whether the given code warrants a documentation update.
// Returns true if the docs should be updated, along with the reasoning.
func (c *Client) ClassifyChange(ctx context.Context, codeContent string) (bool, string, error) {
	resp, err := c.api.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: ClassifierSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: ClassifierUserPrompt(codeContent)},
		},
		Temperature: 0.1,
		MaxTokens:   256,
	})
	if err != nil {
		return false, "", fmt.Errorf("LLM classification request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return false, "", fmt.Errorf("LLM returned no choices")
	}

	answer := resp.Choices[0].Message.Content
	needsUpdate := strings.HasPrefix(strings.TrimSpace(strings.ToUpper(answer)), "YES")

	return needsUpdate, answer, nil
}

// UpdateDocumentation asks the LLM to update the documentation based on the code.
// Returns the updated documentation content.
func (c *Client) UpdateDocumentation(ctx context.Context, codeContent, docContent string) (string, error) {
	resp, err := c.api.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: UpdaterSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: UpdaterUserPrompt(codeContent, docContent)},
		},
		Temperature: 0.2,
		MaxTokens:   4096,
	})
	if err != nil {
		return "", fmt.Errorf("LLM update request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	content := resp.Choices[0].Message.Content
	updated, err := extractUpdatedDoc(content)
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
