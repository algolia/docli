package clients

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/algolia/docli/pkg/cmd/generate/utils"
	"go.yaml.in/yaml/v4"
)

func buildMethodTemplate(t *testing.T) *template.Template {
	t.Helper()

	return template.Must(template.New("method").Funcs(template.FuncMap{
		"frontmatterString": utils.QuoteFrontmatterString,
		"mintFieldType":     mintFieldType,
		"renderParamFields": renderParamFields,
		"renderResponses":   renderResponses,
		"trim":              strings.TrimSpace,
	}).Parse(methodTemplate))
}

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

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	frontmatter := parseFrontmatter(t, rendered.String())
	assertFrontmatterDescription(t, frontmatter, "Use the API key endpoint with filters:active.")
	assertFrontmatterOperationIDs(t, frontmatter, []string{"getApiKey", "GetApiKey", "get_api_key"})

	assertRenderedContains(t, rendered.String(), []string{
		`description: "Use the API key endpoint with filters:active."`,
		"operationId:\n  - getApiKey\n  - GetApiKey\n  - get_api_key",
		"This body keeps [markdown](https://algolia.com/doc) and **details** intact.",
		"**Required ACL:** `search`",
		"For details about the underlying HTTP method.",
	})
	assertRenderedNotContains(t, rendered.String(), []string{
		"For HTTP endpoint details and response fields.",
	})
}

func TestGetAPIDataMergesTopLevelSDKInputs(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/indexes/{indexName}/query:
    post:
      operationId: searchSingleIndex
      summary: Search an index
      description: Search index.
      parameters:
        - name: indexName
          in: path
          required: true
          description: Index to search.
          schema:
            type: string
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                filters:
                  type: string
                  description: Filter expression.
                optionalFilters:
                  $ref: '#/components/schemas/optionalFilters'
                hitsPerPage:
                  type: integer
                  description: Maximum hits per page.
              required:
                - optionalFilters
components:
  schemas:
    optionalFilters:
      oneOf:
        - type: array
          items:
            $ref: '#/components/schemas/optionalFilters'
        - type: string
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

	paramsMap := map[string]Parameter{}
	for _, param := range data[0].Params {
		paramsMap[param.Name] = param
	}

	assertParameter(t, paramsMap, Parameter{
		Name:        "indexName",
		Description: "Index to search.",
		Required:    true,
		Type:        "string",
	})

	assertParameter(t, paramsMap, Parameter{
		Name:        "filters",
		Description: "Filter expression.",
		Required:    false,
		Type:        "string",
	})

	assertParameter(t, paramsMap, Parameter{
		Name:        "optionalFilters",
		Description: "",
		Required:    true,
		Type:        "array<optionalFilters> (recursive) | string",
	})

	if got := len(paramsMap["optionalFilters"].Variants); got != 0 {
		t.Fatalf("optionalFilters variants len = %d, want 0", got)
	}

	assertParameter(t, paramsMap, Parameter{
		Name:        "hitsPerPage",
		Description: "Maximum hits per page.",
		Required:    false,
		Type:        "integer",
	})

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	frontmatter := parseFrontmatter(t, rendered.String())
	assertFrontmatterOperationIDs(t, frontmatter, []string{
		"searchSingleIndex",
		"SearchSingleIndex",
		"search_single_index",
	})

	assertRenderedContains(t, rendered.String(), []string{
		"## Parameters",
		"The tabs in the [Usage](#usage) section show API client-specific call shapes.",
		"The parameters below describe the shared request fields that you pass as arguments in most API clients.",
		"Parameter names and syntax may vary by language.",
		"For example, some clients use `snake_case`.",
		"Others use different patterns, such as builder methods.",
		"operationId:\n  - searchSingleIndex\n  - SearchSingleIndex\n  - search_single_index",
		"<ParamField path=\"indexName\" type=\"string\" required>",
		"Index to search.",
		"<ParamField path=\"optionalFilters\" type=\"array&lt;optionalFilters&gt; (recursive) | string\" required>",
		"<ParamField path=\"hitsPerPage\" type=\"number\">",
		"Maximum hits per page.",
	})
	assertRenderedNotContains(t, rendered.String(), []string{
		"<Tab title=\"Nested list\">",
		"<Tab title=\"String\">",
	})
}

func TestGetAPIDataKeepsBodyAndPathFieldWhenNamesCollide(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/indexes/{indexName}/synonyms/{objectID}:
    put:
      operationId: saveSynonym
      summary: Save synonym
      description: Save synonym.
      parameters:
        - name: indexName
          in: path
          required: true
          description: Index to update.
          schema:
            type: string
        - name: objectID
          in: path
          required: true
          description: Path object ID.
          schema:
            type: string
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/synonymHit'
components:
  schemas:
    synonymHit:
      type: object
      properties:
        objectID:
          type: string
          description: Body object ID.
        synonymType:
          type: string
          description: Synonym type.
      required:
        - objectID
        - synonymType
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

	assertParameter(t, paramsByNameAndIn(data[0].Params), Parameter{
		Name:        "indexName",
		In:          "path",
		Description: "Index to update.",
		Required:    true,
		Type:        "string",
	})

	assertParameter(t, paramsByNameAndIn(data[0].Params), Parameter{
		Name:        "objectID",
		In:          "path",
		Description: "Path object ID.",
		Required:    true,
		Type:        "string",
	})

	assertParameter(t, paramsByNameAndIn(data[0].Params), Parameter{
		Name:        "synonymHit",
		In:          "body",
		Description: "",
		Required:    true,
		Type:        "object",
	})

	body := requireParameter(t, paramsByNameAndIn(data[0].Params), "synonymHit", "body")
	assertParameter(t, paramsByName(body.Children), Parameter{
		Name:        "objectID",
		Description: "Body object ID.",
		Required:    true,
		Type:        "string",
	})

	if len(data[0].Params) != 3 {
		t.Fatalf("parameter count = %d, want 3", len(data[0].Params))
	}
}

func TestGetAPIDataKeepsQueryAndHeaderParamsWhenNamesCollide(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/colliding:
    get:
      operationId: getColliding
      summary: Get colliding
      description: Get colliding.
      parameters:
        - name: id
          in: query
          required: true
          description: Query ID.
          schema:
            type: string
        - name: id
          in: header
          required: true
          description: Header ID.
          schema:
            type: integer
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

	params := paramsByNameAndIn(data[0].Params)
	assertParameter(t, params, Parameter{
		Name:        "id",
		In:          "query",
		Description: "Query ID.",
		Required:    true,
		Type:        "string",
	})
	assertParameter(t, params, Parameter{
		Name:        "id",
		In:          "header",
		Description: "Header ID.",
		Required:    true,
		Type:        "integer",
	})

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"Query ID.",
		"Header ID.",
	})

	if got := strings.Count(rendered.String(), `<ParamField path="id"`); got != 2 {
		t.Fatalf("rendered id param count = %d, want 2\n%s", got, rendered.String())
	}
}

