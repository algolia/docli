package clients

import (
	_ "embed"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"unicode"

	"github.com/MakeNowJust/heredoc"
	"github.com/algolia/docli/pkg/cmd/generate/utils"
	"github.com/algolia/docli/pkg/dictionary"
	"github.com/algolia/docli/pkg/output"
	"github.com/algolia/docli/pkg/validate"
	"github.com/pb33f/libopenapi"
	base "github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
)

// Options represents configuration options and CLI flags for this command.
type Options struct {
	APIName         string
	InputFilename   string
	OutputDirectory string
}

// ExternalDocs holds an externalDocs reference.
type ExternalDocs struct {
	Description string
	URL         string
}

// OperationData represents relevant information about an API operation.
type OperationData struct {
	ACL              string
	APIName          string
	CodeSamples      []CodeSample
	Deprecated       bool
	Description      string
	ExternalDocs     ExternalDocs
	InputFilename    string
	OutputFilename   string
	OutputPath       string
	OperationIDKebab string
	Params           []Parameter
	Returns          []ResponseField
	RequiresAdmin    bool
	SeeAlso          bool
	ShortDescription string
	Summary          string
}

type CodeSample struct {
	Lang   string
	Label  string
	Source string
}

type Parameter struct {
	Name          string
	Description   string
	Required      bool
	Type          string
	In            string
	Children      []Parameter
	Variants      []ParameterVariant
	AllowedValues []string
}

type ParameterVariant struct {
	Title         string
	Description   string
	Type          string
	Children      []Parameter
	AllowedValues []string
}

type ResponseField struct {
	Name          string
	Description   string
	Required      bool
	Type          string
	Children      []ResponseField
	Variants      []ResponseVariant
	AllowedValues []string
}

type ResponseVariant struct {
	Title         string
	Description   string
	Type          string
	Children      []ResponseField
	AllowedValues []string
}

//go:embed method.mdx.tmpl
var methodTemplate string

func NewClientsCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:     "clients",
		Aliases: []string{"c"},
		Short:   "Generate MDX files for the API client method references",
		Long: heredoc.Doc(`
			This command reads an OpenAPI 3 spec file and generates one MDX file per operation.
			It writes an API reference with usage information specific to API clients,
			which may follow different conventions depending on the programming language used.
			This command doesn't delete MDX files. If you remove or rename an operation,
			you need to update or delete its MDX file manually.
		`),
		Example: heredoc.Doc(`
			# Run from root of algolia/docs-new
			docli gen clients specs/search.yml -o doc/libraries/sdk/methods
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.InputFilename = args[0]
			opts.APIName = utils.GetAPIName(opts.InputFilename)

			printer, err := output.New(cmd)
			if err != nil {
				return err
			}

			return runCommand(opts, printer)
		},
	}

	cmd.Flags().
		StringVarP(&opts.OutputDirectory, "output", "o", "out", "Output directory for generated MDX files")

	return cmd
}

func runCommand(opts *Options, printer *output.Printer) error {
	if err := validate.ExistingFile(opts.InputFilename, "spec file"); err != nil {
		return err
	}

	if err := validate.OutputDir(opts.OutputDirectory, "output directory"); err != nil {
		return err
	}

	specFile, err := os.ReadFile(opts.InputFilename)
	if err != nil {
		return fmt.Errorf("read spec file %s: %w", opts.InputFilename, err)
	}

	printer.Infof("Generating API client references for spec: %s\n", opts.InputFilename)
	printer.Infof("Writing output in: %s\n", opts.OutputDirectory)

	spec, err := utils.LoadSpec(specFile)
	if err != nil {
		return fmt.Errorf("load spec %s: %w", opts.InputFilename, err)
	}

	opData, err := getAPIData(spec, opts)
	if err != nil {
		return fmt.Errorf("parse spec %s: %w", opts.InputFilename, err)
	}

	printer.Verbosef("Spec %s has %d operations.\n", opts.InputFilename, len(opData))

	tmpl := template.Must(template.New("method").Funcs(template.FuncMap{
		"frontmatterString":    utils.QuoteFrontmatterString,
		"mintFieldType":        mintFieldType,
		"renderParamFields":    renderParamFields,
		"renderResponseFields": renderResponseFields,
		"trim":                 strings.TrimSpace,
	}).Parse(methodTemplate))

	if err := writeAPIData(opData, tmpl, printer); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}

// getAPIData reads the OpenAPI spec and parses the operation data.
//
//nolint:funlen
func getAPIData(
	doc *libopenapi.DocumentModel[v3.Document],
	opts *Options,
) ([]OperationData, error) {
	var result []OperationData

	count := 0

	prefix := fmt.Sprintf("%s/%s", opts.OutputDirectory, opts.APIName)

	for pathPairs := doc.Model.Paths.PathItems.First(); pathPairs != nil; pathPairs = pathPairs.Next() {
		pathName := pathPairs.Key()
		// Ignore custom HTTP requests
		if pathName == "/{path}" {
			continue
		}

		pathItem := pathPairs.Value()

		for opPairs := pathItem.GetOperations().First(); opPairs != nil; opPairs = opPairs.Next() {
			op := opPairs.Value()

			acl, err := utils.GetACL(op)
			if err != nil {
				return nil, fmt.Errorf("get ACL for %s %s: %w", opPairs.Key(), pathName, err)
			}

			short, long := utils.SplitDescription(op.Description)
			short = utils.StripMarkdown(short)

			params, err := getParameters(pathItem, op)
			if err != nil {
				return nil, fmt.Errorf("get parameters for %s %s: %w", opPairs.Key(), pathName, err)
			}

			data := OperationData{
				ACL:              utils.AclToString(acl),
				APIName:          opts.APIName,
				CodeSamples:      getCodeSamples(op),
				Deprecated:       boolOrFalse(op.Deprecated),
				Description:      long,
				OutputFilename:   utils.GetOutputFilename(op),
				OutputPath:       prefix,
				OperationIDKebab: utils.ToKebabCase(op.OperationId),
				Params:           sortParameters(pruneParameters(params)),
				Returns:          sortResponseFields(getReturns(op)),
				RequiresAdmin:    false,
				ShortDescription: short,
				Summary:          op.Summary,
			}

			if data.ACL == "`admin`" {
				data.RequiresAdmin = true
			}

			if op.ExternalDocs != nil {
				desc := strings.TrimSpace(op.ExternalDocs.Description)
				data.ExternalDocs.Description = strings.TrimSuffix(desc, ".")
				data.ExternalDocs.URL = op.ExternalDocs.URL
			}

			if data.ExternalDocs.Description != "" && data.ExternalDocs.URL != "" {
				data.SeeAlso = true
			}

			result = append(result, data)
			count++
		}
	}

	return result, nil
}

// writeAPIData writes the OpenAPI data to MDX files.
func writeAPIData(
	data []OperationData,
	template *template.Template,
	printer *output.Printer,
) error {
	for _, item := range data {
		if !printer.IsDryRun() {
			if err := os.MkdirAll(item.OutputPath, 0o700); err != nil {
				return err
			}
		}

		fullPath := filepath.Join(item.OutputPath, item.OutputFilename)

		if err := printer.WriteFile(fullPath, func(w io.Writer) error {
			return template.Execute(w, item)
		}); err != nil {
			return err
		}
	}

	return nil
}

func getCodeSamples(op *v3.Operation) []CodeSample {
	node, ok := op.Extensions.Get("x-codeSamples")
	// Operations can be without code samples
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
	seen := map[string]struct{}{}

	for _, p := range pathItem.Parameters {
		if err := appendOpenAPIParameter(&result, seen, p); err != nil {
			return nil, err
		}
	}

	for _, p := range op.Parameters {
		if err := appendOpenAPIParameter(&result, seen, p); err != nil {
			return nil, err
		}
	}

	bodyParams, err := getRequestBodyParameters(op)
	if err != nil {
		return nil, err
	}

	for _, p := range bodyParams {
		if _, ok := seen[p.Name]; ok {
			result = removeParameterByName(result, p.Name)
		}

		result = append(result, p)
		seen[p.Name] = struct{}{}
	}

	return result, nil
}

func appendOpenAPIParameter(result *[]Parameter, seen map[string]struct{}, p *v3.Parameter) error {
	if p == nil {
		return nil
	}

	name := strings.TrimSpace(p.Name)
	if name == "" {
		return nil
	}

	if _, ok := seen[name]; ok {
		return nil
	}

	*result = append(*result, buildParameter(
		name,
		strings.TrimSpace(p.Description),
		boolOrFalse(p.Required),
		p.In,
		parameterSchema(p),
	))

	seen[name] = struct{}{}

	return nil
}

func getRequestBodyParameters(op *v3.Operation) ([]Parameter, error) {
	if op.RequestBody == nil || op.RequestBody.Content == nil {
		return nil, nil
	}

	mediaType := getJSONMediaType(op.RequestBody.Content)
	if mediaType == nil || mediaType.Schema == nil {
		return nil, nil
	}

	schema, err := mediaType.Schema.BuildSchema()
	if err != nil {
		return nil, err
	}

	if schema != nil && schemaHasType(schema, "object") && schema.Properties != nil {
		return getObjectParameters(schema.Properties, schema.Required), nil
	}

	return []Parameter{buildSyntheticRequestBodyParameter(op, schema, mediaType.Schema)}, nil
}

func getReturns(op *v3.Operation) []ResponseField {
	response := getSuccessResponse(op)
	if response == nil || response.Content == nil {
		return nil
	}

	mediaType := getJSONMediaType(response.Content)
	if mediaType == nil || mediaType.Schema == nil {
		return nil
	}

	schema, err := mediaType.Schema.BuildSchema()
	if err != nil {
		return nil
	}

	if schema != nil {
		if fields := responseFieldsFromSchemaObject(schema, map[string]bool{}); len(fields) > 0 {
			return pruneResponseFields(fields)
		}
	}

	return pruneResponseFields(
		[]ResponseField{buildSyntheticResponseField(response, mediaType.Schema)},
	)
}

func getSuccessResponse(op *v3.Operation) *v3.Response {
	if op == nil || op.Responses == nil || op.Responses.Codes == nil {
		return nil
	}

	if response := op.Responses.FindResponseByCode(200); response != nil {
		return response
	}

	for pair := op.Responses.Codes.First(); pair != nil; pair = pair.Next() {
		code := strings.TrimSpace(pair.Key())
		if len(code) == 3 && code[0] == '2' {
			return pair.Value()
		}
	}

	return nil
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
		result = append(result, buildParameterWithSeen(
			prop.Key(),
			strings.TrimSpace(schemaDescription(prop.Value())),
			hasKey(requiredSet, prop.Key()),
			"body",
			prop.Value(),
			seen,
		))
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
		Required:      required,
		Type:          typeSummary,
		In:            in,
		Children:      children,
		Variants:      variants,
		AllowedValues: allowedValues,
	}
}

func normalizeVariantParentType(current string, variants []ParameterVariant) string {
	if strings.Count(current, "object") >= 2 {
		return "object"
	}

	if !variantsAllHaveChildren(variants) {
		return current
	}

	return "object"
}

func variantsAllHaveChildren(variants []ParameterVariant) bool {
	if len(variants) == 0 {
		return false
	}

	for _, variant := range variants {
		if len(variant.Children) == 0 {
			return false
		}
	}

	return true
}

func schemaProxyAllowedValues(proxy *base.SchemaProxy, seen map[string]bool) []string {
	if proxy == nil {
		return nil
	}

	ref, alreadySeen := enterSchemaRef(seen, proxy)
	if alreadySeen {
		return nil
	}

	defer leaveSchemaRef(seen, ref)

	schema, err := proxy.BuildSchema()
	if err != nil || schema == nil {
		return nil
	}

	if nullableProxy, ok := nullableSingleSchemaVariant(schema); ok {
		return schemaProxyAllowedValues(nullableProxy, seen)
	}

	return largeEnumValues(schema)
}

func schemaDescription(proxy *base.SchemaProxy) string {
	if proxy == nil {
		return ""
	}

	schema, err := proxy.BuildSchema()
	if err != nil || schema == nil {
		return ""
	}

	return schema.Description
}

func parameterSchema(p *v3.Parameter) *base.SchemaProxy {
	if p == nil {
		return nil
	}

	if p.Schema != nil {
		return p.Schema
	}

	if p.Content == nil {
		return nil
	}

	mediaType := getJSONMediaType(p.Content)
	if mediaType == nil {
		return nil
	}

	return mediaType.Schema
}

func getJSONMediaType(content *orderedmap.Map[string, *v3.MediaType]) *v3.MediaType {
	if content == nil {
		return nil
	}

	if mediaType, ok := content.Get("application/json"); ok {
		return mediaType
	}

	if mediaType, ok := content.Get("application/x-www-form-urlencoded"); ok {
		return mediaType
	}

	if mediaType, ok := content.Get("multipart/form-data"); ok {
		return mediaType
	}

	first := content.First()
	if first == nil {
		return nil
	}

	return first.Value()
}

func requestBodyName(op *v3.Operation, schema *base.Schema, proxy *base.SchemaProxy) string {
	if op.Extensions != nil {
		if node, ok := op.Extensions.Get("x-codegen-request-body-name"); ok {
			name := strings.TrimSpace(node.Value)
			if name != "" {
				return name
			}
		}
	}

	if proxy != nil {
		if ref := strings.TrimSpace(proxy.GetReference()); ref != "" {
			return refName(ref)
		}
	}

	if schema != nil {
		if title := strings.TrimSpace(schema.Title); title != "" {
			return title
		}
	}

	return "requestBody"
}

func schemaProxyTypeSummary(proxy *base.SchemaProxy, seen map[string]bool) string {
	if proxy == nil {
		return "object"
	}

	ref, alreadySeen := enterSchemaRef(seen, proxy)
	if alreadySeen {
		return refName(ref)
	}

	defer leaveSchemaRef(seen, ref)

	schema, err := proxy.BuildSchema()
	if err != nil || schema == nil {
		return fallbackTypeSummary(ref)
	}

	if summary := polymorphicTypeSummary(schema, seen); summary != "" {
		return summary
	}

	if summary := arrayTypeSummary(schema, seen); summary != "" {
		return summary
	}

	if summary := scalarTypeSummary(schema); summary != "" {
		return summary
	}

	return objectOrNamedTypeSummary(schema, ref)
}

func compositeTypeSummary(proxies []*base.SchemaProxy, sep string, seen map[string]bool) string {
	if len(proxies) == 0 {
		return ""
	}

	parts := make([]string, 0, len(proxies))
	seenParts := map[string]struct{}{}

	for _, proxy := range proxies {
		part := schemaProxyTypeSummary(proxy, cloneSeenRefs(seen))
		if part == "" {
			continue
		}

		if schemaProxyContainsRecursiveRef(proxy, cloneSeenRefs(seen)) {
			part += " (recursive)"
		}

		if _, ok := seenParts[part]; ok {
			continue
		}

		seenParts[part] = struct{}{}
		parts = append(parts, part)
	}

	if len(parts) == 0 {
		return ""
	}

	sort.Slice(parts, func(i, j int) bool {
		return compareTypeSummary(parts[i], parts[j]) < 0
	})

	return strings.Join(parts, sep)
}

func schemaHasType(schema *base.Schema, want string) bool {
	for _, got := range schema.Type {
		if got == want {
			return true
		}
	}

	return false
}

func refName(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "object"
	}

	if idx := strings.LastIndex(ref, "/"); idx != -1 && idx < len(ref)-1 {
		return ref[idx+1:]
	}

	return ref
}

func hasKey(values map[string]struct{}, key string) bool {
	_, ok := values[key]

	return ok
}

func removeParameterByName(params []Parameter, name string) []Parameter {
	result := params[:0]
	for _, param := range params {
		if param.Name == name {
			continue
		}

		result = append(result, param)
	}

	return result
}

func mintFieldType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "object"
	}

	switch value {
	case "integer":
		return "number"
	case "array<string>":
		return "string[]"
	case "array<integer>", "array<number>":
		return "number[]"
	case "array<boolean>":
		return "boolean[]"
	case "array<object>":
		return "object[]"
	default:
		return value
	}
}

func pruneParameters(params []Parameter) []Parameter {
	result := make([]Parameter, 0, len(params))
	for _, param := range params {
		param.Children = pruneParameters(param.Children)

		param.Variants = pruneVariants(param.Variants)

		if shouldRenderParameter(param) {
			result = append(result, param)
		}
	}

	return result
}

func shouldRenderParameter(param Parameter) bool {
	if param.Required {
		return true
	}

	if strings.TrimSpace(param.Description) != "" {
		return true
	}

	if len(param.Children) > 0 {
		return true
	}

	return len(param.Variants) > 0
}

func pruneVariants(variants []ParameterVariant) []ParameterVariant {
	result := make([]ParameterVariant, 0, len(variants))
	for _, variant := range variants {
		variant.Children = pruneParameters(variant.Children)
		if shouldRenderVariant(variant) {
			result = append(result, variant)
		}
	}

	return result
}

func shouldRenderVariant(variant ParameterVariant) bool {
	if strings.TrimSpace(variant.Description) != "" {
		return true
	}

	if len(variant.Children) > 0 {
		return true
	}

	return strings.TrimSpace(variant.Type) != ""
}

func sortParameters(params []Parameter) []Parameter {
	for idx := range params {
		params[idx].Children = sortParameters(params[idx].Children)
		params[idx].Variants = sortParameterVariants(params[idx].Variants)
	}

	sort.SliceStable(params, func(i, j int) bool {
		return strings.ToLower(params[i].Name) < strings.ToLower(params[j].Name)
	})

	return params
}

func sortParameterVariants(variants []ParameterVariant) []ParameterVariant {
	for idx := range variants {
		variants[idx].Children = sortParameters(variants[idx].Children)
	}

	return variants
}

func responseFieldsFromParameters(params []Parameter) []ResponseField {
	result := make([]ResponseField, 0, len(params))
	for _, param := range params {
		result = append(result, responseFieldFromParameter(param))
	}

	return result
}

func responseFieldFromParameter(param Parameter) ResponseField {
	return ResponseField{
		Name:          param.Name,
		Description:   param.Description,
		Required:      param.Required,
		Type:          param.Type,
		Children:      responseFieldsFromParameters(param.Children),
		Variants:      responseVariantsFromParameters(param.Variants),
		AllowedValues: append([]string(nil), param.AllowedValues...),
	}
}

func responseVariantsFromParameters(variants []ParameterVariant) []ResponseVariant {
	result := make([]ResponseVariant, 0, len(variants))
	for _, variant := range variants {
		result = append(result, ResponseVariant{
			Title:         variant.Title,
			Description:   variant.Description,
			Type:          variant.Type,
			Children:      responseFieldsFromParameters(variant.Children),
			AllowedValues: append([]string(nil), variant.AllowedValues...),
		})
	}

	return result
}

func pruneResponseFields(fields []ResponseField) []ResponseField {
	result := make([]ResponseField, 0, len(fields))
	for _, field := range fields {
		field.Children = pruneResponseFields(field.Children)

		field.Variants = pruneResponseVariants(field.Variants)

		if shouldRenderResponseField(field) {
			result = append(result, field)
		}
	}

	return result
}

func pruneResponseVariants(variants []ResponseVariant) []ResponseVariant {
	result := make([]ResponseVariant, 0, len(variants))
	for _, variant := range variants {
		variant.Children = pruneResponseFields(variant.Children)
		if shouldRenderResponseVariant(variant) {
			result = append(result, variant)
		}
	}

	return result
}

func shouldRenderResponseField(field ResponseField) bool {
	if field.Required {
		return true
	}

	if strings.TrimSpace(field.Description) != "" {
		return true
	}

	if len(field.Children) > 0 {
		return true
	}

	if len(field.AllowedValues) > 0 {
		return true
	}

	return len(field.Variants) > 0
}

func shouldRenderResponseVariant(variant ResponseVariant) bool {
	if strings.TrimSpace(variant.Description) != "" {
		return true
	}

	if len(variant.Children) > 0 {
		return true
	}

	if len(variant.AllowedValues) > 0 {
		return true
	}

	return strings.TrimSpace(variant.Type) != ""
}

func sortResponseFields(fields []ResponseField) []ResponseField {
	for idx := range fields {
		fields[idx].Children = sortResponseFields(fields[idx].Children)
		fields[idx].Variants = sortResponseVariants(fields[idx].Variants)
	}

	sort.SliceStable(fields, func(i, j int) bool {
		return strings.ToLower(fields[i].Name) < strings.ToLower(fields[j].Name)
	})

	return fields
}

func sortResponseVariants(variants []ResponseVariant) []ResponseVariant {
	for idx := range variants {
		variants[idx].Children = sortResponseFields(variants[idx].Children)
	}

	return variants
}

func renderParamFields(params []Parameter) string {
	var builder strings.Builder
	for _, param := range params {
		writeParamField(&builder, param)
	}

	return strings.TrimRight(builder.String(), "\n")
}

func renderResponseFields(fields []ResponseField) string {
	var builder strings.Builder
	for _, field := range fields {
		writeResponseField(&builder, field)
	}

	return strings.TrimRight(builder.String(), "\n")
}

func writeParamField(builder *strings.Builder, param Parameter) {
	builder.WriteString(`<ParamField path="`)
	builder.WriteString(escapeMDXAttr(param.Name))
	builder.WriteString(`" type="`)
	builder.WriteString(escapeMDXAttr(mintFieldType(param.Type)))
	builder.WriteString(`"`)

	if param.Required {
		builder.WriteString(` required`)
	}

	builder.WriteString(`>` + "\n\n")

	if description := strings.TrimSpace(param.Description); description != "" {
		builder.WriteString(description)
		builder.WriteString("\n\n")
	}

	if len(param.Variants) > 0 {
		writeParameterVariants(builder, param.Variants)
	}

	if len(param.AllowedValues) > 0 {
		writeAllowedValues(builder, param.AllowedValues)
	}

	if len(param.Children) > 0 {
		builder.WriteString(`<Expandable title="properties">` + "\n\n")

		for _, child := range param.Children {
			writeParamField(builder, child)
		}

		builder.WriteString(`</Expandable>` + "\n\n")
	}

	builder.WriteString(`</ParamField>` + "\n\n")
}

