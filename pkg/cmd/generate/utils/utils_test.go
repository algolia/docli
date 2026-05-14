package utils

import (
	"strings"
	"testing"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"go.yaml.in/yaml/v4"
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

			got := GetAPIName(tt.path)
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

func TestOperationIDVariants(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:  "camelCase input",
			input: "searchSingleIndex",
			expected: []string{
				"searchSingleIndex",
				"SearchSingleIndex",
				"search_single_index",
			},
		},
		{
			name:  "already PascalCase dedupes",
			input: "SearchSingleIndex",
			expected: []string{
				"SearchSingleIndex",
				"search_single_index",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := OperationIDVariants(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf(
					"OperationIDVariants() len = %d, want %d (%#v)",
					len(got),
					len(tt.expected),
					got,
				)
			}

			for i := range tt.expected {
				if got[i] != tt.expected[i] {
					t.Fatalf(
						"OperationIDVariants()[%d] = %q, want %q (%#v)",
						i,
						got[i],
						tt.expected[i],
						got,
					)
				}
			}
		})
	}
}

func TestSplitDescription(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantShort string
		wantLong  string
	}{
		{
			name:      "Split paragraphs with whitespace",
			input:     "First paragraph.\n \nSecond paragraph.",
			wantShort: "First paragraph.",
			wantLong:  "Second paragraph.",
		},
		{
			name:      "Split CRLF paragraphs",
			input:     "First paragraph.\r\n\r\nSecond paragraph.",
			wantShort: "First paragraph.",
			wantLong:  "Second paragraph.",
		},
		{
			name:      "Skip decimal point",
			input:     "Version 3.14 is supported. Use it carefully.",
			wantShort: "Version 3.14 is supported.",
			wantLong:  "Use it carefully.",
		},
		{
			name:      "Skip abbreviation",
			input:     "Use e.g. filters. They narrow results.",
			wantShort: "Use e.g. filters.",
			wantLong:  "They narrow results.",
		},
		{
			name:      "Keep entire text when no safe sentence boundary",
			input:     "Use the API. then continue",
			wantShort: "Use the API. then continue",
			wantLong:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotShort, gotLong := SplitDescription(tt.input)
			if gotShort != tt.wantShort || gotLong != tt.wantLong {
				t.Fatalf(
					"SplitDescription() = (%q, %q), want (%q, %q)",
					gotShort,
					gotLong,
					tt.wantShort,
					tt.wantLong,
				)
			}
		})
	}
}

func TestStripMarkdown(t *testing.T) {
	input := "Use **bold** [links](https://algolia.com), _emphasis_, `code`, <em>HTML</em>, and <https://algolia.com>."
	want := "Use bold links, emphasis, code, HTML, and https://algolia.com."

	if got := StripMarkdown(input); got != want {
		t.Fatalf("StripMarkdown() = %q, want %q", got, want)
	}
}

func TestQuoteFrontmatterString(t *testing.T) {
	input := `A "quoted" path C:\tmp: #tag`
	want := `"A \"quoted\" path C:\\tmp: #tag"`

	if got := QuoteFrontmatterString(input); got != want {
		t.Fatalf("QuoteFrontmatterString() = %q, want %q", got, want)
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

			got, err := GetACL(&op)
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