func TestGetAPIDataPrefersOperationParametersOverPathItemParameters(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/override:
    parameters:
      - name: hitsPerPage
        in: query
        required: false
        description: Path page size.
        schema:
          type: string
    get:
      operationId: getOverride
      summary: Get override
      description: Get override.
      parameters:
        - name: hitsPerPage
          in: query
          required: true
          description: Operation page size.
          schema:
            type: integer
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

	params := paramsByNameAndIn(data[0].Params)
	assertParameter(t, params, Parameter{
		Name:        "hitsPerPage",
		In:          "query",
		Description: "Operation page size.",
		Required:    true,
		Type:        "integer",
	})

	if got := len(data[0].Params); got != 1 {
		t.Fatalf("parameter count = %d, want 1", got)
	}

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"Operation page size.",
		`<ParamField path="hitsPerPage" type="number" required>`,
	})
	assertRenderedNotContains(t, rendered.String(), []string{
		"Path page size.",
	})
}

func TestGetAPIDataRendersNestedObjectParameters(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/test:
    post:
      operationId: createThing
      summary: Create thing
      description: Create thing.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                user:
                  type: object
                  description: User to create.
                  properties:
                    id:
                      type: string
                      description: User identifier.
                    profile:
                      type: object
                      description: Profile details.
                      properties:
                        displayName:
                          type: string
                          description: Public display name.
                      required:
                        - displayName
                  required:
                    - id
                enabled:
                  type: boolean
                  description: Whether creation is enabled.
                emptyLeaf:
                  type: string
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

	assertNestedUserParameters(t, data[0].Params)

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<ParamField path=\"emptyLeaf\" type=\"string\">",
		"<ParamField path=\"user\" type=\"object\">",
		"<Expandable title=\"properties\">",
		"<ParamField path=\"id\" type=\"string\" required>",
		"User identifier.",
		"<ParamField path=\"profile\" type=\"object\">",
		"<ParamField path=\"displayName\" type=\"string\" required>",
		"Public display name.",
	})

	assertRenderedNotContains(t, rendered.String(), []string{
		"\n  <Expandable",
		"\n    <ParamField",
	})

	assertRenderedContains(t, rendered.String(), []string{
		"<ParamField path=\"id\" type=\"string\" required>\n\nUser identifier.\n\n</ParamField>",
		"<Expandable title=\"properties\">\n\n<ParamField path=\"id\" type=\"string\" required>",
		"</ParamField>\n\n<ParamField path=\"profile\" type=\"object\">",
	})
}

func TestGetAPIDataRendersVariantTabs(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/variant:
    post:
      operationId: createVariant
      summary: Create variant
      description: Create variant.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                payload:
                  oneOf:
                    - type: string
                      description: String payload.
                    - type: object
                      description: Object payload.
                      properties:
                        query:
                          type: string
                          description: Query to send.
                      required:
                        - query
              required:
                - payload
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

	payload := requireParameter(t, paramsByName(data[0].Params), "payload")
	if got := len(payload.Variants); got != 2 {
		t.Fatalf("payload variants len = %d, want 2", got)
	}

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<ParamField path=\"payload\" type=\"object | string\" required>",
		"<Tabs sync={false}>",
		"<Tab title=\"String\">",
		"String payload.",
		"Type: `string`",
		"<Tab title=\"Object\">",
		"Object payload.",
		"<ParamField path=\"query\" type=\"string\" required>",
		"Query to send.",
		"</Tab>",
		"</Tabs>",
	})
}

func TestGetAPIDataMergesAllOfObjectChildren(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/allof:
    post:
      operationId: createAllOf
      summary: Create allOf
      description: Create allOf.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                payload:
                  oneOf:
                    - type: string
                    - $ref: '#/components/schemas/payloadObject'
              required:
                - payload
components:
  schemas:
    payloadObject:
      title: payloadObject
      description: Object payload.
      allOf:
        - $ref: '#/components/schemas/basePayload'
        - type: object
          properties:
            page:
              type: integer
              description: Page number.
          required:
            - page
    basePayload:
      allOf:
        - type: object
          properties:
            query:
              type: string
              description: Query text.
          required:
            - query
        - type: object
          properties:
            filters:
              type: string
              description: Filter text.
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

	payload := requireParameter(t, paramsByName(data[0].Params), "payload")
	if got := len(payload.Variants); got != 2 {
		t.Fatalf("payload variants len = %d, want 2", got)
	}

	var objectVariant ParameterVariant

	found := false

	for _, variant := range payload.Variants {
		if variant.Title == "Object" {
			objectVariant = variant
			found = true

			break
		}
	}

	if !found {
		t.Fatalf("Object variant missing in %#v", payload.Variants)
	}

	variantChildren := paramsByName(objectVariant.Children)
	assertParameter(t, variantChildren, Parameter{
		Name:        "query",
		Description: "Query text.",
		Required:    true,
		Type:        "string",
	})
	assertParameter(t, variantChildren, Parameter{
		Name:        "filters",
		Description: "Filter text.",
		Required:    false,
		Type:        "string",
	})
	assertParameter(t, variantChildren, Parameter{
		Name:        "page",
		Description: "Page number.",
		Required:    true,
		Type:        "integer",
	})

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<Tab title=\"Object\">",
		"Object payload.",
		"<ParamField path=\"query\" type=\"string\" required>",
		"Query text.",
		"<ParamField path=\"filters\" type=\"string\">",
		"Filter text.",
		"<ParamField path=\"page\" type=\"number\" required>",
		"Page number.",
	})
}

