package guides

import (
	"reflect"
	"testing"
)

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

func TestGenerateMarkdownSnippet(t *testing.T) {
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
				"```go Go\n" +
				"fmt.Println(\"hello\")\n" +
				"```\n\n" +
				"</CodeGroup>",
		},
		{
			name: "two languages unsorted input",
			snippet: map[string]string{
				"python": `print("hi")`,
				"go":     `fmt.Println("hi")`,
			},
			want: "<CodeGroup>\n\n" +
				"```go Go\n" +
				"fmt.Println(\"hi\")\n" +
				"```\n\n" +
				"```python Python\n" +
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
				"```go Go\n" +
				"fmt.Println(\"hey\")\n" +
				"```\n\n" +
				"```js JavaScript\n" +
				"console.log(\"hey\")\n" +
				"```\n\n" +
				"```ruby Ruby\n" +
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
