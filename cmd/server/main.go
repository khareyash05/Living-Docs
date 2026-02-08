package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	ghWebhooks "github.com/go-playground/webhooks/v6/github"
	"github.com/khareyash05/living-docs/internal/config"
	ghClient "github.com/khareyash05/living-docs/internal/github"
	"github.com/khareyash05/living-docs/internal/llm"
)

func main() {
	ctx := context.Background()

	cfg, err := config.MustLoadForServer()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	llmClient, err := llm.NewClient(ctx, cfg.AnthropicAPIKey, cfg.LLMModel)
	if err != nil {
		log.Fatalf("Failed to create LLM client: %v", err)
	}
	handler := ghClient.NewWebhookHandler(llmClient)

	hook, err := ghWebhooks.New(ghWebhooks.Options.Secret(cfg.GitHubWebhookSecret))
	if err != nil {
		log.Fatalf("Failed to create webhook handler: %v", err)
	}

	r := gin.Default()

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "living-docs-agent",
		})
	})

	// GitHub webhook endpoint
	r.POST("/webhook/github", func(c *gin.Context) {
		payload, err := hook.Parse(c.Request, ghWebhooks.PullRequestEvent)
		if err != nil {
			if err == ghWebhooks.ErrEventNotFound {
				// Event not relevant, acknowledge and ignore
				c.JSON(http.StatusOK, gin.H{"message": "event type not handled"})
				return
			}
			log.Printf("Error parsing webhook: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook payload"})
			return
		}

		switch p := payload.(type) {
		case ghWebhooks.PullRequestPayload:
			handlePullRequestEvent(c, p, handler, cfg)
		default:
			c.JSON(http.StatusOK, gin.H{"message": "event type not handled"})
		}
	})

	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("Living Docs Agent server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handlePullRequestEvent(c *gin.Context, payload ghWebhooks.PullRequestPayload, handler *ghClient.WebhookHandler, cfg *config.Config) {
	// Only process merged pull requests
	action := payload.Action
	merged := payload.PullRequest.Merged

	log.Printf("Received PR event: action=%s, merged=%v, repo=%s, pr=#%d",
		action, merged, payload.Repository.FullName, int(payload.PullRequest.Number))

	if action != "closed" || !merged {
		c.JSON(http.StatusOK, gin.H{"message": "PR not merged, skipping"})
		return
	}

	// Extract repo info
	repoFullName := payload.Repository.FullName // "owner/repo"
	prNumber := int(payload.PullRequest.Number)
	baseBranch := payload.PullRequest.Base.Ref

	log.Printf("Processing merged PR #%d in %s (base: %s)", prNumber, repoFullName, baseBranch)

	// Acknowledge the webhook immediately, process asynchronously
	c.JSON(http.StatusAccepted, gin.H{"message": "processing merged PR", "pr": prNumber})

	// Process in background
	go func() {
		ctx := context.Background()

		// TODO: In production, use GitHub App installation tokens instead of a personal token.
		// For now, use GITHUB_TOKEN env var for development/testing.
		githubToken := os.Getenv("GITHUB_TOKEN")
		if githubToken == "" {
			log.Printf("GITHUB_TOKEN not set, cannot process PR #%d", prNumber)
			return
		}

		owner, repo := parseRepoFullName(repoFullName)
		client := ghClient.NewClient(githubToken, owner, repo)

		if err := handler.HandleMergedPR(ctx, client, prNumber, baseBranch); err != nil {
			log.Printf("Error processing PR #%d: %v", prNumber, err)
		} else {
			log.Printf("Successfully processed PR #%d", prNumber)
		}
	}()
}

func parseRepoFullName(fullName string) (owner, repo string) {
	for i, c := range fullName {
		if c == '/' {
			return fullName[:i], fullName[i+1:]
		}
	}
	return fullName, ""
}