func TestGetAPIDataFlattensTopLevelAllOfRequestBody(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/top-level-allof:
    post:
      operationId: createTopLevelAllOf
      summary: Create top level allOf
      description: Create top level allOf.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              allOf:
                - $ref: '#/components/schemas/basePayload'
                - type: object
                  properties:
                    page:
                      type: integer
                      description: Page number.
                  required:
                    - page
components:
  schemas:
    basePayload:
      allOf:
        - type: object
          properties:
            query:
              type: string
              description: Query text.
          required:
            - query
        - type: object
          properties:
            filters:
              type: string
              description: Filter text.
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

	params := paramsByName(data[0].Params)
	assertParameter(t, params, Parameter{
		Name:        "query",
		Description: "Query text.",
		Required:    true,
		Type:        "string",
	})
	assertParameter(t, params, Parameter{
		Name:        "filters",
		Description: "Filter text.",
		Required:    false,
		Type:        "string",
	})
	assertParameter(t, params, Parameter{
		Name:        "page",
		Description: "Page number.",
		Required:    true,
		Type:        "integer",
	})

	if got := len(data[0].Params); got != 3 {
		t.Fatalf("params len = %d, want 3", got)
	}

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"## Parameters",
		"<ParamField path=\"query\" type=\"string\" required>",
		"<ParamField path=\"filters\" type=\"string\">",
		"<ParamField path=\"page\" type=\"number\" required>",
	})
	assertRenderedNotContains(t, rendered.String(), []string{
		"<ParamField path=\"requestBody\"",
	})
}

func TestGetAPIDataKeepsExplicitlyNamedRequestBodyWrapper(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/search:
    post:
      operationId: search
      summary: Search multiple indices
      description: Search multiple indices.
      requestBody:
        required: true
        description: Multi-search request body.
        content:
          application/json:
            schema:
              title: searchMethodParams
              type: object
              properties:
                requests:
                  type: array
                  description: Requests to run.
                  items:
                    type: object
                    properties:
                      query:
                        type: string
                        description: Search query.
                strategy:
                  type: string
                  description: Search strategy.
              required:
                - requests
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

	if got := len(data[0].Params); got != 1 {
		t.Fatalf("params len = %d, want 1", got)
	}

	params := paramsByName(data[0].Params)
	body := requireParameter(t, params, "searchMethodParams")
	assertParameter(t, params, Parameter{
		Name:        "searchMethodParams",
		Description: "Multi-search request body.",
		Required:    true,
		Type:        "object",
	})

	children := paramsByName(body.Children)
	requests := requireParameter(t, children, "requests")
	assertParameter(t, children, Parameter{
		Name:        "requests",
		Description: "Requests to run.",
		Required:    true,
		Type:        "array<object>",
	})
	assertParameter(t, children, Parameter{
		Name:        "strategy",
		Description: "Search strategy.",
		Required:    false,
		Type:        "string",
	})
	assertParameter(t, paramsByName(requests.Children), Parameter{
		Name:        "query",
		Description: "Search query.",
		Required:    false,
		Type:        "string",
	})

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<ParamField path=\"searchMethodParams\" type=\"object\" required>",
		"<ParamField path=\"requests\" type=\"object[]\" required>",
		"<ParamField path=\"query\" type=\"string\">",
		"<ParamField path=\"strategy\" type=\"string\">",
		"<Expandable title=\"properties\">",
	})
	assertRenderedNotContains(t, rendered.String(), []string{
		"<ParamField path=\"requestBody\"",
	})
}

func TestGetAPIDataKeepsRefNamedRequestBodyWrapper(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/indexes/{indexName}/settings:
    put:
      operationId: setSettings
      summary: Update index settings
      description: Update index settings.
      parameters:
        - name: indexName
          in: path
          required: true
          description: Index to update.
          schema:
            type: string
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/indexSettings'
components:
  schemas:
    indexSettings:
      description: Index settings.
      allOf:
        - type: object
          properties:
            paginationLimitedTo:
              type: integer
              description: Max pagination limit.
        - type: object
          properties:
            typoTolerance:
              type: string
              description: Typo tolerance mode.
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

	params := paramsByName(data[0].Params)
	assertParameter(t, paramsByNameAndIn(data[0].Params), Parameter{
		Name:        "indexName",
		In:          "path",
		Description: "Index to update.",
		Required:    true,
		Type:        "string",
	})
	assertParameter(t, params, Parameter{
		Name:        "indexSettings",
		Description: "",
		Required:    true,
		Type:        "object",
	})

	body := requireParameter(t, params, "indexSettings")
	children := paramsByName(body.Children)
	assertParameter(t, children, Parameter{
		Name:        "paginationLimitedTo",
		Description: "Max pagination limit.",
		Required:    false,
		Type:        "integer",
	})
	assertParameter(t, children, Parameter{
		Name:        "typoTolerance",
		Description: "Typo tolerance mode.",
		Required:    false,
		Type:        "string",
	})

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<ParamField path=\"indexName\" type=\"string\" required>",
		"<ParamField path=\"indexSettings\" type=\"object\" required>",
		"<ParamField path=\"paginationLimitedTo\" type=\"number\">",
		"<ParamField path=\"typoTolerance\" type=\"string\">",
	})
	assertRenderedNotContains(t, rendered.String(), []string{
		"<ParamField path=\"requestBody\"",
	})
}

func TestGetAPIDataKeepsVariantsAndAllowedValuesWhenAllOfMergesDuplicateFields(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/merged-allof:
    post:
      operationId: createMergedAllOf
      summary: Create merged allOf
      description: Create merged allOf.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              allOf:
                - type: object
                  properties:
                    payload:
                      oneOf:
                        - type: string
                          description: String payload.
                        - type: object
                          description: Object payload.
                          properties:
                            query:
                              type: string
                              description: Query text.
                          required:
                            - query
                    status:
                      type: string
                      enum:
                        - one
                        - two
                        - three
                        - four
                        - five
                        - six
                        - seven
                        - eight
                        - nine
                - type: object
                  properties:
                    payload:
                      description: Payload to send.
                      oneOf:
                        - type: string
                          description: String payload.
                        - type: object
                          description: Object payload.
                          properties:
                            query:
                              type: string
                              description: Query text.
                          required:
                            - query
                    status:
                      description: Status value.
                  required:
                    - payload
                    - status
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

	payload := requireParameter(t, paramsByName(data[0].Params), "payload")
	if got := len(payload.Variants); got != 2 {
		t.Fatalf("payload variants len = %d, want 2", got)
	}

	if got, want := payload.Description, "Payload to send."; got != want {
		t.Fatalf("payload description = %q, want %q", got, want)
	}

	status := requireParameter(t, paramsByName(data[0].Params), "status")
	if got, want := status.Description, "Status value."; got != want {
		t.Fatalf("status description = %q, want %q", got, want)
	}

	if got := len(status.AllowedValues); got != 9 {
		t.Fatalf("status allowed values len = %d, want 9", got)
	}

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"Payload to send.",
		"<Tab title=\"String\">",
		"<Tab title=\"Object\">",
		"Status value.",
		"<Expandable title=\"Allowed values\">",
		"- `'nine'`",
	})
}

