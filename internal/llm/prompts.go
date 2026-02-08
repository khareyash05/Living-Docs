package llm

import "fmt"

// ClassifierSystemPrompt is the system prompt for determining if code changes require a doc update.
const ClassifierSystemPrompt = `You are a senior software engineer reviewing code changes.
Your job is to determine whether a code change requires updating documentation.

Rules:
- Only answer with "YES" or "NO" followed by a brief one-line reason.
- Cosmetic changes (formatting, renaming private variables, refactoring internals) do NOT require doc updates.
- Changes to public APIs, function signatures, exported types, configuration, or user-facing behavior DO require doc updates.
- If unsure, lean towards "YES".`

// ClassifierUserPrompt builds the user prompt for the classifier.
func ClassifierUserPrompt(codeContent string) string {
	return fmt.Sprintf(`Analyze the following code file and determine if its current state suggests the documentation may need to be updated.

Code file contents:
%s

Does this code contain public APIs, exported functions, types, or user-facing behavior that should be documented? Answer YES or NO with a brief reason.`, "```\n"+codeContent+"\n```")
}

// UpdaterSystemPrompt is the system prompt for the documentation updater.
const UpdaterSystemPrompt = `You are a precise technical documentation writer.

Rules:
- Update the documentation to accurately reflect the code.
- Only change sections that are outdated or missing based on the code.
- Do NOT add speculative features or behaviors not present in the code.
- Do NOT remove documentation sections that are still accurate.
- Preserve the existing formatting style, headings, and structure.
- If code has new exported functions, types, or parameters not in the docs, add them.
- If code has removed or renamed exports that are in the docs, update them.
- Use chain-of-thought: first explain what changed, then output the updated documentation.
- Output ONLY the updated documentation content after your reasoning, wrapped in <updated_doc> tags.`

// UpdaterUserPrompt builds the user prompt for the documentation updater.
func UpdaterUserPrompt(codeContent, docContent string) string {
	return fmt.Sprintf(`Here is the current code:

%s

Here is the current documentation:

%s

Instructions:
1. Compare the code with the documentation.
2. Identify any discrepancies (missing parameters, outdated function signatures, removed features, etc.).
3. Explain what needs to change and why.
4. Output the fully updated documentation wrapped in <updated_doc> and </updated_doc> tags.

If the documentation is already accurate, return it unchanged within the tags.`,
		"```\n"+codeContent+"\n```",
		"```\n"+docContent+"\n```")
}
