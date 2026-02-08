package analyzer

import (
	"os"
	"path/filepath"
	"strings"
)

// DocCandidate represents a documentation file that may need updating.
type DocCandidate struct {
	Path     string
	Priority int // Lower is higher priority
	Reason   string
}

// FindLinkedDocs searches for documentation files related to the given code file.
// It walks upward from the code file's directory to the repoRoot, looking for
// README.md, docs/ directories, and OpenAPI spec files.
func FindLinkedDocs(codeFilePath, repoRoot string) []DocCandidate {
	var candidates []DocCandidate
	priority := 0

	codeDir := filepath.Dir(codeFilePath)
	if !filepath.IsAbs(codeDir) {
		codeDir = filepath.Join(repoRoot, codeDir)
	}

	// Walk upward from code file directory to repo root
	currentDir := codeDir
	for {
		// Check for README.md in current directory
		readmePath := filepath.Join(currentDir, "README.md")
		if fileExists(readmePath) {
			relPath, _ := filepath.Rel(repoRoot, readmePath)
			candidates = append(candidates, DocCandidate{
				Path:     relPath,
				Priority: priority,
				Reason:   "README.md found near code file",
			})
		}

		// Check for docs/ directory
		docsDir := filepath.Join(currentDir, "docs")
		if dirExists(docsDir) {
			mdFiles := findMarkdownFiles(docsDir)
			for _, md := range mdFiles {
				relPath, _ := filepath.Rel(repoRoot, md)
				candidates = append(candidates, DocCandidate{
					Path:     relPath,
					Priority: priority + 1,
					Reason:   "Markdown file in docs/ directory",
				})
			}
		}

		priority++

		// Check if we've reached the repo root
		absCurrentDir, _ := filepath.Abs(currentDir)
		absRepoRoot, _ := filepath.Abs(repoRoot)
		if absCurrentDir == absRepoRoot {
			break
		}

		// Move up one directory
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			// Reached filesystem root without finding repo root
			break
		}
		currentDir = parent
	}

	// Check for OpenAPI/Swagger specs at repo root
	specFiles := []string{
		"openapi.yaml", "openapi.yml", "openapi.json",
		"swagger.yaml", "swagger.yml", "swagger.json",
	}
	for _, spec := range specFiles {
		specPath := filepath.Join(repoRoot, spec)
		if fileExists(specPath) {
			candidates = append(candidates, DocCandidate{
				Path:     spec,
				Priority: priority,
				Reason:   "OpenAPI/Swagger spec at repo root",
			})
		}
	}

	return candidates
}

// FindLinkedDocsForFiles finds documentation candidates for multiple code files and deduplicates.
func FindLinkedDocsForFiles(codeFiles []ChangedFile, repoRoot string) []DocCandidate {
	seen := make(map[string]bool)
	var allCandidates []DocCandidate

	for _, cf := range codeFiles {
		candidates := FindLinkedDocs(cf.Filename, repoRoot)
		for _, c := range candidates {
			if !seen[c.Path] {
				seen[c.Path] = true
				allCandidates = append(allCandidates, c)
			}
		}
	}

	return allCandidates
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func findMarkdownFiles(dir string) []string {
	var files []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".md" || ext == ".mdx" {
				files = append(files, path)
			}
		}
		return nil
	})
	return files
}
