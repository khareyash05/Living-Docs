# Living Docs Agent - Webhook Server Guide

## Architecture Overview

```
┌─────────────────┐
│   GitHub Repo   │
│                 │
│  PR #42 Merged  │
└────────┬────────┘
         │
         │ Webhook Event (POST)
         │
         ▼
┌─────────────────────────────────┐
│  Living Docs Server              │
│  (Port 8080)                     │
│                                  │
│  POST /webhook/github            │
│  ├─ Validates webhook signature  │
│  ├─ Filters: action=closed       │
│  └─ Filters: merged=true         │
└────────┬─────────────────────────┘
         │
         │ Async Processing (goroutine)
         │
         ▼
┌─────────────────────────────────┐
│  WebhookHandler                 │
│                                  │
│  1. Fetch PR diff               │
│  2. Identify code files         │
│  3. Find linked docs            │
│  4. For each doc:               │
│     ├─ Classify (LLM)           │
│     ├─ Generate update (LLM)    │
│     └─ Create PR                │
└────────┬─────────────────────────┘
         │
         ├─────────────────┬─────────────────┐
         ▼                 ▼                 ▼
    ┌─────────┐      ┌──────────┐      ┌──────────┐
    │ GitHub  │      │ Anthropic│      │ GitHub   │
    │   API   │      │   API    │      │   API    │
    │         │      │          │      │          │
    │ Get diff│      │ Classify │      │ Create   │
    │ Get file│      │ Generate │      │ Branch   │
    └─────────┘      └──────────┘      │ Create PR│
                                      └──────────┘
```

## End-to-End Flow

### Step 1: Setup & Configuration

1. **Set Environment Variables**
   ```bash
   export ANTHROPIC_API_KEY="sk-ant-..."
   export GITHUB_WEBHOOK_SECRET="your-webhook-secret"
   export GITHUB_TOKEN="ghp_..."  # For GitHub API access
   export SERVER_PORT="8080"
   ```

2. **Start the Server**
   ```bash
   go run ./cmd/server
   # Or build and run:
   go build -o docs-server ./cmd/server
   ./docs-server
   ```

3. **Verify Server is Running**
   ```bash
   curl http://localhost:8080/health
   # Response: {"status":"ok","service":"living-docs-agent"}
   ```

### Step 2: Configure GitHub Webhook

1. **Go to your GitHub repository**
   - Settings → Webhooks → Add webhook

2. **Configure Webhook**
   - **Payload URL**: `https://your-server.com/webhook/github`
   - **Content type**: `application/json`
   - **Secret**: Same as `GITHUB_WEBHOOK_SECRET` env var
   - **Events**: Select "Pull requests" only
   - **Active**: ✓

3. **Test the Webhook**
   - GitHub will send a test ping
   - Check server logs to verify receipt

### Step 3: Trigger Event (Developer Workflow)

1. **Developer makes code changes**
   ```bash
   # Example: Add a new function parameter
   git checkout -b feature/add-limit-param
   # Edit handler.go: func GetUsers(limit int)
   git commit -m "Add limit parameter to GetUsers"
   git push origin feature/add-limit-param
   ```

2. **Developer opens Pull Request**
   - PR #42: "Add limit parameter to GetUsers"
   - Code changes: `handler.go` (added `limit int` parameter)

3. **PR gets merged**
   - Maintainer reviews and merges PR #42
   - GitHub sends webhook event to your server

### Step 4: Server Processing (Automatic)

#### 4.1 Webhook Reception
```
POST /webhook/github
Headers:
  X-GitHub-Event: pull_request
  X-Hub-Signature-256: sha256=...
Body: {
  "action": "closed",
  "pull_request": {
    "number": 42,
    "merged": true,
    "base": { "ref": "main" }
  },
  "repository": {
    "full_name": "owner/repo"
  }
}
```

**Server Actions:**
- ✅ Validates webhook signature using `GITHUB_WEBHOOK_SECRET`
- ✅ Parses payload as `PullRequestPayload`
- ✅ Checks: `action == "closed"` AND `merged == true`
- ✅ Returns `202 Accepted` immediately (async processing)

#### 4.2 Background Processing (Goroutine)

**Step 1: Fetch PR Diff**
```go
diff := ghClient.GetPRDiff(ctx, 42)
// Returns:
// {
//   Files: [
//     {
//       Filename: "handler.go",
//       Patch: "@@ -10,6 +10,6 @@\n func GetUsers() {\n+func GetUsers(limit int) {\n",
//       Status: "modified"
//     }
//   ]
// }
```

**Step 2: Identify Code Files**
```go
codeFiles := analyzer.FilterCodeFiles(changedFiles)
// Filters to: [handler.go] (only .go files)
```

**Step 3: Find Linked Documentation**
```go
docPaths := findCommonDocPaths(codeFiles)
// Returns: ["README.md", "docs/README.md", "openapi.yaml", ...]
```

**Step 4: Process Each Doc File**

For each doc file (e.g., `README.md`):

**4a. Fetch Current Doc Content**
```go
docContent := ghClient.GetFileContent(ctx, "README.md", "main")
// Returns: "# API\n\n## GetUsers\n\nGet all users.\n"
```

**4b. Build Code Context**
```go
codeContext := "--- File: handler.go ---\n@@ -10,6 +10,6 @@\n func GetUsers() {\n+func GetUsers(limit int) {\n\n\n"
```