func writeParameterVariants(builder *strings.Builder, variants []ParameterVariant) {
	builder.WriteString(`<Tabs sync={false}>` + "\n\n")

	for _, variant := range variants {
		writeParameterVariant(builder, variant)
	}

	builder.WriteString(`</Tabs>` + "\n\n")
}

func writeParameterVariant(builder *strings.Builder, variant ParameterVariant) {
	builder.WriteString(`<Tab title="`)
	builder.WriteString(escapeMDXAttr(variant.Title))
	builder.WriteString(`">` + "\n\n")

	if description := strings.TrimSpace(variant.Description); description != "" {
		builder.WriteString(description)
		builder.WriteString("\n\n")
	}

	if len(variant.Children) > 0 {
		for _, child := range variant.Children {
			writeParamField(builder, child)
		}
	} else if len(variant.AllowedValues) > 0 {
		writeAllowedValues(builder, variant.AllowedValues)
	} else if variant.Type != "" {
		builder.WriteString(`Type: `)
		builder.WriteString("`")
		builder.WriteString(variant.Type)
		builder.WriteString("`")
		builder.WriteString("\n\n")
	}

	builder.WriteString(`</Tab>` + "\n\n")
}

func writeResponseField(builder *strings.Builder, field ResponseField) {
	builder.WriteString(`<ResponseField name="`)
	builder.WriteString(escapeMDXAttr(field.Name))
	builder.WriteString(`" type="`)
	builder.WriteString(escapeMDXAttr(field.Type))
	builder.WriteString(`"`)

	if field.Required {
		builder.WriteString(` required`)
	}

	builder.WriteString(`>` + "\n\n")

	if description := strings.TrimSpace(field.Description); description != "" {
		builder.WriteString(description)
		builder.WriteString("\n\n")
	}

	if len(field.Variants) > 0 {
		writeResponseVariants(builder, field.Variants)
	}

	if len(field.AllowedValues) > 0 {
		writeAllowedValues(builder, field.AllowedValues)
	}

	if len(field.Children) > 0 {
		builder.WriteString(`<Expandable title="properties">` + "\n\n")

		for _, child := range field.Children {
			writeResponseField(builder, child)
		}

		builder.WriteString(`</Expandable>` + "\n\n")
	}

	builder.WriteString(`</ResponseField>` + "\n\n")
}

