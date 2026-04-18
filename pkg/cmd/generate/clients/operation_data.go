package clients

import (
	"strings"

	"github.com/algolia/docli/pkg/dictionary"
	base "github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
)

func getCodeSamples(op *v3.Operation) []CodeSample {
	node, ok := op.Extensions.Get("x-codeSamples")
	if !ok {
		return nil
	}

	var result []CodeSample

	for _, child := range node.Content {
		var c CodeSample

		child.Decode(&c)

		c.Lang = dictionary.NormalizeLang(c.Lang)

		if strings.ToLower(c.Label) != "curl" {
			result = append(result, c)
		}
	}

	return result
}

func getParameters(pathItem *v3.PathItem, op *v3.Operation) ([]Parameter, error) {
	result := make([]Parameter, 0, len(pathItem.Parameters)+len(op.Parameters))
	indexes := map[string]int{}

	for _, p := range pathItem.Parameters {
		if err := appendOpenAPIParameter(&result, indexes, p, false); err != nil {
			return nil, err
		}
	}

	for _, p := range op.Parameters {
		if err := appendOpenAPIParameter(&result, indexes, p, true); err != nil {
			return nil, err
		}
	}

	bodyParams, err := getRequestBodyParameters(op)
	if err != nil {
		return nil, err
	}

	result = append(result, bodyParams...)

	return result, nil
}

func appendOpenAPIParameter(
	result *[]Parameter,
	indexes map[string]int,
	p *v3.Parameter,
	replace bool,
) error {
	if p == nil {
		return nil
	}

	name := strings.TrimSpace(p.Name)
	if name == "" {
		return nil
	}

	key := parameterKey(name, p.In)
	param := buildParameter(
		name,
		strings.TrimSpace(p.Description),
		boolOrFalse(p.Required),
		p.In,
		parameterSchema(p),
	)

	if idx, ok := indexes[key]; ok {
		if replace {
			(*result)[idx] = param
		}

		return nil
	}

	indexes[key] = len(*result)
	*result = append(*result, param)

	return nil
}

func parameterKey(name, in string) string {
	return strings.TrimSpace(in) + ":" + strings.TrimSpace(name)
}

func getRequestBodyParameters(op *v3.Operation) ([]Parameter, error) {
	if op.RequestBody == nil || op.RequestBody.Content == nil {
		return nil, nil
	}

	mediaType := getRequestMediaType(op.RequestBody.Content)
	if mediaType == nil || mediaType.Schema == nil {
		return nil, nil
	}

	schema, err := mediaType.Schema.BuildSchema()
	if err != nil {
		return nil, err
	}

	if schema != nil {
		if params := schemaObjectChildren(schema, map[string]bool{}); len(params) > 0 {
			if hasNamedRequestBody(op, schema, mediaType.Schema) {
				param := buildSyntheticRequestBodyParameter(op, schema, mediaType.Schema)
				param.Children = params

				return []Parameter{param}, nil
			}

			return params, nil
		}
	}

	return []Parameter{buildSyntheticRequestBodyParameter(op, schema, mediaType.Schema)}, nil
}

func getResponses(op *v3.Operation) []OperationResponse {
	if op == nil || op.Responses == nil {
		return nil
	}

	var responses []OperationResponse

	if op.Responses.Codes != nil {
		for pair := op.Responses.Codes.First(); pair != nil; pair = pair.Next() {
			statusCode := strings.TrimSpace(pair.Key())

			response := buildOperationResponse(statusCode, pair.Value())
			if response == nil {
				continue
			}

			responses = append(responses, *response)
		}
	}

	if response := buildOperationResponse("default", op.Responses.Default); response != nil {
		responses = append(responses, *response)
	}

	return responses
}

func buildOperationResponse(statusCode string, response *v3.Response) *OperationResponse {
	if response == nil {
		return nil
	}

	description := strings.TrimSpace(response.Description)

	if response.Content == nil {
		return buildDescriptionOnlyResponse(statusCode, description)
	}

	mediaType := getResponseMediaType(response.Content)
	if mediaType == nil || mediaType.Schema == nil {
		return buildDescriptionOnlyResponse(statusCode, description)
	}

	schema, err := mediaType.Schema.BuildSchema()
	if err != nil {
		return nil
	}

	fields := []ResponseField(nil)
	if schema != nil {
		fields = responseFieldsFromSchemaObject(schema, map[string]bool{})
	}

	if len(fields) == 0 {
		fields = []ResponseField{buildSyntheticResponseField(response, mediaType.Schema)}
	}

	fields = sortResponseFields(pruneResponseFields(fields))
	if len(fields) == 0 && description == "" {
		return nil
	}

	return &OperationResponse{
		StatusCode:   statusCode,
		Description:  description,
		Fields:       fields,
		SortPriority: responseSortPriority(statusCode),
	}
}

