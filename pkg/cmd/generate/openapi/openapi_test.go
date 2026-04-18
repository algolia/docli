package openapi

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/algolia/docli/pkg/cmd/generate/utils"
	"go.yaml.in/yaml/v4"
)

func TestGetAPIDataIncludesOperationDescriptionsInStubTemplate(t *testing.T) {
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
        Retrieve the [API key](https://algolia.com) with **filters:active**.

        Use this endpoint to fetch a key by its value with **sample** output.
      x-acl:
        - search
`)

	doc, err := utils.LoadSpec(spec)
	if err != nil {
		t.Fatalf("LoadSpec() error = %v", err)
	}

	data, err := getAPIData(doc, &Options{
		APIName:         "search",
		InputFileName:   "specs/search.yml",
		OutputDirectory: "out",
	})
	if err != nil {
		t.Fatalf("getAPIData() error = %v", err)
	}

	if len(data) != 1 {
		t.Fatalf("getAPIData() len = %d, want 1", len(data))
	}

	if got := data[0].ShortDescription; got != "Retrieve the API key with filters:active." {
		t.Fatalf("ShortDescription = %q, want %q", got, "Retrieve the API key with filters:active.")
	}

	if got := data[0].Description; got != "Use this endpoint to fetch a key by its value with **sample** output." {
		t.Fatalf(
			"Description = %q, want %q",
			got,
			"Use this endpoint to fetch a key by its value with **sample** output.",
		)
	}

	tmpl := template.Must(template.New("stub").Funcs(template.FuncMap{
		"frontmatterString": utils.QuoteFrontmatterString,
	}).Parse(stubTemplate))

	var got bytes.Buffer
	if err := tmpl.Execute(&got, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	rendered := got.String()

	frontmatter := parseFrontmatter(t, rendered)
	assertFrontmatterTitle(t, frontmatter, "Get an API key")
	assertFrontmatterDescription(t, frontmatter, "Retrieve the API key with filters:active.")

	assertRenderedContains(t, rendered, []string{
		`title: "Get an API key"`,
		`description: "Retrieve the API key with filters:active."`,
		"Use this endpoint to fetch a key by its value with **sample** output.",
		"**Required ACL:** `search`",
	})
}

func TestGetAPIDataRejectsMissingOperationID(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/keys/{key}:
    get:
      summary: Get an API key
      description: Retrieve key.
`)

	doc, err := utils.LoadSpec(spec)
	if err != nil {
		t.Fatalf("LoadSpec() error = %v", err)
	}

	_, err = getAPIData(doc, &Options{
		APIName:         "search",
		InputFileName:   "specs/search.yml",
		OutputDirectory: "out",
	})
	if err == nil {
		t.Fatal("expected missing operationId error")
	}

	assertRenderedContains(t, err.Error(), []string{"missing operationId for GET /1/keys/{key}"})
}

func TestGetAPIOverviewDataSplitsDescriptionWhenSummaryMissing(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: "Search API: [Core]"
  version: 1.0.0
  description: |
    Learn the [Search API](https://algolia.com) endpoints.

    Use **filters** and _examples_ in the full overview.
paths: {}
`)

	doc, err := utils.LoadSpec(spec)
	if err != nil {
		t.Fatalf("LoadSpec() error = %v", err)
	}

	data, err := getAPIOverviewData(doc, &Options{
		APIName:         "search",
		InputFileName:   "specs/search.yml",
		OutputDirectory: "out",
	})
	if err != nil {
		t.Fatalf("getAPIOverviewData() error = %v", err)
	}

	if got := data.ShortDescription; got != "Learn the Search API endpoints." {
		t.Fatalf("ShortDescription = %q, want %q", got, "Learn the Search API endpoints.")
	}

	if got := data.Description; got != "Use **filters** and _examples_ in the full overview." {
		t.Fatalf(
			"Description = %q, want %q",
			got,
			"Use **filters** and _examples_ in the full overview.",
		)
	}

	tmpl := template.Must(template.New("overview").Funcs(template.FuncMap{
		"frontmatterString": utils.QuoteFrontmatterString,
	}).Parse(overviewTemplate))

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	frontmatter := parseFrontmatter(t, rendered.String())
	assertFrontmatterTitle(t, frontmatter, "Search API: [Core]")
	assertFrontmatterDescription(t, frontmatter, "Learn the Search API endpoints.")

	assertRenderedContains(t, rendered.String(), []string{
		`title: "Search API: [Core]"`,
		"Use **filters** and _examples_ in the full overview.",
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

func assertFrontmatterTitle(t *testing.T, frontmatter map[string]any, want string) {
	t.Helper()

	gotTitle, ok := frontmatter["title"].(string)
	if !ok || gotTitle != want {
		t.Fatalf("frontmatter title = %#v, want %q", frontmatter["title"], want)
	}
}

func assertRenderedContains(t *testing.T, rendered string, wants []string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered output missing %q:\n%s", want, rendered)
		}
	}
}