func writeResponseVariants(builder *strings.Builder, variants []ResponseVariant) {
	builder.WriteString(`<Tabs sync={false}>` + "\n\n")

	for _, variant := range variants {
		writeResponseVariant(builder, variant)
	}

	builder.WriteString(`</Tabs>` + "\n\n")
}

func writeResponseVariant(builder *strings.Builder, variant ResponseVariant) {
	builder.WriteString(`<Tab title="`)
	builder.WriteString(escapeMDXAttr(variant.Title))
	builder.WriteString(`">` + "\n\n")

	if description := strings.TrimSpace(variant.Description); description != "" {
		builder.WriteString(description)
		builder.WriteString("\n\n")
	}

	if len(variant.Children) > 0 {
		for _, child := range variant.Children {
			writeResponseField(builder, child)
		}
	} else if len(variant.AllowedValues) > 0 {
		writeAllowedValues(builder, variant.AllowedValues)
	} else if variant.Type != "" {
		builder.WriteString(`Type: `)
		builder.WriteString("`")
		builder.WriteString(variant.Type)
		builder.WriteString("`")
		builder.WriteString("\n\n")
	}

	builder.WriteString(`</Tab>` + "\n\n")
}

func writeAllowedValues(builder *strings.Builder, values []string) {
	builder.WriteString(`<Expandable title="Allowed values">` + "\n\n")

	for _, value := range values {
		builder.WriteString("- `")
		builder.WriteString(value)
		builder.WriteString("`\n")
	}

	builder.WriteString("\n</Expandable>\n\n")
}