**4c. Classify (LLM Call #1)**
```go
needsUpdate, reason := llmClient.ClassifyChange(ctx, codeContext)
// LLM Response: "YES - Function signature changed, added limit parameter"
// needsUpdate = true
```

**4d. Generate Updated Documentation (LLM Call #2)**
```go
updatedDoc := llmClient.UpdateDocumentation(ctx, codeContext, docContent)
// LLM Response:
// <updated_doc>
// # API
//
// ## GetUsers
//
// Get users with optional limit.
//
// Parameters:
// - limit (int): Maximum number of users to return
// </updated_doc>
```

**4e. Create Pull Request**
```go
// 1. Create branch: docs/update-readme-md-pr-42
// 2. Update README.md with new content
// 3. Commit: "docs: update README.md based on PR #42"
// 4. Open PR: "docs: update README.md (triggered by PR #42)"
```

### Step 5: Result

**New Pull Request Created:**
- **Title**: `docs: update README.md (triggered by PR #42)`
- **Branch**: `docs/update-readme-md-pr-42`
- **Description**: 
  ```
  This PR was automatically generated by the Living Docs agent.
  
  It updates `README.md` to reflect changes made in PR #42.
  
  Please review the changes carefully before merging.
  ```
- **Changes**: Updated `README.md` with new function signature

**Developer/Maintainer:**
- Reviews the doc update PR
- Merges if changes look correct
- Documentation stays in sync! ✅

## Code Flow Details

### Entry Point: `cmd/server/main.go`

```go
main()
├─ Load config (ANTHROPIC_API_KEY, GITHUB_WEBHOOK_SECRET)
├─ Create LLM client
├─ Create WebhookHandler
├─ Setup Gin router
│  ├─ GET /health (health check)
│  └─ POST /webhook/github (webhook endpoint)
└─ Start server on :8080
```

### Webhook Handler: `internal/github/webhook.go`

```go
HandleMergedPR()
├─ GetPRDiff() → Fetch changed files
├─ FilterCodeFiles() → Only .go, .py, etc.
├─ findCommonDocPaths() → ["README.md", ...]
└─ For each doc:
   └─ processDocFile()
      ├─ GetFileContent() → Current doc
      ├─ ClassifyChange() → LLM: "Does this need update?"
      ├─ UpdateDocumentation() → LLM: "Generate updated doc"
      └─ CreateDocUpdatePR() → Create branch + commit + PR
```

### GitHub Client: `internal/github/client.go`

```go
GetPRDiff()
└─ GitHub API: GET /repos/{owner}/{repo}/pulls/{pr}/files

GetFileContent()
└─ GitHub API: GET /repos/{owner}/{repo}/contents/{path}

CreateDocUpdatePR()
├─ Git API: POST /repos/{owner}/{repo}/git/refs (create branch)
├─ Contents API: PUT /repos/{owner}/{repo}/contents/{path} (update file)
└─ PR API: POST /repos/{owner}/{repo}/pulls (create PR)
```

## Testing Locally

### 1. Use ngrok to expose local server

```bash
# Terminal 1: Start server
go run ./cmd/server

# Terminal 2: Expose to internet
ngrok http 8080
# Copy the HTTPS URL: https://abc123.ngrok.io
```

### 2. Configure GitHub webhook with ngrok URL

- Payload URL: `https://abc123.ngrok.io/webhook/github`
- Secret: Your `GITHUB_WEBHOOK_SECRET`

### 3. Test with a real PR

- Create a test PR in your repo
- Merge it
- Watch server logs for processing
- Check for new PR created by the bot

## Environment Variables Reference

| Variable | Required | Description |
|----------|----------|-------------|
| `ANTHROPIC_API_KEY` | Yes | Anthropic Claude API key |
| `GITHUB_WEBHOOK_SECRET` | Yes | Secret for webhook signature validation |
| `GITHUB_TOKEN` | Yes | GitHub personal access token (for API calls) |
| `LLM_MODEL` | No | Model to use (default: `claude-sonnet-4-5`) |
| `SERVER_PORT` | No | Port to listen on (default: `8080`) |

## Troubleshooting

### Webhook not received
- ✅ Check server is running and accessible
- ✅ Verify webhook URL is correct
- ✅ Check GitHub webhook delivery logs (Settings → Webhooks → Recent Deliveries)
- ✅ Verify webhook secret matches

### PR not created
- ✅ Check `GITHUB_TOKEN` is set and has correct permissions
- ✅ Verify token has `repo` scope
- ✅ Check server logs for errors
- ✅ Ensure PR was actually merged (not just closed)

### LLM errors
- ✅ Verify `ANTHROPIC_API_KEY` is valid
- ✅ Check API quota/rate limits
- ✅ Verify model name is correct

## Security Considerations

1. **Webhook Secret**: Always use a strong secret and validate signatures
2. **GitHub Token**: Use fine-grained tokens with minimal permissions
3. **Rate Limiting**: Consider adding rate limiting to prevent abuse
4. **Error Handling**: Don't expose sensitive info in error messages
5. **Production**: Use GitHub App installation tokens instead of personal tokens

## Future Enhancements

- [ ] Use GitHub App instead of personal tokens
- [ ] Implement full doc-finder heuristic (walk repo tree)
- [ ] Support multiple documentation formats (OpenAPI, etc.)
- [ ] Add retry logic for failed API calls
- [ ] Add metrics/monitoring
- [ ] Support for private repositories
