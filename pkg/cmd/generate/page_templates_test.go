package generate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPageTemplatesHavePublicFrontmatter(t *testing.T) {
	t.Parallel()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine current test file path")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))

	// Keep this list explicit: these built-in generators render full pages with
	// frontmatter, unlike snippet/include generators or runtime CDN templates.
	pageTemplates := []string{
		"pkg/cmd/generate/clients/method.mdx.tmpl",
		"pkg/cmd/generate/openapi/overview.mdx.tmpl",
		"pkg/cmd/generate/openapi/stub.mdx.tmpl",
		"pkg/cmd/generate/sla/page.mdx.tmpl",
	}

	for _, relativePath := range pageTemplates {
		t.Run(relativePath, func(t *testing.T) {
			t.Parallel()

			contents, err := os.ReadFile(filepath.Join(repoRoot, relativePath))
			if err != nil {
				t.Fatalf("read template %s: %v", relativePath, err)
			}

			frontmatter, ok := extractFrontmatter(string(contents))
			if !ok {
				t.Fatalf("expected %s to start with frontmatter", relativePath)
			}

			if !frontmatterHasLine(frontmatter, "public: true") {
				t.Fatalf("expected %s frontmatter to include `public: true`", relativePath)
			}
		})
	}
}

func extractFrontmatter(contents string) (string, bool) {
	normalized := strings.ReplaceAll(contents, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	if len(lines) == 0 || lines[0] != "---" {
		return "", false
	}

	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return strings.Join(lines[1:i], "\n"), true
		}
	}

	return "", false
}

func frontmatterHasLine(frontmatter string, want string) bool {
	for _, line := range strings.Split(frontmatter, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}

	return false
}