func enterSchemaRef(seen map[string]bool, proxy *base.SchemaProxy) (string, bool) {
	ref := proxy.GetReference()
	if ref == "" {
		return "", false
	}

	if seen[ref] {
		return ref, true
	}

	seen[ref] = true

	return ref, false
}

func leaveSchemaRef(seen map[string]bool, ref string) {
	if ref == "" {
		return
	}

	delete(seen, ref)
}

func fallbackTypeSummary(ref string) string {
	if ref != "" {
		return refName(ref)
	}

	return "object"
}

func polymorphicTypeSummary(schema *base.Schema, seen map[string]bool) string {
	if summary := compositeTypeSummary(schema.OneOf, " | ", seen); summary != "" {
		return summary
	}

	if summary := compositeTypeSummary(schema.AnyOf, " | ", seen); summary != "" {
		return summary
	}

	if summary := allOfTypeSummary(schema.AllOf, seen); summary != "" {
		return summary
	}

	return ""
}

func allOfTypeSummary(proxies []*base.SchemaProxy, seen map[string]bool) string {
	if len(proxies) == 0 {
		return ""
	}

	if allOfObjectLike(proxies) {
		return "object"
	}

	return compositeTypeSummary(proxies, " & ", seen)
}

func allOfObjectLike(proxies []*base.SchemaProxy) bool {
	if len(proxies) == 0 {
		return false
	}

	for _, proxy := range proxies {
		if !schemaProxyIsObjectLike(proxy) {
			return false
		}
	}

	return true
}

func arrayTypeSummary(schema *base.Schema, seen map[string]bool) string {
	if !schemaHasType(schema, "array") || schema.Items == nil || !schema.Items.IsA() ||
		schema.Items.A == nil {
		return ""
	}

	return fmt.Sprintf("array<%s>", schemaProxyTypeSummary(schema.Items.A, seen))
}

func scalarTypeSummary(schema *base.Schema) string {
	if summary := enumTypeSummary(schema); summary != "" {
		return summary
	}

	if len(schema.Type) == 0 {
		return ""
	}

	return strings.Join(schema.Type, " | ")
}

func enumTypeSummary(schema *base.Schema) string {
	if schema == nil || len(schema.Enum) == 0 || len(schema.Enum) > 8 {
		return ""
	}

	parts := make([]string, 0, len(schema.Enum))
	seenLiterals := map[string]struct{}{}

	for _, node := range schema.Enum {
		literal := enumNodeLiteral(node)
		if literal == "" {
			return ""
		}

		if _, ok := seenLiterals[literal]; ok {
			continue
		}

		seenLiterals[literal] = struct{}{}
		parts = append(parts, literal)
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " | ")
}

