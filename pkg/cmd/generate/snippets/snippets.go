package snippets

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/algolia/docli/pkg/cmd/generate/utils"
	"github.com/spf13/cobra"
)

type Options struct {
	SnippetsFile    string
	OutputDirectory string
}

// NestedMap represents the data from the nested snippet file.
type NestedMap map[string]map[string]map[string]string

func NewSnippetsCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "snippets",
		Short: "Generate snippets from an OpenAPI snippet file.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			opts.SnippetsFile = args[0]
			runCommand(opts)
		},
	}

	cmd.Flags().
		StringVarP(&opts.OutputDirectory, "output", "o", "out", "Output directory for generated MDX files")

	return cmd
}

func runCommand(opts *Options) {
	bytes, err := os.ReadFile(opts.SnippetsFile)
	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	var data NestedMap
	if err := json.Unmarshal(bytes, &data); err != nil {
		log.Fatalf("Error: %e", err)
	}

	fmt.Printf("Generating usage snippet files for: %s\n", opts.SnippetsFile)
	fmt.Printf("Writing output in: %s\n", opts.OutputDirectory)

	rawSnippets := invertSnippets(data)

	for snippet, examples := range rawSnippets {
		for name, example := range examples {
			err := writeSnippet(
				filepath.Join(opts.OutputDirectory, snippet),
				fmt.Sprintf("%s.mdx", utils.ToCamelCase(name)),
				generateMarkdownSnippet(example),
			)
			if err != nil {
				log.Fatalf("Error: %e", err)
			}
		}
	}
}

// generateMarkdownSnippet generates a CodeGroup block.
func generateMarkdownSnippet(snippet map[string]string) string {
	result := "<CodeGroup>\n"
	languages := sortLanguages(snippet)

	for _, lang := range languages {
		result += fmt.Sprintf("\n```%s %s\n", lang, utils.GetLanguageName(lang))
		result += snippet[lang]
		result += "\n```\n"
	}

	result += "\n</CodeGroup>"

	return result
}

// sortLanguages returns a list of sorted languages.
func sortLanguages(snippet map[string]string) []string {
	sorted := make([]string, 0, len(snippet))

	for lang := range snippet {
		sorted = append(sorted, lang)
	}

	sort.Strings(sorted)

	return sorted
}

// invertSnippets converts the original structure LANG -> SNIPPET -> VARIANT
// and returns SNIPPET -> EXAMPLE -> LANG.
func invertSnippets(data NestedMap) NestedMap {
	result := make(NestedMap)

	for lang, snippets := range data {
		for snippet, examples := range snippets {
			if _, ok := result[snippet]; !ok {
				result[snippet] = make(map[string]map[string]string)
			}

			for example, code := range examples {
				if _, ok := result[snippet][example]; !ok {
					result[snippet][example] = make(map[string]string)
				}

				result[snippet][example][lang] = code
			}
		}
	}

	return result
}

// writeSnippet writes the snippets into MDX files.
func writeSnippet(path string, filename string, snippet string) error {
	err := os.MkdirAll(path, 0o755)
	if err != nil {
		return err
	}

	fullPath := filepath.Join(path, filename)

	out, err := os.Create(fullPath)
	if err != nil {
		return err
	}

	out.WriteString(snippet)

	return nil
}