func buildDescriptionOnlyResponse(statusCode, description string) *OperationResponse {
	if description == "" {
		return nil
	}

	return &OperationResponse{
		StatusCode:   statusCode,
		Description:  description,
		SortPriority: responseSortPriority(statusCode),
	}
}

func responseSortPriority(code string) int {
	if code == "default" {
		return 1000
	}

	if len(code) != 3 {
		return 999
	}

	value := 0

	for _, r := range code {
		if r < '0' || r > '9' {
			return 999
		}

		value = value*10 + int(r-'0')
	}

	return value
}

func buildSyntheticResponseField(response *v3.Response, proxy *base.SchemaProxy) ResponseField {
	children := schemaProxyChildren(proxy, map[string]bool{})
	variants := schemaProxyVariants(proxy, map[string]bool{})
	typeSummary := normalizeVariantParentType(
		schemaProxyTypeSummary(proxy, map[string]bool{}),
		variants,
	)
	allowedValues := schemaProxyAllowedValues(proxy, map[string]bool{})

	return responseFieldFromParameter(Parameter{
		Name:          "response",
		Description:   strings.TrimSpace(response.Description),
		Deprecated:    schemaProxyDeprecated(proxy),
		Type:          typeSummary,
		Children:      children,
		Variants:      variants,
		AllowedValues: allowedValues,
	})
}

func responseFieldsFromSchemaObject(schema *base.Schema, seen map[string]bool) []ResponseField {
	params := schemaObjectChildren(schema, seen)
	if len(params) == 0 {
		return nil
	}

	return responseFieldsFromParameters(params)
}

func buildSyntheticRequestBodyParameter(
	op *v3.Operation,
	schema *base.Schema,
	proxy *base.SchemaProxy,
) Parameter {
	return buildParameter(
		requestBodyName(op, schema, proxy),
		strings.TrimSpace(op.RequestBody.Description),
		boolOrFalse(op.RequestBody.Required),
		"body",
		proxy,
	)
}

func getObjectParameters(
	properties *orderedmap.Map[string, *base.SchemaProxy],
	required []string,
) []Parameter {
	return getObjectParametersWithSeen(properties, required, map[string]bool{})
}

func getObjectParametersWithSeen(
	properties *orderedmap.Map[string, *base.SchemaProxy],
	required []string,
	seen map[string]bool,
) []Parameter {
	var result []Parameter

	requiredSet := make(map[string]struct{}, len(required))
	for _, name := range required {
		requiredSet[name] = struct{}{}
	}

	for prop := properties.First(); prop != nil; prop = prop.Next() {
		result = append(
			result,
			buildParameterWithSeen(
				prop.Key(),
				strings.TrimSpace(schemaDescription(prop.Value())),
				hasKey(requiredSet, prop.Key()),
				"body",
				prop.Value(),
				seen,
			),
		)
	}

	return result
}

func buildParameter(
	name, description string,
	required bool,
	in string,
	proxy *base.SchemaProxy,
) Parameter {
	return buildParameterWithSeen(name, description, required, in, proxy, map[string]bool{})
}

func buildParameterWithSeen(
	name, description string,
	required bool,
	in string,
	proxy *base.SchemaProxy,
	seen map[string]bool,
) Parameter {
	children := schemaProxyChildren(proxy, cloneSeenRefs(seen))
	variants := schemaProxyVariants(proxy, cloneSeenRefs(seen))
	typeSummary := normalizeVariantParentType(
		schemaProxyTypeSummary(proxy, cloneSeenRefs(seen)),
		variants,
	)
	allowedValues := schemaProxyAllowedValues(proxy, cloneSeenRefs(seen))

	return Parameter{
		Name:          name,
		Description:   description,
		Deprecated:    schemaProxyDeprecated(proxy),
		Required:      required,
		Type:          typeSummary,
		In:            in,
		Children:      children,
		Variants:      variants,
		AllowedValues: allowedValues,
	}
}

func boolOrFalse(val *bool) bool {
	if val == nil {
		return false
	}

	return *val
}