func TestGetAPIDataMergesDuplicateVariantChildrenAcrossAllOf(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/variant-merge:
    post:
      operationId: createVariantMerge
      summary: Create variant merge
      description: Create variant merge.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              allOf:
                - type: object
                  properties:
                    payload:
                      oneOf:
                        - type: object
                          description: Object payload.
                          properties:
                            query:
                              type: string
                              description: Query text.
                          required:
                            - query
                - type: object
                  properties:
                    payload:
                      oneOf:
                        - type: object
                          description: Object payload.
                          properties:
                            filters:
                              type: string
                              description: Filter text.
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

	payload := requireParameter(t, paramsByName(data[0].Params), "payload")
	if got := len(payload.Variants); got != 1 {
		t.Fatalf("payload variants len = %d, want 1", got)
	}

	objectVariant := requireParameterVariant(t, payload.Variants, "Object")
	children := paramsByName(objectVariant.Children)
	assertParameter(t, children, Parameter{
		Name:        "query",
		Description: "Query text.",
		Required:    true,
		Type:        "string",
	})
	assertParameter(t, children, Parameter{
		Name:        "filters",
		Description: "Filter text.",
		Required:    false,
		Type:        "string",
	})

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<Tab title=\"Object\">",
		"Query text.",
		"Filter text.",
	})
}

func TestGetAPIDataRendersReturns(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/returns:
    post:
      operationId: createReturn
      summary: Create return
      description: Create return.
      responses:
        '201':
          description: Created
          content:
            application/json:
              schema:
                allOf:
                  - type: object
                    properties:
                      taskID:
                        type: string
                        description: Task identifier.
                    required:
                      - taskID
                  - type: object
                    properties:
                      result:
                        oneOf:
                          - type: string
                            description: String result.
                          - type: object
                            description: Object result.
                            properties:
                              status:
                                type: string
                                description: Result status.
                            required:
                              - status
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

	if got := len(data[0].Responses); got != 1 {
		t.Fatalf("responses len = %d, want 1", got)
	}

	if got, want := data[0].Responses[0].StatusCode, "201"; got != want {
		t.Fatalf("success response status = %q, want %q", got, want)
	}

	returns := paramsByNameFromResponses(data[0].Responses[0].Fields)
	assertResponseField(t, returns, ResponseField{
		Name:        "taskID",
		Description: "Task identifier.",
		Required:    true,
		Type:        "string",
	})

	result := requireResponseField(t, returns, "result")
	if got := len(result.Variants); got != 2 {
		t.Fatalf("result variants len = %d, want 2", got)
	}

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"## Returns",
		"The fields below describe the shared response shape.",
		"Some API clients wrap this payload in language-specific response objects or generics.",
		"<Tabs sync={false}>",
		"<Tab title=\"201\">",
		"Created",
		"<ResponseField name=\"taskID\" type=\"string\">",
		"Task identifier.",
		"<ResponseField name=\"result\" type=\"object | string\">",
		"<Tabs sync={false}>",
		"<Tab title=\"String\">",
		"String result.",
		"<Tab title=\"Object\">",
		"Object result.",
		"<ResponseField name=\"status\" type=\"string\">",
		"Result status.",
	})
}

func TestGetAPIDataCollapsesNullableUnionTabs(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/nullable:
    post:
      operationId: createNullable
      summary: Create nullable
      description: Create nullable.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                payload:
                  oneOf:
                    - type: object
                      description: Object payload.
                      properties:
                        query:
                          type: string
                          description: Query text.
                      required:
                        - query
                    - type: 'null'
              required:
                - payload
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  result:
                    anyOf:
                      - type: string
                        description: String result.
                      - type: 'null'
                required:
                  - result
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

	payload := requireParameter(t, paramsByName(data[0].Params), "payload")
	if got := len(payload.Variants); got != 0 {
		t.Fatalf("payload variants len = %d, want 0", got)
	}

	if payload.Type != "object | null" {
		t.Fatalf("payload type = %q, want %q", payload.Type, "object | null")
	}

	if got := len(payload.Children); got != 1 {
		t.Fatalf("payload children len = %d, want 1", got)
	}

	result := requireResponseField(
		t,
		paramsByNameFromResponses(data[0].Responses[0].Fields),
		"result",
	)
	if got := len(result.Variants); got != 0 {
		t.Fatalf("result variants len = %d, want 0", got)
	}

	if result.Type != "string | null" {
		t.Fatalf("result type = %q, want %q", result.Type, "string | null")
	}

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<ParamField path=\"payload\" type=\"object | null\" required>",
		"<ParamField path=\"query\" type=\"string\" required>",
		"<ResponseField name=\"result\" type=\"string | null\">",
	})
	assertRenderedNotContains(t, rendered.String(), []string{
		"<Tab title=\"Object\">",
		"<Tab title=\"String\">",
	})
}

func TestGetAPIDataKeepsTabsForNonNullVariantsWhenNullable(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/multi-nullable:
    post:
      operationId: createMultiNullable
      summary: Create multi nullable
      description: Create multi nullable.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                payload:
                  oneOf:
                    - type: string
                      description: String payload.
                    - type: object
                      description: Object payload.
                      properties:
                        query:
                          type: string
                          description: Query text.
                      required:
                        - query
                    - type: 'null'
              required:
                - payload
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

	payload := requireParameter(t, paramsByName(data[0].Params), "payload")
	if got := len(payload.Variants); got != 2 {
		t.Fatalf("payload variants len = %d, want 2", got)
	}

	if payload.Type != "object | string | null" {
		t.Fatalf("payload type = %q, want %q", payload.Type, "object | string | null")
	}

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<ParamField path=\"payload\" type=\"object | string | null\" required>",
		"<Tabs sync={false}>",
		"<Tab title=\"String\">",
		"<Tab title=\"Object\">",
	})
	assertRenderedNotContains(t, rendered.String(), []string{
		"<Tab title=\"null\">",
	})
}

