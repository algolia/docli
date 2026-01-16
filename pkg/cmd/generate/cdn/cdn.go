package cdn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/algolia/docli/pkg/output"
	"github.com/algolia/docli/pkg/validate"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Options represents the options and flags for this command.
type Options struct {
	DataFile        string
	OutputDirectory string
	TemplateDir     string
}

const (
	npmRegistryURL  = "https://registry.npmjs.org"
	jsDelivrDataURL = "https://data.jsdelivr.com/v1/package/npm"
	jsDelivrCdnURL  = "https://cdn.jsdelivr.net/npm"
	defaultTimeout  = 10 * time.Second
)

// packageVersion represents NPM metadata for one specific version.
type packageVersion struct {
	JSDelivr string `json:"jsdelivr"`
	UNPKG    string `json:"unpkg"`
	Browser  any    `json:"browser"`
	Module   string `json:"module"`
	Main     string `json:"main"`
}

// packageMetadata represents package metadata from the NPM registry.
type packageMetadata struct {
	DistTags map[string]string         `json:"dist-tags"`
	Versions map[string]packageVersion `json:"versions"`
}

// PackageSpec represents information about a package from the data file.
type PackageSpec struct {
	// Package name or label to identify snippets and templates
	Name string `yaml:"name"`
	// Optional: file to include. If omitted, the default import is used
	File string `yaml:"file,omitempty"`
	// Optional: package name if different from the Name field
	PackageName string `yaml:"pkg,omitempty"`
}

// ResolvedPackage represents a fully populated package ready for templating.
type ResolvedPackage struct {
	PackageSpec

	// The SRI hash of the file. Retrieved from CDN
	Integrity string
	// The CDN include link. Retrieved from CDN
	Src string
	// Latest version of the package. Retrieved from CDN
	Version string
}

type Resolver struct {
	client          *http.Client
	npmRegistryURL  string
	jsDelivrDataURL string
	jsDelivrCdnURL  string
	metaCache       map[string]*packageMetadata
	cdnCache        map[string]map[string]string
	mu              sync.Mutex
}

func NewResolver(client *http.Client) *Resolver {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}

	return &Resolver{
		client:          client,
		npmRegistryURL:  npmRegistryURL,
		jsDelivrDataURL: jsDelivrDataURL,
		jsDelivrCdnURL:  jsDelivrCdnURL,
		metaCache:       make(map[string]*packageMetadata),
		cdnCache:        make(map[string]map[string]string),
	}
}

func (r *Resolver) Resolve(pkg PackageSpec) (ResolvedPackage, error) {
	return r.ResolveWithContext(context.Background(), pkg)
}

func (r *Resolver) ResolveWithContext(
	ctx context.Context,
	pkg PackageSpec,
) (ResolvedPackage, error) {
	resolved := ResolvedPackage{PackageSpec: pkg}
	if resolved.PackageName == "" {
		resolved.PackageName = resolved.Name
	}

	metaData, err := r.fetchNPMMetadata(ctx, resolved.PackageName)
	if err != nil {
		return ResolvedPackage{}, err
	}

	version, err := r.latestVersion(metaData, resolved.Name)
	if err != nil {
		return ResolvedPackage{}, err
	}

	resolved.Version = version

	if resolved.File == "" {
		file, err := r.defaultFile(metaData, resolved.PackageName, resolved.Name, resolved.Version)
		if err != nil {
			return ResolvedPackage{}, err
		}

		resolved.File = file
	}

	sanitizedFile, err := sanitizeFilePath(resolved.File)
	if err != nil {
		return ResolvedPackage{}, err
	}

	resolved.File = sanitizedFile

	integrity, src, err := r.includeLink(
		ctx,
		resolved.PackageName,
		resolved.Version,
		resolved.File,
		resolved.Name,
	)
	if err != nil {
		return ResolvedPackage{}, err
	}

	resolved.Integrity = integrity
	resolved.Src = src

	return resolved, nil
}

