package sla

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"text/template"
	"unicode"

	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

type Options struct {
	DataFile string
}

// VersionInfo represents the version information for a single version of an API client.
type VersionInfo struct {
	SlaStatus      string `json:"slaStatus"`
	SupportStatus  string `json:"supportStatus"`
	ReleaseDate    string `json:"releaseDate"`
	SlaEndDate     string `json:"slaEndDate,omitempty"`
	SupportEndDate string `json:"supportEndDate,omitempty"`
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

func NewSlaCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "sla",
		Short: "Generate page with SLA information",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			opts.DataFile = args[0]
			runCommand(opts)
		},
	}

	return cmd
}

func runCommand(opts *Options) {
	data, err := parseVersions(opts.DataFile)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	sorted := sortVersions(&data)

	if err = renderPage(pageTemplate, sorted); err != nil {
		log.Fatalf("error: %v", err)
	}
}

// parseVersions reads a JSON file and parses it into a SlaData struct.
func parseVersions(path string) (Clients, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %q: %w", path, err)
	}

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

// languages is a map for translating a language id to its proper name.
var languages = map[string]string{
	"csharp":     "C#",
	"javascript": "JavaScript",
	"php":        "PHP",
}

// capitalize returns the capitalized word.
func capitalize(word string) string {
	word = strings.ToLower(word)
	runes := []rune(word)
	runes[0] = unicode.ToUpper(runes[0])

	return string(runes)
}

// getLanguageName returns the printable name of a language id.
func getLanguageName(lang string) string {
	if lang == "" {
		return ""
	}

	lang = strings.ToLower(lang)

	if special, ok := languages[lang]; ok {
		return special
	}

	return capitalize(lang)
}

func renderPage(templateString string, data []ClientEntry) error {
	funcMap := template.FuncMap{
		"capitalize":      capitalize,
		"getLanguageName": getLanguageName,
	}

	tmpl, err := template.New("versions").Funcs(funcMap).Parse(templateString)
	if err != nil {
		return err
	}

	if err = tmpl.Execute(os.Stdout, data); err != nil {
		return err
	}

	return nil
}
