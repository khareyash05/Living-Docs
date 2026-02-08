package analyzer

import (
	"path/filepath"
	"strings"
)

// ChangedFile represents a file that was modified in a diff.
type ChangedFile struct {
	Filename  string
	Extension string
	Patch     string // The raw diff/patch content
}

// ParseChangedFiles extracts file information from a list of filenames.
// In the CLI context, this takes local file paths.
// In the webhook context, this takes file paths from the GitHub API.
func ParseChangedFiles(filenames []string) []ChangedFile {
	files := make([]ChangedFile, 0, len(filenames))
	for _, f := range filenames {
		files = append(files, ChangedFile{
			Filename:  f,
			Extension: strings.ToLower(filepath.Ext(f)),
		})
	}
	return files
}

// IsCodeFile returns true if the file is a source code file that could affect documentation.
func IsCodeFile(ext string) bool {
	codeExtensions := map[string]bool{
		".go":   true,
		".py":   true,
		".js":   true,
		".ts":   true,
		".java": true,
		".rs":   true,
		".rb":   true,
		".c":    true,
		".cpp":  true,
		".h":    true,
		".hpp":  true,
		".cs":   true,
		".php":  true,
		".swift": true,
		".kt":   true,
	}
	return codeExtensions[ext]
}

// IsDocFile returns true if the file is a documentation file.
func IsDocFile(ext string) bool {
	docExtensions := map[string]bool{
		".md":   true,
		".mdx":  true,
		".rst":  true,
		".txt":  true,
		".yaml": true,
		".yml":  true,
		".json": true,
	}
	return docExtensions[ext]
}

// FilterCodeFiles returns only the code files from a list of changed files.
func FilterCodeFiles(files []ChangedFile) []ChangedFile {
	var result []ChangedFile
	for _, f := range files {
		if IsCodeFile(f.Extension) {
			result = append(result, f)
		}
	}
	return result
}
