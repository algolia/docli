package supportedllms

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/algolia/docli/pkg/output"
	"github.com/spf13/cobra"
)

// newTestPrinter builds a printer backed by discard buffers for tests.
func newTestPrinter(t *testing.T) *output.Printer {
	t.Helper()

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().BoolP(output.FlagVerbose, "v", false, "verbose")
	cmd.Flags().BoolP(output.FlagQuiet, "q", false, "quiet")
	cmd.Flags().Bool(output.FlagDryRun, false, "dry run")

	printer, err := output.New(cmd)
	if err != nil {
		t.Fatalf("new printer: %v", err)
	}

	return printer
}

func TestRenderSnippet(t *testing.T) {
	var sb bytes.Buffer

	// Order must be preserved verbatim, suffixes kept.
	models := []string{"claude-fable-5", "claude-opus-4-8", "claude-sonnet-4-5-20250929"}
	if err := renderSnippet(&sb, models); err != nil {
		t.Fatalf("renderSnippet: %v", err)
	}

	want := "- `claude-fable-5`\n- `claude-opus-4-8`\n- `claude-sonnet-4-5-20250929`\n"
	if got := sb.String(); got != want {
		t.Fatalf("renderSnippet:\n got %q\nwant %q", got, want)
	}
}

func TestRunGeneratesExpectedSnippets(t *testing.T) {
	body := `{
		"anthropic": ["claude-opus-4-8", "claude-sonnet-5"],
		"openai": ["gpt-5.6", "gpt-4.1"],
		"google_genai": ["gemini-3.5-flash"],
		"deepseek": ["deepseek-chat"],
		"azure_openai": ["gpt-5", "gpt-4.1"],
		"openai_compatible": []
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	dir := t.TempDir()
	t.Setenv(appIDEnv, "app")
	t.Setenv(apiKeyEnv, "key")

	opts := &Options{OutputDir: dir, URL: server.URL}
	if err := runCommand(context.Background(), opts, newTestPrinter(t)); err != nil {
		t.Fatalf("runCommand: %v", err)
	}

	// deepseek is generated; azure_openai and openai_compatible are not.
	wantFiles := map[string]string{
		"anthropic.mdx":    "- `claude-opus-4-8`\n- `claude-sonnet-5`\n",
		"openai.mdx":       "- `gpt-5.6`\n- `gpt-4.1`\n",
		"google_genai.mdx": "- `gemini-3.5-flash`\n",
		"deepseek.mdx":     "- `deepseek-chat`\n",
	}

	for name, want := range wantFiles {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("read %s: %v", name, err)

			continue
		}

		if string(got) != want {
			t.Errorf("%s:\n got %q\nwant %q", name, string(got), want)
		}
	}

	for _, name := range []string{"azure_openai.mdx", "openai_compatible.mdx"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("expected %s not to be generated", name)
		}
	}
}

func TestRunDryRunWritesNothing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"anthropic":["claude-opus-4-8"]}`))
	}))
	defer server.Close()

	dir := filepath.Join(t.TempDir(), "out")
	t.Setenv(appIDEnv, "app")
	t.Setenv(apiKeyEnv, "key")

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().BoolP(output.FlagVerbose, "v", false, "verbose")
	cmd.Flags().BoolP(output.FlagQuiet, "q", false, "quiet")
	cmd.Flags().Bool(output.FlagDryRun, false, "dry run")

	if err := cmd.Flags().Set(output.FlagDryRun, "true"); err != nil {
		t.Fatalf("set dry run: %v", err)
	}

	printer, err := output.New(cmd)
	if err != nil {
		t.Fatalf("new printer: %v", err)
	}

	opts := &Options{OutputDir: dir, URL: server.URL}
	if err := runCommand(context.Background(), opts, printer); err != nil {
		t.Fatalf("runCommand: %v", err)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("dry run should not create output directory")
	}
}

func TestValidateOptions(t *testing.T) {
	tests := []struct {
		name    string
		appID   string
		apiKey  string
		wantErr bool
	}{
		{name: "both set", appID: "app", apiKey: "key", wantErr: false},
		{name: "missing app id", appID: "", apiKey: "key", wantErr: true},
		{name: "missing api key", appID: "app", apiKey: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOptions(&Options{OutputDir: t.TempDir()}, tt.appID, tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateOptions err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	if err := os.WriteFile(
		envPath,
		[]byte("ALGOLIA_APPLICATION_ID=from-file\nALGOLIA_API_KEY=key-from-file\n"),
		0o600,
	); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	t.Setenv(appIDEnv, "")
	t.Setenv(apiKeyEnv, "")
	os.Unsetenv(appIDEnv)
	os.Unsetenv(apiKeyEnv)

	if err := loadEnvFile(envPath, true, newTestPrinter(t)); err != nil {
		t.Fatalf("loadEnvFile: %v", err)
	}

	if got := os.Getenv(appIDEnv); got != "from-file" {
		t.Errorf("%s = %q, want from-file", appIDEnv, got)
	}

	if got := os.Getenv(apiKeyEnv); got != "key-from-file" {
		t.Errorf("%s = %q, want key-from-file", apiKeyEnv, got)
	}
}

func TestLoadEnvFileEnvTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	if err := os.WriteFile(
		envPath,
		[]byte("ALGOLIA_APPLICATION_ID=from-file\n"),
		0o600,
	); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	t.Setenv(appIDEnv, "from-env")

	if err := loadEnvFile(envPath, true, newTestPrinter(t)); err != nil {
		t.Fatalf("loadEnvFile: %v", err)
	}

	if got := os.Getenv(appIDEnv); got != "from-env" {
		t.Errorf("%s = %q, want from-env (env must win over .env)", appIDEnv, got)
	}
}

func TestLoadEnvFileMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.env")

	if err := loadEnvFile(missing, false, newTestPrinter(t)); err != nil {
		t.Errorf("expected no error for missing optional env file, got %v", err)
	}

	if err := loadEnvFile(missing, true, newTestPrinter(t)); err == nil {
		t.Error("expected error for missing required env file")
	}
}

func TestFetchModels(t *testing.T) {
	const (
		appID  = "test-app"
		apiKey = "test-key"
	)

	var gotAppID, gotAPIKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAppID = r.Header.Get("x-algolia-application-id")
		gotAPIKey = r.Header.Get("x-algolia-api-key")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"openai":["gpt-4"],"anthropic":["claude-opus-4-8"]}`))
	}))
	defer server.Close()

	data, err := fetchModels(context.Background(), server.URL, appID, apiKey)
	if err != nil {
		t.Fatalf("fetchModels: %v", err)
	}

	if gotAppID != appID || gotAPIKey != apiKey {
		t.Fatalf("headers not forwarded: appID=%q apiKey=%q", gotAppID, gotAPIKey)
	}

	if !reflect.DeepEqual(data["openai"], []string{"gpt-4"}) {
		t.Fatalf("unexpected data: %+v", data)
	}
}

func TestFetchModelsNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := fetchModels(context.Background(), server.URL, "app", "key")
	if err == nil {
		t.Fatal("expected error on non-200 response")
	}
}