func TestGetAPIDataUsesBetterVariantLabels(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/labels:
    post:
      operationId: createLabels
      summary: Create labels
      description: Create labels.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                searchParams:
                  oneOf:
                    - $ref: '#/components/schemas/searchParamsString'
                    - $ref: '#/components/schemas/searchParamsObject'
              required:
                - searchParams
components:
  schemas:
    searchParamsString:
      type: string
      description: Search parameters as query string.
    searchParamsObject:
      type: object
      description: Search parameters as object.
      properties:
        query:
          type: string
          description: Search query.
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

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<Tab title=\"Query string\">",
		"<Tab title=\"Object\">",
	})
}

func TestGetAPIDataInlinesLiteralUnionTypesWithoutTabs(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/literals:
    post:
      operationId: createLiterals
      summary: Create literals
      description: Create literals.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                aroundRadius:
                  oneOf:
                    - type: integer
                    - type: string
                      enum:
                        - all
                typoTolerance:
                  description: Typo tolerance mode.
                  oneOf:
                    - type: boolean
                    - type: string
                      enum:
                        - min
                        - strict
              required:
                - aroundRadius
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

	params := paramsByName(data[0].Params)
	assertParameter(t, params, Parameter{
		Name:     "aroundRadius",
		Required: true,
		Type:     `integer | 'all'`,
	})
	assertParameter(t, params, Parameter{
		Name:        "typoTolerance",
		Description: "Typo tolerance mode.",
		Required:    false,
		Type:        `boolean | 'min' | 'strict'`,
	})

	if got := len(params["aroundRadius"].Variants); got != 0 {
		t.Fatalf("aroundRadius variants len = %d, want 0", got)
	}

	if got := len(params["typoTolerance"].Variants); got != 0 {
		t.Fatalf("typoTolerance variants len = %d, want 0", got)
	}

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<ParamField path=\"aroundRadius\" type=\"integer | &#39;all&#39;\" required>",
		"<ParamField path=\"typoTolerance\" type=\"boolean | &#39;min&#39; | &#39;strict&#39;\">",
	})
	assertRenderedNotContains(t, rendered.String(), []string{
		"<Tab title=\"All\">",
		"<Tab title=\"Min\">",
		"<Tab title=\"Strict\">",
	})
}

func TestGetAPIDataDoesNotInlineEnumsAboveThreshold(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/enums:
    post:
      operationId: createEnums
      summary: Create enums
      description: Create enums.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                typoTolerance:
                  type: string
                  description: Typo tolerance mode.
                  enum:
                    - false
                    - min
                    - strict
                    - disabled
                    - true
              required:
                - typoTolerance
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

	params := paramsByName(data[0].Params)

	field := requireParameter(t, params, "typoTolerance")
	if field.Type != "string" {
		t.Fatalf("typoTolerance type = %q, want %q", field.Type, "string")
	}

	if got := len(field.AllowedValues); got != 5 {
		t.Fatalf("allowed values len = %d, want 5", got)
	}

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<ParamField path=\"typoTolerance\" type=\"string\" required>",
		"<Expandable title=\"Allowed values\">",
		"- `false`",
		"- `'disabled'`",
		"- `true`",
	})
	assertRenderedNotContains(t, rendered.String(), []string{
		"boolean | &#39;false&#39; | &#39;min&#39; | &#39;strict&#39; | &#39;true&#39;",
	})
}

func TestGetAPIDataKeepsArrayTypeForPolymorphicRefItems(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/search:
    post:
      operationId: search
      summary: Search multiple indices
      description: Search multiple indices.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                requests:
                  type: array
                  items:
                    $ref: '#/components/schemas/SearchQuery'
              required:
                - requests
components:
  schemas:
    SearchQuery:
      oneOf:
        - $ref: '#/components/schemas/SearchForHits'
        - $ref: '#/components/schemas/SearchForFacets'
    SearchForHits:
      type: object
      properties:
        indexName:
          type: string
          description: Index to search.
        query:
          type: string
          description: Search query.
      required:
        - indexName
    SearchForFacets:
      type: object
      properties:
        indexName:
          type: string
          description: Index to search.
        facet:
          type: string
          description: Facet to search.
      required:
        - indexName
        - facet
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

	params := paramsByName(data[0].Params)
	assertParameter(t, params, Parameter{
		Name:     "requests",
		Required: true,
		Type:     "array<object>",
	})

	requests := requireParameter(t, params, "requests")
	if got := len(requests.Children); got != 0 {
		t.Fatalf("requests children len = %d, want 0", got)
	}

	if got := len(requests.Variants); got != 2 {
		t.Fatalf("requests variants len = %d, want 2", got)
	}

	hitsVariant := requireParameterVariant(t, requests.Variants, "Search For Hits")
	assertParameter(t, paramsByName(hitsVariant.Children), Parameter{
		Name:        "indexName",
		Description: "Index to search.",
		Required:    true,
		Type:        "string",
	})
	assertParameter(t, paramsByName(hitsVariant.Children), Parameter{
		Name:        "query",
		Description: "Search query.",
		Required:    false,
		Type:        "string",
	})

	facetsVariant := requireParameterVariant(t, requests.Variants, "Search For Facets")
	assertParameter(t, paramsByName(facetsVariant.Children), Parameter{
		Name:        "indexName",
		Description: "Index to search.",
		Required:    true,
		Type:        "string",
	})
	assertParameter(t, paramsByName(facetsVariant.Children), Parameter{
		Name:        "facet",
		Description: "Facet to search.",
		Required:    true,
		Type:        "string",
	})

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<ParamField path=\"requests\" type=\"object[]\" required>",
		"<Tab title=\"Search For Hits\">",
		"<Tab title=\"Search For Facets\">",
		"<ParamField path=\"query\" type=\"string\">",
		"<ParamField path=\"facet\" type=\"string\" required>",
	})
	assertRenderedNotContains(t, rendered.String(), []string{
		"<Expandable title=\"properties\">",
	})
}

