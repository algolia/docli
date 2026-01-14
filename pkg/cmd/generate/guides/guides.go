package guides

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/algolia/docli/pkg/cmd/generate/utils"
	"github.com/algolia/docli/pkg/dictionary"
	"github.com/spf13/cobra"
)

type Options struct {
	GuidesFile      string
	OutputDirectory string
}

// GuidesMap represents the data from a guide file.
type GuidesMap map[string]map[string]string

func NewGuidesCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "guides <guides>",
		Short: "Generate guide snippets from a JSON file",
		Long: heredoc.Doc(`
			This command reads a data file with guide snippets.
			It generates an MDX file for each guide.
		`),
		Example: heredoc.Doc(`
			# Run from root of algolia/docs-new
			docli gen guides guides.json -o snippets/guides
		`),
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			opts.GuidesFile = args[0]
			runCommand(opts)
		},
	}

	cmd.Flags().
		StringVarP(&opts.OutputDirectory, "output", "o", "out", "Output directory for generated MDX files")

	return cmd
}

func runCommand(opts *Options) {
	bytes, err := os.ReadFile(opts.GuidesFile)
	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	var data GuidesMap
	if err := json.Unmarshal(bytes, &data); err != nil {
		log.Fatalf("Error: %e", err)
	}

	fmt.Printf("Generating guide snippet files for: %s\n", opts.GuidesFile)
	fmt.Printf("Writing output in: %s\n", opts.OutputDirectory)

	guideNames := make([]string, 0, len(data))
	for guide := range data {
		guideNames = append(guideNames, guide)
	}

	sort.Strings(guideNames)

	for _, guide := range guideNames {
		err := writeGuide(
			opts.OutputDirectory,
			fmt.Sprintf("%s.mdx", utils.ToKebabCase(guide)),
			generateMarkdownSnippet(data[guide]),
		)
		if err != nil {
			log.Fatalf("Error: %e", err)
		}
	}
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

// writeGuide writes the guide snippets into MDX files.
func writeGuide(path string, filename string, snippet string) error {
	err := os.MkdirAll(path, 0o755)
	if err != nil {
		return err
	}

	fullPath := filepath.Join(path, filename)

	out, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := out.WriteString(snippet); err != nil {
		return err
	}

	return nil
}
