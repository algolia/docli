package utils

import (
	"strings"
	"testing"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"gopkg.in/yaml.v3"
)

func TestGetLanguageName(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		expected string
	}{
		{
			name:     "Test C#",
			label:    "csharp",
			expected: "C#",
		},
		{
			name:     "Test Dart",
			label:    "dart",
			expected: "Dart",
		},
		{
			name:     "Test Go",
			label:    "go",
			expected: "Go",
		},
		{
			name:     "Test Java",
			label:    "java",
			expected: "Java",
		},
		{
			name:     "Test JavaScript",
			label:    "javascript",
			expected: "JavaScript",
		},
		{
			name:     "Test Kotlin",
			label:    "kotlin",
			expected: "Kotlin",
		},
		{
			name:     "Test PHP",
			label:    "php",
			expected: "PHP",
		},
		{
			name:     "Test Python",
			label:    "python",
			expected: "Python",
		},
		{
			name:     "Test Ruby",
			label:    "ruby",
			expected: "Ruby",
		},
		{
			name:     "Test Scala",
			label:    "scala",
			expected: "Scala",
		},
		{
			name:     "Test Swift",
			label:    "swift",
			expected: "Swift",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := GetLanguageName(tt.label)
			if got != tt.expected {
				t.Errorf("Error in GetLanguageName, got %s, expected %s", got, tt.expected)
			}
		})
	}
}

func TestCapitalize(t *testing.T) {
	tests := []struct {
		name     string
		word     string
		expected string
	}{
		{
			name:     "Capitalize Test",
			word:     "test",
			expected: "Test",
		},
		{
			name:     "Capitalize TEst",
			word:     "tEst",
			expected: "TEst",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Capitalize(tt.word)
			if got != tt.expected {
				t.Errorf("Error in Capitalize. got %s, expected %s", got, tt.expected)
			}
		})
	}
}

func TestKebabCase(t *testing.T) {
	tests := []struct {
		name     string
		word     string
		expected string
	}{
		{
			name:     "Test searchSingleIndex",
			word:     "searchSingleIndex",
			expected: "search-single-index",
		},
		{
			name:     "Test getApiKey",
			word:     "getApiKey",
			expected: "get-api-key",
		},
		{
			name:     "Test listAPIKeys",
			word:     "listAPIKeys",
			expected: "list-api-keys",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ToKebabCase(tt.word)
			if got != tt.expected {
				t.Errorf("Error in ToKebabCase. got %s, expected %s", got, tt.expected)
			}
		})
	}
}

func TestCamelCase(t *testing.T) {
	tests := []struct {
		name     string
		word     string
		expected string
	}{
		{
			name:     "Test with spaces",
			word:     "search single index",
			expected: "searchSingleIndex",
		},
		{
			name:     "Test with underscores",
			word:     "search_single_index",
			expected: "searchSingleIndex",
		},
		{
			name:     "Test with dashes",
			word:     "search-single-index",
			expected: "searchSingleIndex",
		},
		{
			name:     "Test with mixed stuff",
			word:     "search-single_index now",
			expected: "searchSingleIndexNow",
		},
		{
			name:     "Test with camelCase",
			word:     "searchSingleIndex",
			expected: "searchSingleIndex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ToCamelCase(tt.word)
			if got != tt.expected {
				t.Errorf("Error in ToCamelCase. got %s, expected %s", got, tt.expected)
			}
		})
	}
}

func TestAclToString(t *testing.T) {
	tests := []struct {
		name     string
		acl      []string
		expected string
	}{
		{
			name:     "Test search",
			acl:      []string{"search"},
			expected: "`search`",
		},
		{
			name:     "Test search and settings",
			acl:      []string{"search", "settings"},
			expected: "`search`, `settings`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := AclToString(tt.acl)
			if got != tt.expected {
				t.Errorf("Error in AclToString. got %s, expected %s", got, tt.expected)
			}
		})
	}
}

func TestGetApiName(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Test /specs/search.yml",
			path:     "/specs/search.yml",
			expected: "search",
		},
		{
			name:     "Test /specs/searchstats.yml",
			path:     "/specs/searchstats.yml",
			expected: "analytics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := GetApiName(tt.path)
			if got != tt.expected {
				t.Errorf("Error in GetApiName. got %s, expected %s", got, tt.expected)
			}
		})
	}
}

func TestOutputFilename(t *testing.T) {
	testOp := &v3.Operation{OperationId: "searchSingleIndex"}
	expected := "search-single-index.mdx"

	got := GetOutputFilename(testOp)
	if got != expected {
		t.Errorf("Error in TestOutputFilename. Got %s, expected %s", got, expected)
	}
}

func TestOutputPath(t *testing.T) {
	prefix := "foo"

	tests := []struct {
		name     string
		op       *v3.Operation
		expected string
	}{
		{
			name:     "With tags",
			op:       &v3.Operation{Tags: []string{"Search"}},
			expected: "foo/search",
		},
		{
			name:     "Without tags",
			op:       &v3.Operation{},
			expected: prefix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := GetOutputPath(tt.op, prefix)

			if got != tt.expected {
				t.Errorf("Got %s, expected %s", got, tt.expected)
			}
		})
	}
}

func mockOp(extensions *yaml.Node) v3.Operation {
	op := v3.Operation{}
	op.Extensions = orderedmap.New[string, *yaml.Node]()
	op.Extensions.Set("x-acl", extensions)

	return op
}

func TestGetAcl(t *testing.T) {
	tests := []struct {
		name        string
		extensions  *yaml.Node
		expected    []string
		expectError bool
		errorSubstr string
	}{
		{
			name: "Good ACL",
			extensions: &yaml.Node{
				Kind: yaml.SequenceNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Value: "search"},
					{Kind: yaml.ScalarNode, Value: "settings"},
				},
			},
			expected:    []string{"search", "settings"},
			expectError: false,
		},
		{
			name: "Extension is a scalar node",
			extensions: &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: "search",
			},
			expected:    []string{},
			expectError: true,
			errorSubstr: "expected a sequence node",
		},
		{
			name: "Extension is a sequence of sequences",
			extensions: &yaml.Node{
				Kind: yaml.SequenceNode,
				Content: []*yaml.Node{
					{Kind: yaml.SequenceNode},
					{Kind: yaml.SequenceNode},
				},
			},
			expected:    []string{},
			expectError: true,
			errorSubstr: "expected scalar nodes in sequence",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			op := mockOp(tt.extensions)

			got, err := GetAcl(&op)
			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			if tt.expectError {
				if err == nil {
					t.Fatal("error expected, but there was none")
				}

				if !strings.Contains(err.Error(), tt.errorSubstr) {
					t.Errorf("expected error substring %s, got %s", tt.errorSubstr, err.Error())
				}
			}

			if len(got) != len(tt.expected) {
				t.Errorf("Got %d elements, expected %d", len(got), len(tt.expected))
			}

			for i := range tt.expected {
				if got[i] != tt.expected[i] {
					t.Errorf(
						"Unexpected result at position %d. Got %s, expected %s",
						i,
						got[i],
						tt.expected[i],
					)
				}
			}
		})
	}
}
