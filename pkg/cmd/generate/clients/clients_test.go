package clients

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/algolia/docli/pkg/cmd/generate/utils"
	"go.yaml.in/yaml/v4"
)

func TestGetAPIDataRendersQuotedPlainTextDescription(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/keys/{key}:
    get:
      operationId: getApiKey
      summary: Get an API key
      description: |
        Use the [API key](https://algolia.com) endpoint with **filters:active**.

        This body keeps [markdown](https://algolia.com/doc) and **details** intact.
      x-acl:
        - search
`)

	doc, err := utils.LoadSpec(spec)
	if err != nil {
		t.Fatalf("LoadSpec() error = %v", err)
	}

	data, err := getAPIData(doc, &Options{
		APIName:         "search",
		InputFilename:   "specs/search.yml",
		OutputDirectory: "out",
	})
	if err != nil {
		t.Fatalf("getAPIData() error = %v", err)
	}

	if len(data) != 1 {
		t.Fatalf("getAPIData() len = %d, want 1", len(data))
	}

	if got := data[0].ShortDescription; got != "Use the API key endpoint with filters:active." {
		t.Fatalf(
			"ShortDescription = %q, want %q",
			got,
			"Use the API key endpoint with filters:active.",
		)
	}

	if got := data[0].Description; got != "This body keeps [markdown](https://algolia.com/doc) and **details** intact." {
		t.Fatalf(
			"Description = %q, want %q",
			got,
			"This body keeps [markdown](https://algolia.com/doc) and **details** intact.",
		)
	}

	tmpl := template.Must(template.New("method").Funcs(template.FuncMap{
		"frontmatterString": utils.QuoteFrontmatterString,
		"trim":              strings.TrimSpace,
	}).Parse(methodTemplate))

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	frontmatter := parseFrontmatter(t, rendered.String())
	assertFrontmatterDescription(t, frontmatter, "Use the API key endpoint with filters:active.")

	assertRenderedContains(t, rendered.String(), []string{
		`description: "Use the API key endpoint with filters:active."`,
		"This body keeps [markdown](https://algolia.com/doc) and **details** intact.",
		"**Required ACL:** `search`",
	})
}

func parseFrontmatter(t *testing.T, rendered string) map[string]any {
	t.Helper()

	parts := strings.SplitN(rendered, "---\n", 3)
	if len(parts) < 3 || parts[0] != "" {
		t.Fatalf("rendered output missing frontmatter:\n%s", rendered)
	}

	frontmatter := strings.TrimSuffix(parts[1], "\n")

	var parsed map[string]any

	if err := yaml.Unmarshal([]byte(frontmatter), &parsed); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v\nfrontmatter:\n%s", err, frontmatter)
	}

	return parsed
}

func assertFrontmatterDescription(t *testing.T, frontmatter map[string]any, want string) {
	t.Helper()

	gotDescription, ok := frontmatter["description"].(string)
	if !ok || gotDescription != want {
		t.Fatalf("frontmatter description = %#v, want %q", frontmatter["description"], want)
	}
}

func assertRenderedContains(t *testing.T, rendered string, wants []string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered method missing %q:\n%s", want, rendered)
		}
	}
}