func (r *Resolver) fetchNPMMetadata(ctx context.Context, pkgName string) (*packageMetadata, error) {
	url := fmt.Sprintf("%s/%s", strings.TrimRight(r.npmRegistryURL, "/"), pkgName)

	if meta := r.cachedMetadata(pkgName); meta != nil {
		return meta, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"can't get latest version of package %s from npm: %s",
			pkgName,
			res.Status,
		)
	}

	var metaData packageMetadata
	if err = json.NewDecoder(res.Body).Decode(&metaData); err != nil {
		return nil, err
	}

	r.storeMetadata(pkgName, &metaData)

	return &metaData, nil
}

func (r *Resolver) latestVersion(metaData *packageMetadata, name string) (string, error) {
	if metaData.DistTags == nil {
		return "", fmt.Errorf("no dist-tags found for package %s", name)
	}

	latest, ok := metaData.DistTags["latest"]
	if !ok || latest == "" {
		return "", fmt.Errorf("no latest dist-tag found for package %s", name)
	}

	return latest, nil
}

func (r *Resolver) defaultFile(
	metaData *packageMetadata,
	packageName string,
	name string,
	version string,
) (string, error) {
	pkgInfo, ok := metaData.Versions[version]
	if !ok {
		return "", fmt.Errorf("no pkg information found for %s version %s", name, version)
	}

	if pkgInfo.JSDelivr != "" {
		return "/" + pkgInfo.JSDelivr, nil
	}

	if pkgInfo.UNPKG != "" {
		return "/" + pkgInfo.UNPKG, nil
	}

	if pkgInfo.Module != "" {
		return "/" + pkgInfo.Module, nil
	}

	if pkgInfo.Main != "" {
		return "/" + pkgInfo.Main, nil
	}

	return "", fmt.Errorf(
		"no default file import found for %s version %s. Add it explicitly to the CDN data file",
		packageName,
		version,
	)
}

func sanitizeFilePath(file string) (string, error) {
	trimmed := strings.TrimSpace(file)
	if trimmed == "" {
		return "", fmt.Errorf("file path is empty")
	}

	cleaned := path.Clean("/" + trimmed)
	if cleaned == "/" {
		return "", fmt.Errorf("file path %q resolves to root", file)
	}

	return cleaned, nil
}

func (r *Resolver) includeLink(
	ctx context.Context,
	packageName,
	version,
	file,
	snippetName string,
) (string, string, error) {
	hashes, err := r.cdnFiles(ctx, packageName, version)
	if err != nil {
		return "", "", err
	}

	hash, ok := hashes[file]
	if !ok {
		return "", "", fmt.Errorf("file %s for snippet %s not found on CDN", file, snippetName)
	}

	return hash, r.cdnSrc(packageName, version, file), nil
}

func (r *Resolver) cdnFiles(
	ctx context.Context,
	packageName string,
	version string,
) (map[string]string, error) {
	url := fmt.Sprintf(
		"%s/%s@%s/flat",
		strings.TrimRight(r.jsDelivrDataURL, "/"),
		packageName,
		version,
	)

	if hashes := r.cachedCDNFiles(packageName, version); hashes != nil {
		return hashes, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request to %s failed with status %s", url, res.Status)
	}

	type CDNFile struct {
		Name string `json:"name"`
		Hash string `json:"hash"`
	}

	var cdnFiles struct {
		Files []CDNFile `json:"files"`
	}

	if err = json.NewDecoder(res.Body).Decode(&cdnFiles); err != nil {
		return nil, err
	}

	hashes := make(map[string]string, len(cdnFiles.Files))
	for _, cdnFile := range cdnFiles.Files {
		hashes[cdnFile.Name] = "sha256-" + cdnFile.Hash
	}

	r.storeCDNFiles(packageName, version, hashes)

	return hashes, nil
}

