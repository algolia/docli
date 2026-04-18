package clients

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	base "github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"go.yaml.in/yaml/v4"
)

func normalizeVariantParentType(current string, variants []ParameterVariant) string {
	if normalized, ok := normalizeArrayObjectType(current); ok {
		return normalized
	}

	if strings.HasPrefix(current, "array<") {
		return current
	}

	if strings.Count(current, "object") >= 2 {
		return "object"
	}

	if !variantsAllHaveChildren(variants) {
		return current
	}

	return "object"
}

func normalizeArrayObjectType(current string) (string, bool) {
	if !strings.HasPrefix(current, "array<") || !strings.HasSuffix(current, ">") {
		return "", false
	}

	inner := strings.TrimSuffix(strings.TrimPrefix(current, "array<"), ">")
	if !strings.Contains(inner, "object") {
		return "", false
	}

	if strings.Contains(inner, "&") {
		return "array<object>", true
	}

	for _, scalar := range []string{"string", "number", "integer", "boolean", "null", "'"} {
		if strings.Contains(inner, scalar) {
			return "", false
		}
	}

	return "array<object>", true
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

func schemaProxyDeprecated(proxy *base.SchemaProxy) bool {
	if proxy == nil {
		return false
	}

	schema, err := proxy.BuildSchema()
	if err != nil || schema == nil {
		return false
	}

	return boolOrFalse(schema.Deprecated)
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

	mediaType := getRequestMediaType(p.Content)
	if mediaType == nil {
		return nil
	}

	return mediaType.Schema
}

func getRequestMediaType(content *orderedmap.Map[string, *v3.MediaType]) *v3.MediaType {
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

func getResponseMediaType(content *orderedmap.Map[string, *v3.MediaType]) *v3.MediaType {
	if content == nil {
		return nil
	}

	if mediaType, ok := content.Get("application/json"); ok {
		return mediaType
	}

	for pair := content.First(); pair != nil; pair = pair.Next() {
		key := strings.TrimSpace(strings.ToLower(pair.Key()))
		if strings.HasSuffix(key, "/json") || strings.HasSuffix(key, "+json") {
			return pair.Value()
		}
	}

	for pair := content.First(); pair != nil; pair = pair.Next() {
		if pair.Value() != nil && pair.Value().Schema != nil {
			return pair.Value()
		}
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

func hasExplicitRequestBodyName(op *v3.Operation, schema *base.Schema) bool {
	if op.Extensions != nil {
		if node, ok := op.Extensions.Get("x-codegen-request-body-name"); ok {
			if strings.TrimSpace(node.Value) != "" {
				return true
			}
		}
	}

	return schema != nil && strings.TrimSpace(schema.Title) != ""
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

	sort.Slice(parts, func(i, j int) bool { return compareTypeSummary(parts[i], parts[j]) < 0 })

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
	if schema.Items == nil || !schema.Items.IsA() || schema.Items.A == nil {
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

	if variants := arrayItemVariants(schema, seen); len(variants) > 0 {
		return variants
	}

	return nil
}

func arrayItemVariants(schema *base.Schema, seen map[string]bool) []ParameterVariant {
	if schema == nil || schema.Items == nil || !schema.Items.IsA() || schema.Items.A == nil {
		return nil
	}

	proxy := schema.Items.A
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
	renderableProxies, hasNull := renderableSchemaVariantProxies(proxies, seen)
	if len(renderableProxies) == 0 {
		return nil
	}

	if hasNull && len(renderableProxies) == 1 {
		return nil
	}

	return buildSchemaVariants(renderableProxies, seen)
}

func renderableSchemaVariantProxies(
	proxies []*base.SchemaProxy,
	seen map[string]bool,
) ([]*base.SchemaProxy, bool) {
	nonNullProxies, hasNull := splitNullSchemaVariants(proxies)
	if len(nonNullProxies) == 0 {
		return nil, false
	}

	if hasRecursiveSchemaVariant(nonNullProxies, seen) {
		return nil, false
	}

	if !hasStructuralSchemaVariant(nonNullProxies, seen) {
		return nil, false
	}

	return nonNullProxies, hasNull
}

func hasRecursiveSchemaVariant(proxies []*base.SchemaProxy, seen map[string]bool) bool {
	if len(proxies) == 0 {
		return false
	}

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
		expanded := expandParameterVariants(proxy, cloneSeenRefs(seen), index)
		for _, variant := range expanded {
			variant.Title = uniqueVariantTitle(variant.Title, usedTitles)
			variants = append(variants, variant)
		}
	}

	return variants
}

func expandParameterVariants(
	proxy *base.SchemaProxy,
	seen map[string]bool,
	index int,
) []ParameterVariant {
	if variants := buildAllOfParameterVariants(
		proxy,
		cloneSeenRefs(seen),
		index,
	); len(
		variants,
	) > 0 {
		return variants
	}

	return []ParameterVariant{buildParameterVariant(proxy, cloneSeenRefs(seen), index)}
}

func buildAllOfParameterVariants(
	proxy *base.SchemaProxy,
	seen map[string]bool,
	index int,
) []ParameterVariant {
	if proxy == nil {
		return nil
	}

	ref, alreadySeen := enterSchemaRef(seen, proxy)
	if alreadySeen {
		return nil
	}

	defer leaveSchemaRef(seen, ref)

	schema, err := proxy.BuildSchema()
	if err != nil || schema == nil || len(schema.AllOf) == 0 {
		return nil
	}

	parentTitle := parameterVariantTitle(proxy, cloneSeenRefs(seen), index)
	variantOperandIndex := -1
	var variantProxies []*base.SchemaProxy

	for operandIndex, operand := range schema.AllOf {
		if operand == nil {
			continue
		}

		operandSchema, err := operand.BuildSchema()
		if err != nil || operandSchema == nil {
			continue
		}

		proxies, _ := renderableSchemaVariantProxies(operandSchema.OneOf, cloneSeenRefs(seen))
		if len(proxies) == 0 {
			proxies, _ = renderableSchemaVariantProxies(operandSchema.AnyOf, cloneSeenRefs(seen))
		}

		if len(proxies) == 0 {
			continue
		}

		if variantOperandIndex != -1 {
			return nil
		}

		variantOperandIndex = operandIndex
		variantProxies = proxies
	}

	if variantOperandIndex == -1 {
		return nil
	}

	sharedChildren := allOfSharedChildren(schema.AllOf, variantOperandIndex, seen)
	variants := make([]ParameterVariant, 0, len(variantProxies))

	for variantIndex, variantProxy := range variantProxies {
		variant := buildParameterVariant(variantProxy, cloneSeenRefs(seen), variantIndex)
		variant.Title = combineVariantTitles(parentTitle, variant.Title)
		variant.Children = mergeVariantChildren(variant.Children, sharedChildren)
		variants = append(variants, variant)
	}

	return variants
}

func allOfSharedChildren(
	operands []*base.SchemaProxy,
	skipIndex int,
	seen map[string]bool,
) []Parameter {
	var result []Parameter
	indexes := map[string]int{}

	for operandIndex, operand := range operands {
		if operandIndex == skipIndex {
			continue
		}

		params := schemaProxyAllOfParameters(operand, cloneSeenRefs(seen))
		mergeParameters(&result, indexes, params)
	}

	return result
}

func mergeVariantChildren(existing, shared []Parameter) []Parameter {
	if len(shared) == 0 {
		return existing
	}

	result := append([]Parameter(nil), existing...)
	indexes := make(map[string]int, len(result))
	for idx, child := range result {
		indexes[child.Name] = idx
	}

	mergeParameters(&result, indexes, shared)

	return result
}

func combineVariantTitles(parent, child string) string {
	parent = strings.TrimSpace(parent)
	child = strings.TrimSpace(child)

	if parent == "" {
		return child
	}

	if child == "" || child == parent {
		return parent
	}

	return parent + " " + child
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

	if label := normalizedObjectVariantLabel(humanized, schema); label != "" {
		return label
	}

	if label := normalizedStringVariantLabel(humanized, schema); label != "" {
		return label
	}

	if label := normalizedArrayVariantLabel(humanized, schema); label != "" {
		return label
	}

	return humanized
}

func normalizedObjectVariantLabel(humanized string, schema *base.Schema) string {
	if !schemaIsObjectLike(schema) || !strings.HasSuffix(humanized, " Object") {
		return ""
	}

	return "Object"
}

func normalizedStringVariantLabel(humanized string, schema *base.Schema) string {
	if !schemaHasType(schema, "string") || !strings.HasSuffix(humanized, " String") {
		return ""
	}

	prefix := strings.TrimSpace(strings.TrimSuffix(humanized, " String"))
	if prefix == "" {
		return "String"
	}

	return prefix + " string"
}

func normalizedArrayVariantLabel(humanized string, schema *base.Schema) string {
	if !schemaHasType(schema, "array") || !strings.HasSuffix(humanized, " List") {
		return ""
	}

	prefix := strings.TrimSpace(strings.TrimSuffix(humanized, " List"))
	if prefix == "" {
		return "List"
	}

	return prefix + " list"
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

	return schemaContainsRecursiveRef(schema, seen)
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
	return proxiesContainRecursiveRef(schema.OneOf, seen) ||
		proxiesContainRecursiveRef(schema.AnyOf, seen) ||
		proxiesContainRecursiveRef(schema.AllOf, seen) ||
		arrayContainsRecursiveRef(schema, seen) ||
		objectContainsRecursiveRef(schema, seen)
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

	if children := schemaObjectChildren(schema, seen); len(children) > 0 {
		return children
	}

	if children := singleRenderableObjectVariantChildren(schema, seen); len(children) > 0 {
		return children
	}

	return nil
}

func singleRenderableObjectVariantChildren(schema *base.Schema, seen map[string]bool) []Parameter {
	if schema == nil {
		return nil
	}

	if children := singleRenderableObjectVariantChildrenFromProxies(
		schema.OneOf,
		seen,
	); len(
		children,
	) > 0 {
		return children
	}

	if children := singleRenderableObjectVariantChildrenFromProxies(
		schema.AnyOf,
		seen,
	); len(
		children,
	) > 0 {
		return children
	}

	return nil
}

func singleRenderableObjectVariantChildrenFromProxies(
	proxies []*base.SchemaProxy,
	seen map[string]bool,
) []Parameter {
	if len(proxies) == 0 {
		return nil
	}

	nonNullProxies, _ := splitNullSchemaVariants(proxies)
	objectLike := make([]*base.SchemaProxy, 0, len(nonNullProxies))

	for _, proxy := range nonNullProxies {
		if !schemaProxyIsObjectLike(proxy) {
			continue
		}

		objectLike = append(objectLike, proxy)
	}

	if len(objectLike) != 1 {
		return nil
	}

	return schemaProxyChildren(objectLike[0], seen)
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
	existing.Variants = mergeParameterVariants(existing.Variants, incoming.Variants)
	existing.AllowedValues = mergeAllowedValues(existing.AllowedValues, incoming.AllowedValues)

	return existing
}

func mergeParameterVariants(existing, incoming []ParameterVariant) []ParameterVariant {
	if len(existing) == 0 {
		return append([]ParameterVariant(nil), incoming...)
	}

	if len(incoming) == 0 {
		return existing
	}

	result := append([]ParameterVariant(nil), existing...)
	seen := make(map[string]struct{}, len(result))

	for _, variant := range result {
		seen[parameterVariantKey(variant)] = struct{}{}
	}

	for _, variant := range incoming {
		key := parameterVariantKey(variant)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}

		result = append(result, variant)
	}

	return result
}

func parameterVariantKey(variant ParameterVariant) string {
	return strings.Join([]string{variant.Title, variant.Description, variant.Type}, "\x00")
}

func mergeAllowedValues(existing, incoming []string) []string {
	if len(existing) == 0 {
		return append([]string(nil), incoming...)
	}

	if len(incoming) == 0 {
		return existing
	}

	result := append([]string(nil), existing...)
	seen := make(map[string]struct{}, len(result))

	for _, value := range result {
		seen[value] = struct{}{}
	}

	for _, value := range incoming {
		if _, ok := seen[value]; ok {
			continue
		}

		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}

func arrayChildren(schema *base.Schema, seen map[string]bool) []Parameter {
	if schema.Items == nil || !schema.Items.IsA() || schema.Items.A == nil {
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