func largeEnumValues(schema *base.Schema) []string {
	if schema == nil || len(schema.Enum) <= 8 {
		return nil
	}

	values := make([]string, 0, len(schema.Enum))
	seenLiterals := map[string]struct{}{}

	for _, node := range schema.Enum {
		literal := enumNodeLiteral(node)
		if literal == "" {
			return nil
		}

		if _, ok := seenLiterals[literal]; ok {
			continue
		}

		seenLiterals[literal] = struct{}{}
		values = append(values, literal)
	}

	if len(values) == 0 {
		return nil
	}

	return values
}

func compareTypeSummary(left, right string) int {
	leftRank := typeSummaryRank(left)

	rightRank := typeSummaryRank(right)
	if leftRank != rightRank {
		return leftRank - rightRank
	}

	if left < right {
		return -1
	}

	if left > right {
		return 1
	}

	return 0
}

func typeSummaryRank(value string) int {
	if value == "null" {
		return 3
	}

	if strings.HasPrefix(value, "'") {
		return 2
	}

	if strings.HasPrefix(value, "array<") {
		return -1
	}

	return 0
}

func enumNodeLiteral(node *yaml.Node) string {
	if node == nil {
		return ""
	}

	switch node.Tag {
	case "!!str":
		return singleQuotedLiteral(node.Value)
	case "!!int", "!!float", "!!bool":
		return node.Value
	case "!!null":
		return "null"
	default:
		if node.Kind == yaml.ScalarNode {
			return singleQuotedLiteral(node.Value)
		}

		return ""
	}
}

func singleQuotedLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
}

func escapeMDXAttr(value string) string {
	return html.EscapeString(value)
}

func objectOrNamedTypeSummary(schema *base.Schema, ref string) string {
	if schema.Properties != nil || schema.AdditionalProperties != nil {
		return "object"
	}

	if ref != "" {
		return refName(ref)
	}

	if title := strings.TrimSpace(schema.Title); title != "" {
		return title
	}

	return "object"
}

func schemaProxyVariants(proxy *base.SchemaProxy, seen map[string]bool) []ParameterVariant {
	if proxy == nil {
		return nil
	}

	ref, alreadySeen := enterSchemaRef(seen, proxy)
	if alreadySeen {
		return nil
	}

	defer leaveSchemaRef(seen, ref)

	schema, err := proxy.BuildSchema()
	if err != nil || schema == nil {
		return nil
	}

	if variants := buildRenderableSchemaVariants(schema.OneOf, seen); len(variants) > 0 {
		return variants
	}

	if variants := buildRenderableSchemaVariants(schema.AnyOf, seen); len(variants) > 0 {
		return variants
	}

	return nil
}

func buildRenderableSchemaVariants(
	proxies []*base.SchemaProxy,
	seen map[string]bool,
) []ParameterVariant {
	nonNullProxies, hasNull := splitNullSchemaVariants(proxies)
	if len(nonNullProxies) == 0 {
		return nil
	}

	if hasNull && len(nonNullProxies) == 1 {
		return nil
	}

	if hasRecursiveSchemaVariant(nonNullProxies, seen) {
		return nil
	}

	if !hasStructuralSchemaVariant(nonNullProxies, seen) {
		return nil
	}

	return buildSchemaVariants(nonNullProxies, seen)
}

func hasRecursiveSchemaVariant(proxies []*base.SchemaProxy, seen map[string]bool) bool {
	for _, proxy := range proxies {
		if schemaProxyIsDirectRecursiveVariant(proxy, cloneSeenRefs(seen)) {
			return true
		}
	}

	return false
}

func schemaProxyIsDirectRecursiveVariant(proxy *base.SchemaProxy, seen map[string]bool) bool {
	if proxy == nil {
		return false
	}

	ref := proxy.GetReference()
	if ref != "" {
		return seen[ref]
	}

	schema, err := proxy.BuildSchema()
	if err != nil || schema == nil {
		return false
	}

	if schema.Items != nil && schema.Items.IsA() && schema.Items.A != nil {
		itemRef := schema.Items.A.GetReference()
		if itemRef != "" && seen[itemRef] {
			return true
		}
	}

	return false
}

func hasStructuralSchemaVariant(proxies []*base.SchemaProxy, seen map[string]bool) bool {
	for _, proxy := range proxies {
		if schemaProxyIsObjectLike(proxy) {
			return true
		}

		if len(schemaProxyChildren(proxy, cloneSeenRefs(seen))) > 0 {
			return true
		}
	}

	return false
}

func schemaProxyIsObjectLike(proxy *base.SchemaProxy) bool {
	if proxy == nil {
		return false
	}

	schema, err := proxy.BuildSchema()
	if err != nil || schema == nil {
		return false
	}

	return schemaIsObjectLike(schema)
}

func buildSchemaVariants(proxies []*base.SchemaProxy, seen map[string]bool) []ParameterVariant {
	if len(proxies) == 0 {
		return nil
	}

	variants := make([]ParameterVariant, 0, len(proxies))
	usedTitles := map[string]int{}

	for index, proxy := range proxies {
		variant := buildParameterVariant(proxy, cloneSeenRefs(seen), index)
		variant.Title = uniqueVariantTitle(variant.Title, usedTitles)
		variants = append(variants, variant)
	}

	return variants
}

func buildParameterVariant(
	proxy *base.SchemaProxy,
	seen map[string]bool,
	index int,
) ParameterVariant {
	title := parameterVariantTitle(proxy, cloneSeenRefs(seen), index)
	description := strings.TrimSpace(schemaDescription(proxy))
	children := schemaProxyChildren(proxy, cloneSeenRefs(seen))
	typeSummary := schemaProxyTypeSummary(proxy, cloneSeenRefs(seen))

	return ParameterVariant{
		Title:       title,
		Description: description,
		Type:        typeSummary,
		Children:    children,
	}
}

