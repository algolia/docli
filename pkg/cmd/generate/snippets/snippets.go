package snippets

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/algolia/docli/pkg/cmd/generate/utils"
	"github.com/algolia/docli/pkg/dictionary"
	"github.com/algolia/docli/pkg/output"
	"github.com/algolia/docli/pkg/validate"
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
		Use:   "snippets <snippets>",
		Short: "Generate API client example snippets from an OpenAPI snippet file",
		Long: heredoc.Doc(`
			This command reads a data file with API client usage snippets.
			It generates an MDX file for each snippet so you can include them in the docs.
		`),
		Example: heredoc.Doc(`
			# Run from root of algolia/docs-new
			docli gen snippets specs/search-snippets.json -o openapi-snippets/search
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.SnippetsFile = args[0]

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
	if err := validate.ExistingFile(opts.SnippetsFile, "snippets file"); err != nil {
		return err
	}

	if err := validate.OutputDir(opts.OutputDirectory, "output directory"); err != nil {
		return err
	}

	bytes, err := os.ReadFile(opts.SnippetsFile)
	if err != nil {
		return fmt.Errorf("read snippets file %s: %w", opts.SnippetsFile, err)
	}

	var data NestedMap
	if err := json.Unmarshal(bytes, &data); err != nil {
		return fmt.Errorf("parse snippets file %s: %w", opts.SnippetsFile, err)
	}

	printer.Infof("Generating usage snippet files for: %s\n", opts.SnippetsFile)
	printer.Infof("Writing output in: %s\n", opts.OutputDirectory)

	rawSnippets := invertSnippets(data)

	for snippet, examples := range rawSnippets {
		for name, example := range examples {
			err := writeSnippet(
				filepath.Join(opts.OutputDirectory, utils.ToKebabCase(snippet)),
				fmt.Sprintf("%s.mdx", utils.ToCamelCase(name)),
				generateMarkdownSnippet(example),
				printer,
			)
			if err != nil {
				return fmt.Errorf("write snippet %s/%s: %w", snippet, name, err)
			}
		}
	}

	return nil
}

// generateMarkdownSnippet generates a CodeGroup block.
func generateMarkdownSnippet(snippet map[string]string) string {
	result := "<CodeGroup>\n"
	languages := sortLanguages(snippet)

	for _, lang := range languages {
		result += fmt.Sprintf(
			"\n```%s %s\n",
			dictionary.NormalizeLang(lang),
			utils.GetLanguageName(lang),
		)
		replaced := strings.ReplaceAll(snippet[lang], "<YOUR_INDEX_NAME>", "ALGOLIA_INDEX_NAME")
		replaced = strings.ReplaceAll(
			replaced,
			"cts_e2e_deleteObjects_javascript",
			"ALGOLIA_INDEX_NAME",
		)
		result += replaced
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
func writeSnippet(path string, filename string, snippet string, printer *output.Printer) error {
	if !printer.IsDryRun() {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
	}

	fullPath := filepath.Join(path, filename)

	return printer.WriteFile(fullPath, func(w io.Writer) error {
		_, err := io.WriteString(w, snippet)

		return err
	})
}
