package clients

import (
	"html"
	"sort"
	"strings"
)

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
	if param.Required || param.Deprecated || strings.TrimSpace(param.Description) != "" ||
		len(param.Children) > 0 {
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
	if strings.TrimSpace(variant.Description) != "" || len(variant.Children) > 0 {
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
		Deprecated:    param.Deprecated,
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
		result = append(
			result,
			ResponseVariant{
				Title:         variant.Title,
				Description:   variant.Description,
				Type:          variant.Type,
				Children:      responseFieldsFromParameters(variant.Children),
				AllowedValues: append([]string(nil), variant.AllowedValues...),
			},
		)
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
	if field.Required || field.Deprecated || strings.TrimSpace(field.Description) != "" ||
		len(field.Children) > 0 ||
		len(field.AllowedValues) > 0 {
		return true
	}

	return len(field.Variants) > 0
}

func shouldRenderResponseVariant(variant ResponseVariant) bool {
	if strings.TrimSpace(variant.Description) != "" || len(variant.Children) > 0 ||
		len(variant.AllowedValues) > 0 {
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

func sortOperationResponses(responses []OperationResponse) []OperationResponse {
	sort.SliceStable(responses, func(i, j int) bool {
		if responses[i].SortPriority != responses[j].SortPriority {
			return responses[i].SortPriority < responses[j].SortPriority
		}

		return responses[i].StatusCode < responses[j].StatusCode
	})

	return responses
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

func renderResponses(responses []OperationResponse) string {
	var builder strings.Builder
	builder.WriteString(`<Tabs sync={false}>` + "\n\n")

	for _, response := range responses {
		writeOperationResponse(&builder, response)
	}

	builder.WriteString(`</Tabs>` + "\n")

	return strings.TrimRight(builder.String(), "\n")
}

func writeOperationResponse(builder *strings.Builder, response OperationResponse) {
	builder.WriteString(`<Tab title="`)
	builder.WriteString(escapeMDXAttr(response.StatusCode))
	builder.WriteString(`">` + "\n\n")

	if description := strings.TrimSpace(response.Description); description != "" {
		builder.WriteString(description)
		builder.WriteString("\n\n")
	}

	for _, field := range response.Fields {
		writeResponseField(builder, field)
	}

	builder.WriteString(`</Tab>` + "\n\n")
}

func writeParamField(builder *strings.Builder, param Parameter) {
	builder.WriteString(`<ParamField path="`)
	builder.WriteString(escapeMDXAttr(param.Name))
	builder.WriteString(`" type="`)
	builder.WriteString(escapeMDXAttr(mintFieldType(param.Type)))
	builder.WriteString(`"`)

	if param.Deprecated {
		builder.WriteString(` deprecated`)
	}

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
		builder.WriteString(`Type: ` + "`" + variant.Type + "`\n\n")
	}

	builder.WriteString(`</Tab>` + "\n\n")
}

func writeResponseField(builder *strings.Builder, field ResponseField) {
	builder.WriteString(`<ResponseField name="`)
	builder.WriteString(escapeMDXAttr(field.Name))
	builder.WriteString(`" type="`)
	builder.WriteString(escapeMDXAttr(field.Type))
	builder.WriteString(`"`)

	if field.Deprecated {
		builder.WriteString(` deprecated`)
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
		builder.WriteString(`Type: ` + "`" + variant.Type + "`\n\n")
	}

	builder.WriteString(`</Tab>` + "\n\n")
}

func writeAllowedValues(builder *strings.Builder, values []string) {
	builder.WriteString(`<Expandable title="Allowed values">` + "\n\n")

	for _, value := range values {
		builder.WriteString("- `" + value + "`\n")
	}

	builder.WriteString("\n</Expandable>\n\n")
}

func escapeMDXAttr(value string) string {
	return html.EscapeString(value)
}