func parameterVariantTitle(proxy *base.SchemaProxy, seen map[string]bool, index int) string {
	if proxy == nil {
		return fmt.Sprintf("Variant %d", index+1)
	}

	schema, err := proxy.BuildSchema()
	if err == nil && schema != nil {
		if label := variantLabelFromDescription(schemaDescription(proxy), schema); label != "" {
			return label
		}

		if label := variantLabelFromSchemaName(proxy, schema); label != "" {
			return label
		}

		if label := variantLabelFromSchemaType(schema, seen); label != "" {
			return label
		}
	}

	if summary := strings.TrimSpace(schemaProxyTypeSummary(proxy, seen)); summary != "" {
		return humanizeVariantLabel(summary)
	}

	return fmt.Sprintf("Variant %d", index+1)
}

func variantLabelFromDescription(description string, schema *base.Schema) string {
	description = strings.TrimSpace(description)
	if description == "" || schema == nil {
		return ""
	}

	lower := strings.ToLower(description)
	if strings.Contains(lower, "query string") {
		return "Query string"
	}

	return ""
}

func variantLabelFromSchemaName(proxy *base.SchemaProxy, schema *base.Schema) string {
	if schema != nil {
		if title := strings.TrimSpace(schema.Title); title != "" {
			if label := normalizedVariantLabel(title, schema); label != "" {
				return label
			}
		}
	}

	if proxy != nil {
		if ref := strings.TrimSpace(proxy.GetReference()); ref != "" {
			if label := normalizedVariantLabel(refName(ref), schema); label != "" {
				return label
			}
		}
	}

	return ""
}

func normalizedVariantLabel(name string, schema *base.Schema) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	humanized := humanizeVariantLabel(name)
	if humanized == "" || schema == nil {
		return humanized
	}

	if schemaIsObjectLike(schema) && strings.HasSuffix(humanized, " Object") {
		return "Object"
	}

	if schemaHasType(schema, "string") && strings.HasSuffix(humanized, " String") {
		prefix := strings.TrimSpace(strings.TrimSuffix(humanized, " String"))
		if prefix == "" {
			return "String"
		}

		return prefix + " string"
	}

	return humanized
}

func variantLabelFromSchemaType(schema *base.Schema, seen map[string]bool) string {
	if schema == nil {
		return ""
	}

	if schemaHasType(schema, "array") {
		if schema.Items != nil && schema.Items.IsA() && schema.Items.A != nil {
			itemRef := strings.TrimSpace(schema.Items.A.GetReference())
			if itemRef != "" {
				return "Nested list"
			}
		}

		return "List"
	}

	if schemaIsObjectLike(schema) {
		return "Object"
	}

	if len(schema.Type) == 1 {
		return humanizeVariantLabel(schema.Type[0])
	}

	if summary := strings.TrimSpace(scalarTypeSummary(schema)); summary != "" {
		return humanizeVariantLabel(summary)
	}

	return ""
}

func humanizeVariantLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	return strings.Join(titleizeVariantWords(tokenizeVariantLabel(value)), " ")
}

func tokenizeVariantLabel(value string) []string {
	var builder strings.Builder

	var prev rune

	for i, r := range value {
		if r == '_' || r == '-' {
			builder.WriteRune(' ')

			prev = r

			continue
		}

		if i > 0 && shouldInsertSpace(prev, r) {
			builder.WriteRune(' ')
		}

		builder.WriteRune(r)
		prev = r
	}

	return strings.Fields(builder.String())
}

func titleizeVariantWords(words []string) []string {
	for i, word := range words {
		if word == "ID" || word == "IDs" || strings.Contains(word, "<") ||
			strings.Contains(word, "|") {
			continue
		}

		runes := []rune(strings.ToLower(word))
		if len(runes) == 0 {
			continue
		}

		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}

	return words
}

func shouldInsertSpace(prev, curr rune) bool {
	if prev == 0 || prev == ' ' {
		return false
	}

	if unicode.IsLower(prev) && unicode.IsUpper(curr) {
		return true
	}

	if unicode.IsLetter(prev) && unicode.IsDigit(curr) {
		return true
	}

	if unicode.IsDigit(prev) && unicode.IsLetter(curr) {
		return true
	}

	return false
}

func schemaIsObjectLike(schema *base.Schema) bool {
	if schema == nil {
		return false
	}

	if schemaHasType(schema, "object") || schema.Properties != nil || len(schema.AllOf) > 0 {
		return true
	}

	return false
}

func uniqueVariantTitle(title string, used map[string]int) string {
	count := used[title]

	used[title] = count + 1

	if count == 0 {
		return title
	}

	return fmt.Sprintf("%s %d", title, count+1)
}

func schemaProxyChildren(proxy *base.SchemaProxy, seen map[string]bool) []Parameter {
	if proxy == nil {
		return nil
	}

	ref, alreadySeen := enterSchemaRef(seen, proxy)
	if alreadySeen {
		return nil
	}

	defer leaveSchemaRef(seen, ref)

	schema, err := proxy.BuildSchema()
	if err != nil || schema == nil {
		return nil
	}

	if nullableProxy, ok := nullableSingleSchemaVariant(schema); ok {
		return schemaProxyChildren(nullableProxy, seen)
	}

	if children := schemaObjectChildren(schema, seen); len(children) > 0 {
		return children
	}

	if children := arrayChildren(schema, seen); len(children) > 0 {
		return children
	}

	return nil
}

func nullableSingleSchemaVariant(schema *base.Schema) (*base.SchemaProxy, bool) {
	nonNullProxies, hasNull := splitNullSchemaVariants(schema.OneOf)
	if hasNull && len(nonNullProxies) == 1 {
		return nonNullProxies[0], true
	}

	nonNullProxies, hasNull = splitNullSchemaVariants(schema.AnyOf)
	if hasNull && len(nonNullProxies) == 1 {
		return nonNullProxies[0], true
	}

	return nil, false
}