func TestGetAPIDataNormalizesArrayOfCompositeObjectVariants(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/search:
    post:
      operationId: search
      summary: Search multiple indices
      description: Search multiple indices.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                requests:
                  type: array
                  items:
                    $ref: '#/components/schemas/SearchQuery'
              required:
                - requests
components:
  schemas:
    searchParamsString:
      type: string
    searchParamsObject:
      type: object
      properties:
        query:
          type: string
          description: Search query.
    searchParams:
      oneOf:
        - $ref: '#/components/schemas/searchParamsString'
        - $ref: '#/components/schemas/searchParamsObject'
    SearchForHitsOptions:
      type: object
      properties:
        indexName:
          type: string
          description: Index to search.
      required:
        - indexName
    SearchForFacetsOptions:
      type: object
      properties:
        indexName:
          type: string
          description: Index to search.
        facet:
          type: string
          description: Facet to search.
      required:
        - indexName
        - facet
    SearchForHits:
      allOf:
        - $ref: '#/components/schemas/searchParams'
        - $ref: '#/components/schemas/SearchForHitsOptions'
    SearchForFacets:
      allOf:
        - $ref: '#/components/schemas/searchParams'
        - $ref: '#/components/schemas/SearchForFacetsOptions'
    SearchQuery:
      oneOf:
        - $ref: '#/components/schemas/SearchForHits'
        - $ref: '#/components/schemas/SearchForFacets'
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

	params := paramsByName(data[0].Params)
	assertParameter(t, params, Parameter{
		Name:     "requests",
		Required: true,
		Type:     "array<object>",
	})

	requests := requireParameter(t, params, "requests")
	if got := len(requests.Variants); got != 4 {
		t.Fatalf("requests variants len = %d, want 4", got)
	}

	hitsQueryStringVariant := requireParameterVariant(
		t,
		requests.Variants,
		"Search For Hits Search Params string",
	)
	assertParameter(t, paramsByName(hitsQueryStringVariant.Children), Parameter{
		Name:        "indexName",
		Description: "Index to search.",
		Required:    true,
		Type:        "string",
	})

	hitsVariant := requireParameterVariant(t, requests.Variants, "Search For Hits Object")
	assertParameter(t, paramsByName(hitsVariant.Children), Parameter{
		Name:        "indexName",
		Description: "Index to search.",
		Required:    true,
		Type:        "string",
	})
	assertParameter(t, paramsByName(hitsVariant.Children), Parameter{
		Name:        "query",
		Description: "Search query.",
		Required:    false,
		Type:        "string",
	})

	facetsQueryStringVariant := requireParameterVariant(
		t,
		requests.Variants,
		"Search For Facets Search Params string",
	)
	assertParameter(t, paramsByName(facetsQueryStringVariant.Children), Parameter{
		Name:        "indexName",
		Description: "Index to search.",
		Required:    true,
		Type:        "string",
	})

	facetsVariant := requireParameterVariant(t, requests.Variants, "Search For Facets Object")
	assertParameter(t, paramsByName(facetsVariant.Children), Parameter{
		Name:        "indexName",
		Description: "Index to search.",
		Required:    true,
		Type:        "string",
	})
	assertParameter(t, paramsByName(facetsVariant.Children), Parameter{
		Name:        "facet",
		Description: "Facet to search.",
		Required:    true,
		Type:        "string",
	})

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<ParamField path=\"requests\" type=\"object[]\" required>",
		"<Tab title=\"Search For Hits Search Params string\">",
		"<Tab title=\"Search For Hits Object\">",
		"<Tab title=\"Search For Facets Search Params string\">",
		"<Tab title=\"Search For Facets Object\">",
		"<ParamField path=\"query\" type=\"string\">",
		"<ParamField path=\"facet\" type=\"string\" required>",
	})
}

func TestGetAPIDataRendersArrayItemVariantsInResponses(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/search:
    post:
      operationId: search
      summary: Search multiple indices
      description: Search multiple indices.
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  results:
                    type: array
                    items:
                      $ref: '#/components/schemas/SearchResult'
                required:
                  - results
components:
  schemas:
    SearchResult:
      oneOf:
        - $ref: '#/components/schemas/SearchForHitsResult'
        - $ref: '#/components/schemas/SearchForFacetsResult'
    SearchForHitsResult:
      type: object
      properties:
        hits:
          type: array
          items:
            type: string
        query:
          type: string
          description: Search query.
      required:
        - hits
    SearchForFacetsResult:
      type: object
      properties:
        facetHits:
          type: array
          items:
            type: string
        facet:
          type: string
          description: Facet name.
      required:
        - facetHits
        - facet
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

	results := requireResponseField(
		t,
		paramsByNameFromResponses(data[0].Responses[0].Fields),
		"results",
	)
	if got, want := results.Type, "array<object>"; got != want {
		t.Fatalf("results type = %q, want %q", got, want)
	}

	if got := len(results.Variants); got != 2 {
		t.Fatalf("results variants len = %d, want 2", got)
	}

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<ResponseField name=\"results\" type=\"array&lt;object&gt;\">",
		"<Tab title=\"Search For Hits Result\">",
		"<Tab title=\"Search For Facets Result\">",
		"<ResponseField name=\"query\" type=\"string\">",
		"<ResponseField name=\"facet\" type=\"string\">",
	})
}

func TestGetAPIDataRendersAllowedValuesForLargeEnums(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/enums:
    post:
      operationId: createEnums
      summary: Create enums
      description: Create enums.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                supportedLanguage:
                  type: string
                  description: Supported language.
                  enum:
                    - af
                    - ar
                    - az
                    - bg
                    - bn
                    - ca
                    - cs
                    - cy
                    - da
              required:
                - supportedLanguage
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

	params := paramsByName(data[0].Params)

	field := requireParameter(t, params, "supportedLanguage")
	if field.Type != "string" {
		t.Fatalf("supportedLanguage type = %q, want %q", field.Type, "string")
	}

	if got := len(field.AllowedValues); got != 9 {
		t.Fatalf("allowed values len = %d, want 9", got)
	}

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<ParamField path=\"supportedLanguage\" type=\"string\" required>",
		"<Expandable title=\"Allowed values\">",
		"- `'af'`",
		"- `'da'`",
	})
}

