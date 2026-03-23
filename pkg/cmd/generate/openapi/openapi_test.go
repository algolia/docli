package openapi

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/algolia/docli/pkg/cmd/generate/utils"
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
        Retrieve an API key.

        Use this endpoint to fetch a key by its value.
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

	if got := data[0].ShortDescription; got != "Retrieve an API key." {
		t.Fatalf("ShortDescription = %q, want %q", got, "Retrieve an API key.")
	}

	if got := data[0].Description; got != "Use this endpoint to fetch a key by its value." {
		t.Fatalf("Description = %q, want %q", got, "Use this endpoint to fetch a key by its value.")
	}

	tmpl := template.Must(template.New("stub").Parse(stubTemplate))

	var got bytes.Buffer
	if err := tmpl.Execute(&got, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	rendered := got.String()

	for _, want := range []string{
		"title: Get an API key",
		"description: Retrieve an API key.",
		"Use this endpoint to fetch a key by its value.",
		"**Required ACL:** `search`",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered stub missing %q:\n%s", want, rendered)
		}
	}
}
