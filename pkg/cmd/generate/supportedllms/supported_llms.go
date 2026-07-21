package supportedllms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/algolia/docli/pkg/output"
	"github.com/algolia/docli/pkg/validate"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

const (
	appIDEnv       = "ALGOLIA_APPLICATION_ID"
	apiKeyEnv      = "ALGOLIA_API_KEY"
	modelsPath     = "/agent-studio/1/providers/models"
	defaultTimeout = 10 * time.Second
)

// generatedProviders lists the provider slugs for which a model snippet is
// generated, in output order. Providers not listed here are skipped on purpose:
// azure_openai documents "any deployed model" rather than a fixed list, and
// openai_compatible has no fixed model list.
var generatedProviders = []string{
	"anthropic",
	"openai",
	"google_genai",
	"deepseek",
}

// Options represents the options and flags for this command.
type Options struct {
	OutputDir string
	URL       string
	// EnvFile is the path to a .env file with the credentials.
	EnvFile string
	// EnvFileRequired is true when the user explicitly set --env-file,
	// which turns a missing file into an error instead of a silent skip.
	EnvFileRequired bool
}

// ProviderModels is the API response: provider slug to list of model IDs.
type ProviderModels map[string][]string

// NewCommand returns a new instance of the `generate supported-llms` command.
func NewCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "supported-llms",
		Short: "Generate snippets of supported LLMs per provider",
		Long: heredoc.Doc(`
			This command fetches the supported LLMs per provider from the
			Agent Studio API and generates one MDX snippet file per provider,
			each listing the supported model IDs.

			The snippets are meant to be included in the hand-written
			LLM providers guide so the model lists stay up to date.
			A snippet is generated for the following providers:
			anthropic, openai, google_genai, and deepseek.
			azure_openai and openai_compatible are skipped because they
			don't have a fixed list of models.

			The Algolia credentials must be provided through the
			ALGOLIA_APPLICATION_ID and ALGOLIA_API_KEY environment variables,
			or through a .env file (default: .env) with those variables.
			Existing environment variables take precedence over the .env file.
		`),
		Example: heredoc.Doc(`
			# Run from the root of algolia/docs-new
			ALGOLIA_APPLICATION_ID=… ALGOLIA_API_KEY=… \
			docli gen supported-llms -o snippets/agent-studio
		`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			printer, err := output.New(cmd)
			if err != nil {
				return err
			}

			opts.EnvFileRequired = cmd.Flags().Changed("env-file")

			return runCommand(cmd.Context(), opts, printer)
		},
	}

	cmd.Flags().
		StringVarP(&opts.OutputDir, "output", "o", ".", "Output directory for the generated snippet files")
	cmd.Flags().
		StringVar(&opts.URL, "url", "", "Override the API URL (defaults to the Agent Studio models endpoint)")
	cmd.Flags().
		StringVar(&opts.EnvFile, "env-file", ".env", "Path to a .env file with the Algolia credentials")

	return cmd
}

func runCommand(ctx context.Context, opts *Options, printer *output.Printer) error {
	if err := loadEnvFile(opts.EnvFile, opts.EnvFileRequired, printer); err != nil {
		return err
	}

	appID := os.Getenv(appIDEnv)
	apiKey := os.Getenv(apiKeyEnv)

	if err := validateOptions(opts, appID, apiKey); err != nil {
		return err
	}

	requestURL := opts.URL
	if requestURL == "" {
		requestURL = fmt.Sprintf("https://%s.algolia.net%s", appID, modelsPath)
	}

	if err := validateRequestURL(requestURL); err != nil {
		return err
	}

	printer.Infof("Fetching supported LLMs from: %s\n", requestURL)

	data, err := fetchModels(ctx, requestURL, appID, apiKey)
	if err != nil {
		return fmt.Errorf("fetch supported LLMs: %w", err)
	}

	warnUngeneratedProviders(data, printer)

	if !printer.IsDryRun() {
		if err = os.MkdirAll(opts.OutputDir, 0o700); err != nil {
			return fmt.Errorf("create output directory %s: %w", opts.OutputDir, err)
		}
	}

	for _, slug := range generatedProviders {
		if err = writeSnippet(opts.OutputDir, slug, data[slug], printer); err != nil {
			return err
		}
	}

	return nil
}

