package sla

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"text/template"

	"github.com/MakeNowJust/heredoc"
	"github.com/algolia/docli/pkg/cmd/generate/utils"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

type Options struct {
	DataFile     string
	Output       string
	VersionsFile string
}

// VersionInfo represents the version information for a single version of an API client.
type VersionInfo struct {
	ReleaseDate string `json:"releaseDate"`
	SLAStatus   string `json:"slaStatus"`
	SLAEndDate  string `json:"slaEndDate,omitempty"`
}

// Version maps a version string to its version information.
type Version map[string]VersionInfo

// VersionEntry corresponds to one version string and info.
// []VersionEntry is a flat version of Version for sorting.
type VersionEntry struct {
	Version string // Version string, e.g. 1.2.3
	Info    VersionInfo
}

// Clients maps a client language to its versions.
type Clients map[string]Version

// ClientEntry represents all versions for a single language.
// []ClientEntry is a flat version of Clients for sorting.
type ClientEntry struct {
	Language string // Language string, e.g. csharp, go, etc.
	Versions []VersionEntry
}

//go:embed page.mdx.tmpl
var pageTemplate string

func NewSLACommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "sla <data>",
		Short: "Generate page with SLA information for API clients",
		Long: heredoc.Doc(`
			This command reads a data file with API client versions and SLA status,
			then generates an MDX file listing supported versions.

			Use --versions-snippets-file to also generate a snippet file,
			so you can include the latest client version in the docs.
		`),
		Example: heredoc.Doc(`
			# Run from root of algolia/docs-new
			docli gen sla specs/versions-history-with-sla-and-support-policy.json \
		  	-o doc/libraries/sdk/versions.mdx \
				--versions-snippets-file snippets/sdk/versions.mdx`),
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			opts.DataFile = args[0]
			runCommand(opts)
		},
	}

	cmd.Flags().
		StringVarP(&opts.Output, "output", "o", "", "MDX file for listing the supported versions")
	cmd.Flags().
		StringVar(&opts.VersionsFile, "versions-snippets-file", "", "Snippet file with latest released version numbers")

	return cmd
}

func runCommand(opts *Options) {
	// Read data
	rawData, err := os.ReadFile(opts.DataFile)
	if err != nil {
		log.Fatalf("can't read data file: %v", err)
	}

	data, err := parseVersions(rawData)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	sorted := sortVersions(&data)

	funcMap := template.FuncMap{
		"capitalize":      utils.Capitalize,
		"getLanguageName": utils.GetLanguageName,
	}

	var output io.Writer
	if opts.Output == "" {
		output = os.Stdout
	} else {
		output, err = os.Create(opts.Output)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
	}

	if err = renderPage(output, pageTemplate, sorted, funcMap); err != nil {
		log.Fatalf("error: %v", err)
	}

	if opts.VersionsFile != "" {
		versionsOutput, err := os.Create(opts.VersionsFile)
		if err != nil {
			log.Fatalf("error: %v", err)
		}

		renderVersionsFile(versionsOutput, sorted)
	}
}

// parseVersions reads a JSON file and parses it into a Clients struct.
func parseVersions(raw []byte) (Clients, error) {
	var data Clients
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("cannot parse JSON: %w", err)
	}

	return data, nil
}

// sortVersions sorts the version info in descending order.
// Since a map is unordered we need to transform the structure.
func sortVersions(data *Clients) []ClientEntry {
	// sort the keys alphabetically
	langs := make([]string, 0, len(*data))
	for lang := range *data {
		langs = append(langs, lang)
	}

	sort.Strings(langs)

	result := make([]ClientEntry, 0, len(langs))

	for _, lang := range langs {
		versionsMap := (*data)[lang]

		// Extract and sort version strings (descending semver)
		versions := make([]string, 0, len(versionsMap))
		for v := range versionsMap {
			versions = append(versions, v)
		}

		// Custom sort using semver
		sort.Slice(versions, func(i, j int) bool {
			// expects a "v" prefix
			return semver.Compare("v"+versions[i], "v"+versions[j]) > 0
		})

		// Build a slice of VersionEntry in that order
		slice := make([]VersionEntry, 0, len(versions))
		for _, v := range versions {
			slice = append(slice, VersionEntry{
				Version: v,
				Info:    versionsMap[v],
			})
		}

		result = append(result, ClientEntry{
			Language: lang,
			Versions: slice,
		})
	}

	return result
}

func renderPage(
	w io.Writer,
	templateString string,
	data []ClientEntry,
	funcMap template.FuncMap,
) error {
	tmpl, err := template.New("versions").Funcs(funcMap).Parse(templateString)
	if err != nil {
		return err
	}

	if err = tmpl.Execute(w, data); err != nil {
		return err
	}

	return nil
}

func renderVersionsFile(w io.Writer, data []ClientEntry) {
	fmt.Fprintln(w, "export const sdkVersions = {")

	for _, client := range data {
		fmt.Fprintf(w, "  %s: {\n", client.Language)

		seenMajors := make(map[string]bool)

		for _, ver := range client.Versions {
			major := semver.Major("v" + ver.Version)
			if !seenMajors[major] {
				seenMajors[major] = true

				fmt.Fprintf(w, "    %s: \"%s\",\n", major, ver.Version)
			}
		}

		fmt.Fprintln(w, "  },")
	}

	fmt.Fprintln(w, "};")
}