func TestGetAPIDataSortsParametersRequiredFirstThenAlphabetically(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/sorted:
    post:
      operationId: createSorted
      summary: Create sorted
      description: Create sorted.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                zeta:
                  type: string
                  description: Zeta field.
                alpha:
                  type: object
                  description: Alpha field.
                  properties:
                    zulu:
                      type: string
                      description: Zulu child.
                    beta:
                      type: string
                      description: Beta child.
                  required:
                    - zulu
                mixed:
                  oneOf:
                    - type: object
                      description: Object variant.
                      properties:
                        zulu:
                          type: string
                          description: Zulu variant child.
                        alpha:
                          type: string
                          description: Alpha variant child.
                      required:
                        - zulu
                    - type: string
                      description: String variant.
              required:
                - mixed
                - zeta
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  zeta:
                    type: string
                    description: Zeta response.
                  alpha:
                    type: string
                    description: Alpha response.
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

	if got, want := parameterNames(
		data[0].Params,
	), []string{
		"mixed",
		"zeta",
		"alpha",
	}; !equalStrings(
		got,
		want,
	) {
		t.Fatalf("top-level params = %#v, want %#v", got, want)
	}

	alpha := requireParameter(t, paramsByName(data[0].Params), "alpha")
	if got, want := parameterNames(
		alpha.Children,
	), []string{
		"zulu",
		"beta",
	}; !equalStrings(
		got,
		want,
	) {
		t.Fatalf("alpha children = %#v, want %#v", got, want)
	}

	mixed := requireParameter(t, paramsByName(data[0].Params), "mixed")
	if got := len(mixed.Variants); got != 2 {
		t.Fatalf("mixed variants len = %d, want 2", got)
	}

	objectVariant := requireVariantWithChildren(t, mixed.Variants)
	if got, want := parameterNames(
		objectVariant.Children,
	), []string{
		"zulu",
		"alpha",
	}; !equalStrings(
		got,
		want,
	) {
		t.Fatalf("variant children = %#v, want %#v", got, want)
	}

	if got, want := responseFieldNames(
		data[0].Responses[0].Fields,
	), []string{
		"alpha",
		"zeta",
	}; !equalStrings(
		got,
		want,
	) {
		t.Fatalf("returns = %#v, want %#v", got, want)
	}
}

func TestGetAPIDataRendersSuccessAndErrorResponses(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/responses:
    post:
      operationId: createResponses
      summary: Create responses
      description: Create responses.
      responses:
        '202':
          description: Accepted
          content:
            application/json:
              schema:
                type: object
                properties:
                  taskID:
                    type: string
                    description: Task identifier.
                required:
                  - taskID
        '400':
          description: Bad request
          content:
            application/json:
              schema:
                type: object
                properties:
                  message:
                    type: string
                    description: Error message.
                required:
                  - message
        default:
          description: Unexpected error
          content:
            application/json:
              schema:
                type: object
                properties:
                  detail:
                    type: string
                    description: Error detail.
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

	if got := len(data[0].Responses); got != 3 {
		t.Fatalf("responses len = %d, want 3", got)
	}

	if got, want := data[0].Responses[0].StatusCode, "202"; got != want {
		t.Fatalf("first response status = %q, want %q", got, want)
	}

	if got, want := data[0].Responses[1].StatusCode, "400"; got != want {
		t.Fatalf("second response status = %q, want %q", got, want)
	}

	if got, want := data[0].Responses[2].StatusCode, "default"; got != want {
		t.Fatalf("third response status = %q, want %q", got, want)
	}

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"## Returns",
		"<Tabs sync={false}>",
		"<Tab title=\"202\">",
		"Accepted",
		"<ResponseField name=\"taskID\" type=\"string\">",
		"<Tab title=\"400\">",
		"Bad request",
		"<ResponseField name=\"message\" type=\"string\">",
		"<Tab title=\"default\">",
		"Unexpected error",
		"<ResponseField name=\"detail\" type=\"string\">",
	})
	assertRenderedNotContains(t, rendered.String(), []string{
		"## Errors",
	})
}

func TestGetAPIDataKeepsDescriptionOnlyResponses(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/multi-success:
    post:
      operationId: createMultiSuccess
      summary: Create multi success
      description: Create multi success.
      responses:
        '200':
          description: Empty success
        '201':
          description: Created
          content:
            application/json:
              schema:
                type: object
                properties:
                  taskID:
                    type: string
                    description: Task identifier.
                required:
                  - taskID
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

	if got := len(data[0].Responses); got != 2 {
		t.Fatalf("responses len = %d, want 2", got)
	}

	if got, want := data[0].Responses[0].StatusCode, "200"; got != want {
		t.Fatalf("first response status = %q, want %q", got, want)
	}

	if got, want := data[0].Responses[1].StatusCode, "201"; got != want {
		t.Fatalf("second response status = %q, want %q", got, want)
	}

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"## Returns",
		"<Tabs sync={false}>",
		"<Tab title=\"200\">",
		"Empty success",
		"<Tab title=\"201\">",
		"Created",
		"<ResponseField name=\"taskID\" type=\"string\">",
	})
}

func TestGetAPIDataPrefersJSONResponseContentOverEarlierNonJSON(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/multi-content:
    get:
      operationId: getMultiContent
      summary: Get multi content
      description: Get multi content.
      responses:
        '200':
          description: OK
          content:
            text/plain:
              schema:
                type: string
                description: Plain text body.
            application/json:
              schema:
                type: object
                properties:
                  taskID:
                    type: string
                    description: Task identifier.
                required:
                  - taskID
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

	if got := len(data[0].Responses); got != 1 {
		t.Fatalf("responses len = %d, want 1", got)
	}

	fields := paramsByNameFromResponses(data[0].Responses[0].Fields)
	assertResponseField(t, fields, ResponseField{
		Name:        "taskID",
		Description: "Task identifier.",
		Required:    true,
		Type:        "string",
	})

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<ResponseField name=\"taskID\" type=\"string\">",
		"Task identifier.",
	})
	assertRenderedNotContains(t, rendered.String(), []string{
		"Plain text body.",
		"<ResponseField name=\"response\" type=\"string\">",
	})
}

func TestGetAPIDataPrefersJSONRequestContentOverEarlierNonJSON(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/multi-request-content:
    patch:
      operationId: patchMultiContent
      summary: Patch multi content
      description: Patch multi content.
      requestBody:
        required: true
        content:
          text/plain:
            schema:
              type: string
              description: Plain text body.
          application/merge-patch+json:
            schema:
              type: object
              properties:
                query:
                  type: string
                  description: Query text.
              required:
                - query
      responses:
        '200':
          description: OK
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

	params := paramsByName(data[0].Params)
	assertParameter(t, params, Parameter{
		Name:        "query",
		Description: "Query text.",
		Required:    true,
		Type:        "string",
	})

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"Query text.",
		`<ParamField path="query" type="string" required>`,
	})
	assertRenderedNotContains(t, rendered.String(), []string{
		"Plain text body.",
		`<ParamField path="requestBody" type="string" required>`,
	})
}

func TestGetAPIDataRendersTypedOptionalLeafRequestAndResponseFields(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/typed-optional-leaves:
    post:
      operationId: createTypedOptionalLeaves
      summary: Create typed optional leaves
      description: Create typed optional leaves.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                silentFlag:
                  type: boolean
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  cursor:
                    type: string
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

	assertParameter(t, paramsByName(data[0].Params), Parameter{
		Name:     "silentFlag",
		Type:     "boolean",
		Required: false,
	})
	assertResponseField(t, paramsByNameFromResponses(data[0].Responses[0].Fields), ResponseField{
		Name:     "cursor",
		Type:     "string",
		Required: false,
	})

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		`<ParamField path="silentFlag" type="boolean">`,
		`<ResponseField name="cursor" type="string">`,
	})
}