// loadEnvFile loads credentials from a .env file into the environment.
// Existing environment variables are not overridden. A missing file is only
// an error when the user explicitly requested it with --env-file.
func loadEnvFile(path string, required bool, printer *output.Printer) error {
	if path == "" {
		return nil
	}

	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) && !required {
			printer.Verbosef("No .env file at %s, using environment variables\n", path)

			return nil
		}

		return fmt.Errorf("read .env file %s: %w", path, err)
	}

	if err := godotenv.Load(path); err != nil {
		return fmt.Errorf("load .env file %s: %w", path, err)
	}

	printer.Verbosef("Loaded credentials from %s\n", path)

	return nil
}

func validateOptions(opts *Options, appID, apiKey string) error {
	if appID == "" {
		return fmt.Errorf("environment variable %s is required", appIDEnv)
	}

	if apiKey == "" {
		return fmt.Errorf("environment variable %s is required", apiKeyEnv)
	}

	return validate.OutputDir(opts.OutputDir, "output directory")
}

// validateRequestURL restricts requests to the known Agent Studio host to
// guard against SSRF. Loopback addresses are allowed so the command can be
// pointed at a local test server through --url.
func validateRequestURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid API URL %q: %w", raw, err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("API URL %q must use http or https", raw)
	}

	host := parsed.Hostname()

	if host == "localhost" {
		return nil
	}

	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}

	if parsed.Scheme != "https" {
		return fmt.Errorf("API URL %q must use https", raw)
	}

	if host != "algolia.net" && !strings.HasSuffix(host, ".algolia.net") {
		return fmt.Errorf("API URL host %q is not an algolia.net host", host)
	}

	return nil
}

// fetchModels requests the supported models per provider from the API.
// The URL is validated by validateRequestURL before this call.
func fetchModels(ctx context.Context, requestURL, appID, apiKey string) (ProviderModels, error) {
	// #nosec G107 -- requestURL is validated by validateRequestURL and
	// constrained to an algolia.net host (or loopback for tests).
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("x-algolia-application-id", appID)
	req.Header.Set("x-algolia-api-key", apiKey)

	client := &http.Client{Timeout: defaultTimeout}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request to %s failed with status %s", requestURL, res.Status)
	}

	var data ProviderModels
	if err = json.NewDecoder(res.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}

// warnUngeneratedProviders logs any provider returned by the API that isn't in
// the generated set, so a newly supported provider doesn't go unnoticed.
func warnUngeneratedProviders(data ProviderModels, printer *output.Printer) {
	generated := make(map[string]struct{}, len(generatedProviders))
	for _, slug := range generatedProviders {
		generated[slug] = struct{}{}
	}

	extra := make([]string, 0)

	for slug, models := range data {
		if _, ok := generated[slug]; ok {
			continue
		}

		if len(models) == 0 {
			continue
		}

		extra = append(extra, slug)
	}

	sort.Strings(extra)

	for _, slug := range extra {
		printer.Infof("Provider %q has models but no snippet is generated for it\n", slug)
	}
}

// writeSnippet writes the model snippet for a single provider. Providers with
// no models are skipped with a warning.
func writeSnippet(
	outputDir, slug string,
	models []string,
	printer *output.Printer,
) error {
	if len(models) == 0 {
		printer.Infof("No models for provider %q, skipping snippet\n", slug)

		return nil
	}

	out := filepath.Join(outputDir, slug+".mdx")

	if err := printer.WriteFile(out, func(w io.Writer) error {
		return renderSnippet(w, models)
	}); err != nil {
		return fmt.Errorf("write snippet for %s: %w", slug, err)
	}

	printer.Infof("Wrote %d models for %q to %s\n", len(models), slug, out)

	return nil
}

// renderSnippet writes the model IDs as a Markdown bullet list of inline code
// spans, preserving the order returned by the API.
func renderSnippet(w io.Writer, models []string) error {
	for _, model := range models {
		if _, err := fmt.Fprintf(w, "- `%s`\n", model); err != nil {
			return err
		}
	}

	return nil
}