func (r *Resolver) cdnSrc(packageName, version, file string) string {
	return fmt.Sprintf(
		"%s/%s@%s%s",
		strings.TrimRight(r.jsDelivrCdnURL, "/"),
		packageName,
		version,
		file,
	)
}

func (r *Resolver) cachedMetadata(pkgName string) *packageMetadata {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.metaCache[pkgName]
}

func (r *Resolver) storeMetadata(pkgName string, meta *packageMetadata) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.metaCache[pkgName] = meta
}

func (r *Resolver) cachedCDNFiles(packageName, version string) map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.cdnCache[packageName+"@"+version]
}

func (r *Resolver) storeCDNFiles(packageName, version string, hashes map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cdnCache[packageName+"@"+version] = hashes
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
			docli gen cdn -o include-snippets [-d cdn.yml] [-t templates]
		`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			printer, err := output.New(cmd)
			if err != nil {
				return err
			}

			return runCommand(cmd.Context(), opts, printer)
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
func runCommand(ctx context.Context, opts *Options, printer *output.Printer) error {
	if err := validateOptions(opts); err != nil {
		return err
	}

	data, err := readCDNDataFile(opts.DataFile)
	if err != nil {
		return fmt.Errorf("read CDN data file %s: %w", opts.DataFile, err)
	}

	if !printer.IsDryRun() {
		if err = os.MkdirAll(opts.OutputDirectory, 0o700); err != nil {
			return fmt.Errorf("create output directory %s: %w", opts.OutputDirectory, err)
		}
	}

	resolver := NewResolver(nil)

	for _, pkg := range data {
		if err := writePackage(ctx, opts, resolver, printer, pkg); err != nil {
			return err
		}
	}

	return nil
}

func validateOptions(opts *Options) error {
	if err := validate.ExistingFile(opts.DataFile, "data file"); err != nil {
		return err
	}

	if err := validate.ExistingDir(opts.TemplateDir, "template directory"); err != nil {
		return err
	}

	return validate.OutputDir(opts.OutputDirectory, "output directory")
}

func writePackage(
	ctx context.Context,
	opts *Options,
	resolver *Resolver,
	printer *output.Printer,
	pkg PackageSpec,
) error {
	resolved, err := resolver.ResolveWithContext(ctx, pkg)
	if err != nil {
		return fmt.Errorf("resolve package %s: %w", pkg.Name, err)
	}

	t, err := getTemplate(resolved.Name, opts)
	if err != nil {
		return fmt.Errorf("load template for %s: %w", resolved.Name, err)
	}

	out := filepath.Join(opts.OutputDirectory, resolved.Name+".mdx")

	if err := printer.WriteFile(out, func(w io.Writer) error {
		return t.Execute(w, resolved)
	}); err != nil {
		return fmt.Errorf("write output for %s: %w", resolved.Name, err)
	}

	if !printer.IsDryRun() {
		printer.Infof(
			"Writing include snippets for `%s` (%s) version %s\n",
			resolved.Name,
			resolved.PackageName,
			resolved.Version,
		)
	}

	return nil
}

// readCDNDataFile reads a YAML file with the data needed to identify a package on the CDN.
func readCDNDataFile(filename string) ([]PackageSpec, error) {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var data []PackageSpec

	if err = yaml.Unmarshal(contents, &data); err != nil {
		return nil, err
	}

	return data, nil
}

func getTemplate(name string, opts *Options) (*template.Template, error) {
	primary := filepath.Join(opts.TemplateDir, name+".mdx.tmpl")

	_, err := os.Stat(primary)
	if err == nil {
		return template.ParseFiles(primary)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	pattern := filepath.Join(opts.TemplateDir, name+"*")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no template files matched %s", pattern)
	}

	if len(matches) > 1 {
		sort.Strings(matches)

		return nil, fmt.Errorf(
			"multiple template files matched for %s: %s",
			name,
			strings.Join(matches, ", "),
		)
	}

	return template.ParseFiles(matches[0])
}