func TestGetAPIDataRendersDeprecatedParametersAndResponseFields(t *testing.T) {
	t.Parallel()

	spec := []byte(`openapi: 3.0.0
info:
  title: Search API
  version: 1.0.0
paths:
  /1/deprecated-fields:
    post:
      operationId: createDeprecatedFields
      summary: Create deprecated fields
      description: Create deprecated fields.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                legacyParam:
                  type: string
                  deprecated: true
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  legacyField:
                    type: string
                    deprecated: true
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

	params := paramsByName(data[0].Params)
	assertParameter(t, params, Parameter{
		Name:       "legacyParam",
		Deprecated: true,
		Type:       "string",
	})

	fields := paramsByNameFromResponses(data[0].Responses[0].Fields)
	assertResponseField(t, fields, ResponseField{
		Name:       "legacyField",
		Deprecated: true,
		Type:       "string",
	})

	tmpl := buildMethodTemplate(t)

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data[0]); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertRenderedContains(t, rendered.String(), []string{
		"<ParamField path=\"legacyParam\" type=\"string\" deprecated>",
		"<ResponseField name=\"legacyField\" type=\"string\" deprecated>",
	})
}

func assertNestedUserParameters(t *testing.T, params []Parameter) {
	t.Helper()

	paramsMap := paramsByName(params)
	user := requireParameter(t, paramsMap, "user")

	if user.Type != "object" || len(user.Children) != 2 {
		t.Fatalf("user parameter = %#v, want object with 2 children", user)
	}

	childByName := paramsByName(user.Children)
	assertParameter(t, childByName, Parameter{
		Name:        "id",
		Description: "User identifier.",
		Required:    true,
		Type:        "string",
	})

	profile := requireParameter(t, childByName, "profile")
	if profile.Type != "object" || len(profile.Children) != 1 {
		t.Fatalf("profile parameter = %#v, want object with 1 child", profile)
	}

	grandChildByName := paramsByName(profile.Children)
	assertParameter(t, grandChildByName, Parameter{
		Name:        "displayName",
		Description: "Public display name.",
		Required:    true,
		Type:        "string",
	})
}

func parameterNames(params []Parameter) []string {
	result := make([]string, 0, len(params))
	for _, param := range params {
		result = append(result, param.Name)
	}

	return result
}

func responseFieldNames(fields []ResponseField) []string {
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		result = append(result, field.Name)
	}

	return result
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

func paramsByNameFromResponses(fields []ResponseField) map[string]ResponseField {
	result := make(map[string]ResponseField, len(fields))
	for _, field := range fields {
		result[field.Name] = field
	}

	return result
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

func assertFrontmatterOperationIDs(t *testing.T, frontmatter map[string]any, want []string) {
	t.Helper()

	raw, ok := frontmatter["operationId"].([]any)
	if !ok {
		t.Fatalf("frontmatter operationId = %#v, want %#v", frontmatter["operationId"], want)
	}

	if len(raw) != len(want) {
		t.Fatalf("frontmatter operationId len = %d, want %d (%#v)", len(raw), len(want), raw)
	}

	for i := range want {
		got, ok := raw[i].(string)
		if !ok || got != want[i] {
			t.Fatalf("frontmatter operationId[%d] = %#v, want %q", i, raw[i], want[i])
		}
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

func assertRenderedNotContains(t *testing.T, rendered string, wants []string) {
	t.Helper()

	for _, want := range wants {
		if strings.Contains(rendered, want) {
			t.Fatalf("rendered method unexpectedly contains %q:\n%s", want, rendered)
		}
	}
}

func assertParameter(t *testing.T, params map[string]Parameter, want Parameter) {
	t.Helper()

	got := requireParameter(t, params, want.Name, want.In)

	if (want.In != "" && got.In != want.In) || got.Description != want.Description ||
		got.Deprecated != want.Deprecated || got.Required != want.Required ||
		got.Type != want.Type {
		t.Fatalf("parameter %q = %#v, want %#v", want.Name, got, want)
	}
}

func assertResponseField(t *testing.T, fields map[string]ResponseField, want ResponseField) {
	t.Helper()

	got := requireResponseField(t, fields, want.Name)

	if got.Description != want.Description || got.Deprecated != want.Deprecated ||
		got.Required != want.Required ||
		got.Type != want.Type {
		t.Fatalf("response field %q = %#v, want %#v", want.Name, got, want)
	}
}

func requireParameter(
	t *testing.T,
	params map[string]Parameter,
	name string,
	in ...string,
) Parameter {
	t.Helper()

	key := name
	if len(in) > 0 && in[0] != "" {
		key = parameterKey(name, in[0])
	}

	got, ok := params[key]
	if !ok {
		location := ""
		if len(in) > 0 {
			location = in[0]
		}

		t.Fatalf("parameter %q in %q missing in %#v", name, location, params)
	}

	return got
}

func requireResponseField(
	t *testing.T,
	fields map[string]ResponseField,
	name string,
) ResponseField {
	t.Helper()

	got, ok := fields[name]
	if !ok {
		t.Fatalf("response field %q missing in %#v", name, fields)
	}

	return got
}

func requireVariantWithChildren(t *testing.T, variants []ParameterVariant) ParameterVariant {
	t.Helper()

	for _, variant := range variants {
		if len(variant.Children) > 0 {
			return variant
		}
	}

	t.Fatalf("no variant with children in %#v", variants)

	return ParameterVariant{}
}

func requireParameterVariant(
	t *testing.T,
	variants []ParameterVariant,
	title string,
) ParameterVariant {
	t.Helper()

	for _, variant := range variants {
		if variant.Title == title {
			return variant
		}
	}

	t.Fatalf("variant %q missing in %#v", title, variants)

	return ParameterVariant{}
}

func paramsByName(params []Parameter) map[string]Parameter {
	result := make(map[string]Parameter, len(params))
	for _, param := range params {
		result[param.Name] = param
	}

	return result
}

func paramsByNameAndIn(params []Parameter) map[string]Parameter {
	result := make(map[string]Parameter, len(params))
	for _, param := range params {
		result[parameterKey(param.Name, param.In)] = param
	}

	return result
}
