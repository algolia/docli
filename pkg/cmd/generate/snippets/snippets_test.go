package snippets

import (
	"reflect"
	"testing"
)

func TestInvertSnippets(t *testing.T) {
	tests := []struct {
		name string
		data NestedMap
		want NestedMap
	}{
		{
			name: "empty input",
			data: NestedMap{},
			want: NestedMap{},
		},
		{
			name: "single language, multiple snippets/examples",
			data: NestedMap{
				"go": {
					"foo": {"ex1": "codeA", "ex2": "codeB"},
					"bar": {"ex1": "codeC"},
				},
			},
			want: NestedMap{
				"foo": {
					"ex1": {"go": "codeA"},
					"ex2": {"go": "codeB"},
				},
				"bar": {
					"ex1": {"go": "codeC"},
				},
			},
		},
		{
			name: "multiple languages, overlapping snippets/examples",
			data: NestedMap{
				"go": {
					"foo": {"ex1": "goA", "ex2": "goB"},
					"bar": {"ex1": "goC"},
				},
				"py": {
					"foo": {"ex1": "pyA", "ex3": "pyC"},
				},
			},
			want: NestedMap{
				"foo": {
					"ex1": {"go": "goA", "py": "pyA"},
					"ex2": {"go": "goB"},
					"ex3": {"py": "pyC"},
				},
				"bar": {
					"ex1": {"go": "goC"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := invertSnippets(tt.data)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("invertSnippets(%v) = %v; want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestSortLanguages(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
		want  []string
	}{
		{
			name:  "empty map",
			input: map[string]string{},
			want:  []string{},
		},
		{
			name:  "single key",
			input: map[string]string{"go": "fmt.Println"},
			want:  []string{"go"},
		},
		{
			name: "already sorted keys",
			input: map[string]string{
				"c":  "printf",
				"go": "fmt.Println",
				"py": "print",
			},
			want: []string{"c", "go", "py"},
		},
		{
			name: "unsorted keys",
			input: map[string]string{
				"py": "print",
				"go": "fmt.Println",
				"c":  "printf",
			},
			want: []string{"c", "go", "py"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortLanguages(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sortLanguages(%v) = %v; want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateMarkdownSnippet(t *testing.T) { //nolint:funlen
	tests := []struct {
		name    string
		snippet map[string]string
		want    string
	}{
		{
			name:    "empty map",
			snippet: map[string]string{},
			want:    "<CodeGroup>\n\n</CodeGroup>",
		},
		{
			name: "single language",
			snippet: map[string]string{
				"go": `fmt.Println("hello")`,
			},
			want: "<CodeGroup>\n\n" +
				"```go\n" +
				"fmt.Println(\"hello\")\n" +
				"```\n\n" +
				"</CodeGroup>",
		},
		{
			name: "two languages unsorted input",
			snippet: map[string]string{
				"py": `print("hi")`,
				"go": `fmt.Println("hi")`,
			},
			want: "<CodeGroup>\n\n" +
				"```go\n" +
				"fmt.Println(\"hi\")\n" +
				"```\n\n" +
				"```py\n" +
				"print(\"hi\")\n" +
				"```\n\n" +
				"</CodeGroup>",
		},
		{
			name: "multiple languages arbitrary order",
			snippet: map[string]string{
				"ruby": `puts "hey"`,
				"js":   `console.log("hey")`,
				"go":   `fmt.Println("hey")`,
			},
			want: "<CodeGroup>\n\n" +
				"```go\n" +
				"fmt.Println(\"hey\")\n" +
				"```\n\n" +
				"```js\n" +
				"console.log(\"hey\")\n" +
				"```\n\n" +
				"```ruby\n" +
				"puts \"hey\"\n" +
				"```\n\n" +
				"</CodeGroup>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateMarkdownSnippet(tt.snippet)
			if got != tt.want {
				t.Errorf("generateMarkdownSnippet(%v) =\n%q\nwant:\n%q", tt.snippet, got, tt.want)
			}
		})
	}
}