func splitNullSchemaVariants(proxies []*base.SchemaProxy) ([]*base.SchemaProxy, bool) {
	if len(proxies) == 0 {
		return nil, false
	}

	nonNull := make([]*base.SchemaProxy, 0, len(proxies))
	hasNull := false

	for _, proxy := range proxies {
		if isNullSchemaProxy(proxy) {
			hasNull = true

			continue
		}

		nonNull = append(nonNull, proxy)
	}

	return nonNull, hasNull
}

func isNullSchemaProxy(proxy *base.SchemaProxy) bool {
	if proxy == nil {
		return false
	}

	schema, err := proxy.BuildSchema()
	if err != nil || schema == nil {
		return false
	}

	return len(schema.Type) == 1 && schema.Type[0] == "null"
}

func schemaProxyContainsRecursiveRef(proxy *base.SchemaProxy, seen map[string]bool) bool {
	if proxy == nil {
		return false
	}

	ref, recursive := enterRecursiveRef(seen, proxy)
	if recursive {
		return true
	}

	defer leaveSchemaRef(seen, ref)

	schema, err := proxy.BuildSchema()
	if err != nil || schema == nil {
		return false
	}

	if schemaContainsRecursiveRef(schema, seen) {
		return true
	}

	return false
}

func enterRecursiveRef(seen map[string]bool, proxy *base.SchemaProxy) (string, bool) {
	ref := proxy.GetReference()
	if ref == "" {
		return "", false
	}

	if seen[ref] {
		return ref, true
	}

	seen[ref] = true

	return ref, false
}

func schemaContainsRecursiveRef(schema *base.Schema, seen map[string]bool) bool {
	if proxiesContainRecursiveRef(schema.OneOf, seen) ||
		proxiesContainRecursiveRef(schema.AnyOf, seen) ||
		proxiesContainRecursiveRef(schema.AllOf, seen) {
		return true
	}

	if arrayContainsRecursiveRef(schema, seen) {
		return true
	}

	if objectContainsRecursiveRef(schema, seen) {
		return true
	}

	return false
}

func arrayContainsRecursiveRef(schema *base.Schema, seen map[string]bool) bool {
	if schema.Items == nil || !schema.Items.IsA() || schema.Items.A == nil {
		return false
	}

	return schemaProxyContainsRecursiveRef(schema.Items.A, cloneSeenRefs(seen))
}

func objectContainsRecursiveRef(schema *base.Schema, seen map[string]bool) bool {
	if schema.Properties == nil {
		return false
	}

	for prop := schema.Properties.First(); prop != nil; prop = prop.Next() {
		if schemaProxyContainsRecursiveRef(prop.Value(), cloneSeenRefs(seen)) {
			return true
		}
	}

	return false
}

func proxiesContainRecursiveRef(proxies []*base.SchemaProxy, seen map[string]bool) bool {
	for _, proxy := range proxies {
		if schemaProxyContainsRecursiveRef(proxy, cloneSeenRefs(seen)) {
			return true
		}
	}

	return false
}

func schemaObjectChildren(schema *base.Schema, seen map[string]bool) []Parameter {
	if children := objectChildren(schema, seen); len(children) > 0 {
		return children
	}

	if children := allOfChildren(schema, seen); len(children) > 0 {
		return children
	}

	return nil
}

func objectChildren(schema *base.Schema, seen map[string]bool) []Parameter {
	if schema.Properties == nil {
		return nil
	}

	return getObjectParametersWithSeen(schema.Properties, schema.Required, seen)
}

func allOfChildren(schema *base.Schema, seen map[string]bool) []Parameter {
	if len(schema.AllOf) == 0 {
		return nil
	}

	var result []Parameter

	paramIndexes := map[string]int{}

	for _, proxy := range schema.AllOf {
		params := schemaProxyAllOfParameters(proxy, cloneSeenRefs(seen))
		mergeParameters(&result, paramIndexes, params)
	}

	return result
}

func schemaProxyAllOfParameters(proxy *base.SchemaProxy, seen map[string]bool) []Parameter {
	if proxy == nil {
		return nil
	}

	ref, alreadySeen := enterSchemaRef(seen, proxy)
	if alreadySeen {
		return nil
	}

	defer leaveSchemaRef(seen, ref)

	schema, err := proxy.BuildSchema()
	if err != nil || schema == nil {
		return nil
	}

	return schemaObjectChildren(schema, seen)
}

func mergeParameters(result *[]Parameter, indexes map[string]int, incoming []Parameter) {
	for _, param := range incoming {
		idx, ok := indexes[param.Name]
		if !ok {
			indexes[param.Name] = len(*result)
			*result = append(*result, param)

			continue
		}

		merged := mergeParameter((*result)[idx], param)
		(*result)[idx] = merged
	}
}

func mergeParameter(existing, incoming Parameter) Parameter {
	if strings.TrimSpace(existing.Description) == "" {
		existing.Description = incoming.Description
	}

	existing.Required = existing.Required || incoming.Required

	if existing.Type == "" || existing.Type == "object" {
		if incoming.Type != "" {
			existing.Type = incoming.Type
		}
	}

	childIndexes := make(map[string]int, len(existing.Children))
	for idx, child := range existing.Children {
		childIndexes[child.Name] = idx
	}

	mergeParameters(&existing.Children, childIndexes, incoming.Children)

	return existing
}

func arrayChildren(schema *base.Schema, seen map[string]bool) []Parameter {
	if !schemaHasType(schema, "array") || schema.Items == nil || !schema.Items.IsA() ||
		schema.Items.A == nil {
		return nil
	}

	return schemaProxyChildren(schema.Items.A, seen)
}

func cloneSeenRefs(seen map[string]bool) map[string]bool {
	cloned := make(map[string]bool, len(seen))
	for key, value := range seen {
		cloned[key] = value
	}

	return cloned
}

func boolOrFalse(val *bool) bool {
	if val == nil {
		return false
	} else {
		return *val
	}
}
