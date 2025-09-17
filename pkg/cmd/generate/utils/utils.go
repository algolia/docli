package utils

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/algolia/docli/pkg/dictionary"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"go.yaml.in/yaml/v4"
)

// GetApiName returns the name of the YAML file without extension as API name.
func GetApiName(path string) string {
	// Have to make an exception for the Analytics API
	base := filepath.Base(strings.ReplaceAll(path, "searchstats", "analytics"))

	return strings.TrimSuffix(base, filepath.Ext(base))
}

// GetAcl returns the ACL required to perform the given operation.
func GetAcl(op *v3.Operation) ([]string, error) {
	node, ok := op.Extensions.Get("x-acl")
	// Operations can be without ACL
	if !ok {
		return nil, nil
	}

	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("expected a sequence node, got kind %d", node.Kind)
	}

	var result []string

	for _, child := range node.Content {
		if child.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("expected scalar nodes in sequence, got kind %d", child.Kind)
		}

		result = append(result, child.Value)
	}

	return result, nil
}

// AclToString returns a comma-separated string of ACL with backticks.
func AclToString(acl []string) string {
	backticked := make([]string, len(acl))
	for i := range acl {
		backticked[i] = fmt.Sprintf("`%s`", acl[i])
	}

	return strings.Join(backticked, ", ")
}

// ToKebabCase turns a string into kebab-case.
func ToKebabCase(s string) string {
	matchFirstCap := regexp.MustCompile(`(.)([A-Z][a-z]+)`)
	matchAllCap := regexp.MustCompile(`([a-z0-9])([A-Z])`)

	out := matchFirstCap.ReplaceAllString(s, `${1}-${2}`)
	out = matchAllCap.ReplaceAllString(out, `${1}-${2}`)
	out = regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(out, `-`)
	out = strings.Trim(out, `-`)

	return strings.ToLower(out)
}

// ToCamelCase converts a string with underscores, dashes, or spaces to camelCase.
func ToCamelCase(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '_' || r == '-'
	})

	if len(words) == 0 {
		return ""
	}

	result := words[0]
	for _, w := range words[1:] {
		result += Capitalize(w)
	}

	return result
}

// LoadSpec parses the file as OpenAPI 3 spec and returns the data model.
func LoadSpec(specFile []byte) (*libopenapi.DocumentModel[v3.Document], error) {
	doc, err := libopenapi.NewDocument(specFile)
	if err != nil {
		return nil, err
	}

	docModel, errors := doc.BuildV3Model()
	if len(errors) > 0 {
		for i := range errors {
			fmt.Printf("error: %e\n", errors[i])
		}

		return nil, fmt.Errorf("cannot parse spec: %d errors.", len(errors))
	}

	return docModel, nil
}

// GetOutputPath returns the output path for the MDX file for the given operation.
func GetOutputPath(op *v3.Operation, prefix string) string {
	return fmt.Sprintf("%s", prefix)
}

// GetOutputFilename generates the filename from the operationId.
func GetOutputFilename(op *v3.Operation) string {
	return fmt.Sprintf("%s.mdx", ToKebabCase(op.OperationId))
}

// Capitalize returns the capitalized word.
func Capitalize(word string) string {
	runes := []rune(word)
	runes[0] = unicode.ToUpper(runes[0])

	return string(runes)
}

// GetLanguageName returns the printable name of a programming language label.
func GetLanguageName(lang string) string {
	lang = strings.ToLower(lang)
	lang = dictionary.Translate(lang)

	return Capitalize(lang)
}
