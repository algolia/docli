package cdn

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	JSDELIVR_API_URL = "https://data.jsdelivr.com/v1/package/npm"
	JSDELIVR_CDN_URL = "https://cdn.jsdelivr.net/npm"
)

// Options represents the options and flags for this command.
type Options struct {
	DataFile        string
	OutputDirectory string
	TemplateDir     string
}

// Package represents information about a package.
type Package struct {
	// Package name or label to identify snippets and templates
	Name string `yaml:"name"`
	// Optional: file to include. If omitted, the default import is used
	File string `yaml:"file,omitempty"`
	// Optional: package name if different from the Name field
	PackageName string `yaml:"pkg,omitempty"`
	// The SRI hash of the file. Retrieved from CDN
	Integrity string
	// The CDN include link. Retrieved from CDN
	Src string
	// Latest version of the package. Retrieved from CDN
	Version string
}

// NewCdnCommand returns a new instance of the `generate openapi` command.
func NewCdnCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "cdn",
		Short: "Generate HTML import snippets with latest versions",
		Long: heredoc.Doc(`
			This command generates import snippets with version numbers.

			When documenting code with HTML <link> or <script> tags for remote resources,
			it's best to specify a specific version and the matching SRI hash.

			The command reads a data file (default: cdn.yml),
			iterates over the entries,
			and applies matching templates from the templates directory.
			Each package name in cdn.yml must match a template name.
			For example, if the package is autocomplete_js,
			the command looks for the template file autocomplete_js.mdx.tmpl.
		`),
		Example: heredoc.Doc(`
			# Run from the root of algolia/docs-new
			docli gen cdn -o snippets/autocomplete/includes -d cdn.yml -t templates
		`),
		Run: func(cmd *cobra.Command, _ []string) {
			runCommand(opts)
		},
	}

	cmd.Flags().
		StringVarP(&opts.DataFile, "data", "d", "cdn.yml", "Data file with package information.")
	cmd.Flags().
		StringVarP(&opts.TemplateDir, "templates", "t", "templates", "Directory with template files for interpolation.")
	cmd.Flags().
		StringVarP(&opts.OutputDirectory, "output", "o", "out", "Output directory for generated files")

	return cmd
}

// runCommand runs the `generate openapi` command.
func runCommand(opts *Options) {
	data, err := readData(opts.DataFile)
	if err != nil {
		log.Fatalf("error: %e", err)
	}

	if err = os.MkdirAll(opts.OutputDirectory, 0o755); err != nil {
		log.Fatalf("error: %e", err)
	}

	for _, pkg := range data {
		if err = getLatestVersion(JSDELIVR_API_URL, &pkg); err != nil {
			log.Fatalf("error: %v", err)
		}

		if err = getIncludeLinks(JSDELIVR_API_URL, &pkg); err != nil {
			log.Fatalf("error: %v", err)
		}

		t, err := getTemplate(pkg, opts)
		if err != nil {
			log.Fatalf("error: %v", err)
		}

		out := filepath.Join(opts.OutputDirectory, pkg.Name+".mdx")

		f, err := os.Create(out)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		defer f.Close()

		if err = t.Execute(f, pkg); err != nil {
			log.Fatalf("error: %v", err)
		}

		fmt.Printf(
			"Writing include snippets for `%s` (%s) version %s\n",
			pkg.Name,
			pkg.PackageName,
			pkg.Version,
		)
	}
}

// readData reads a YAML file with the data needed to identify a package on the CDN.
func readData(filename string) ([]Package, error) {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var data []Package

	if err = yaml.Unmarshal(contents, &data); err != nil {
		return nil, err
	}

	for i := range data {
		// Handle optional PackageName fields
		if data[i].PackageName == "" {
			data[i].PackageName = data[i].Name
		}

		// Files must start with `/`
		if data[i].File != "" && !strings.HasPrefix(data[i].File, "/") {
			data[i].File = "/" + data[i].File
		}
	}

	return data, nil
}

// getLatestVersion returns the latest version available on JSDELIVR.
func getLatestVersion(baseUrl string, p *Package) error {
	url := fmt.Sprintf("%s/%s", baseUrl, p.PackageName)

	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var versionData struct {
		Tags     map[string]string `json:"tags"`
		Versions []string          `json:"versions"`
	}

	if err = json.NewDecoder(res.Body).Decode(&versionData); err != nil {
		return err
	}

	p.Version = versionData.Tags["latest"]

	return nil
}

func getIncludeLinks(baseUrl string, p *Package) error {
	url := fmt.Sprintf("%s/%s@%s/flat", baseUrl, p.PackageName, p.Version)

	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	type CDNFile struct {
		Name string `json:"name"`
		Hash string `json:"hash"`
	}

	var cdnFiles struct {
		Default string    `json:"default"`
		Files   []CDNFile `json:"files"`
	}

	if err = json.NewDecoder(res.Body).Decode(&cdnFiles); err != nil {
		return err
	}

	// Use the default import if not specified in the YAML file
	if p.File == "" {
		p.File = cdnFiles.Default
	}

	found := false

	for _, cdnFile := range cdnFiles.Files {
		if cdnFile.Name == p.File {
			p.Integrity = cdnFile.Hash
			p.Src = fmt.Sprintf("%s/%s@%s%s", JSDELIVR_CDN_URL, p.PackageName, p.Version, p.File)
			found = true
		}
	}

	if !found {
		return fmt.Errorf("file %s for snippet %s not found on CDN", p.File, p.Name)
	}

	return nil
}

func getTemplate(p Package, opts *Options) (*template.Template, error) {
	pattern := filepath.Join(opts.TemplateDir, p.Name+"*")

	tmpl, err := template.ParseGlob(pattern)
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}
