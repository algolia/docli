package mcptools

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/algolia/docli/pkg/output"
	"github.com/spf13/cobra"
)

func newTestPrinter(t *testing.T, dryRun bool) *output.Printer {
	t.Helper()

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().BoolP(output.FlagVerbose, "v", false, "verbose")
	cmd.Flags().BoolP(output.FlagQuiet, "q", false, "quiet")
	cmd.Flags().Bool(output.FlagDryRun, dryRun, "dry run")

	printer, err := output.New(cmd)
	if err != nil {
		t.Fatalf("new printer: %v", err)
	}

	return printer
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

const toolsJSON = `{
  "tools": {
    "direct": [
      {"name": "algolia_call_read_tool", "group": "meta", "effect": "read", "description": "Run a tool"},
      {"name": "algolia_discover_tools", "group": "meta", "effect": "read", "description": "List tools"},
      {"name": "algolia_extra", "group": "meta", "effect": "read", "description": "Extra"}
    ],
    "catalog": [
      {"name": "algolia_get_ab_test", "group": "analytics", "effect": "read", "endpoints": ["abtesting/getABTest"]},
      {"name": "algolia_search", "group": "search", "effect": "read", "description": "Search\n  an index | now"}
    ],
    "chatgpt": [
      {"name": "search", "group": "chatgpt", "effect": "read", "description": "Search"}
    ]
  }
}`

func TestRenderTable(t *testing.T) {
	got := renderTable([]row{
		{tool: "**Search**"},
		{tool: "`a`", description: "Café  |\n pipe"},
	})

	want := header + "\n\n" +
		"| Tool       | Description  |\n" +
		"| ---------- | ------------ |\n" +
		"| **Search** |              |\n" +
		"| `a`        | Café \\| pipe |\n"

	if got != want {
		t.Fatalf("renderTable:\n got %q\nwant %q", got, want)
	}
}

func TestRenderSnippets(t *testing.T) {
	docs := t.TempDir()
	writeFile(t, filepath.Join(docs, "doc/rest-api/abtesting/get-ab-test.mdx"),
		"---\ntitle: \"Retrieve A/B test details\"\n---\n\nBody\n")

	tools := mustParse(t)

	snippets, err := renderSnippets(&tools, docs)
	if err != nil {
		t.Fatalf("renderSnippets: %v", err)
	}

	direct := snippets["direct.mdx"]
	discover := strings.Index(direct, "algolia_discover_tools")
	call := strings.Index(direct, "algolia_call_read_tool")
	extra := strings.Index(direct, "algolia_extra")

	if discover > call || call > extra {
		t.Errorf("direct tools out of order:\n%s", direct)
	}

	catalog := snippets["catalog.mdx"]

	for _, want := range []string{
		"**Search**",
		"Search an index \\| now",
		"**Analytics**",
		"[Retrieve A/B test details](/doc/rest-api/abtesting/get-ab-test)",
	} {
		if !strings.Contains(catalog, want) {
			t.Errorf("catalog missing %q:\n%s", want, catalog)
		}
	}

	if strings.Index(catalog, "**Search**") > strings.Index(catalog, "**Analytics**") {
		t.Errorf("catalog sections out of order:\n%s", catalog)
	}
}

func mustParse(t *testing.T) Tools {
	t.Helper()

	path := filepath.Join(t.TempDir(), "tools.json")
	writeFile(t, path, toolsJSON)

	tools, err := readTools(path)
	if err != nil {
		t.Fatalf("readTools: %v", err)
	}

	return *tools
}

func TestRenderSnippetsProblems(t *testing.T) {
	tools := Tools{
		Direct: []Tool{{Name: "w", Group: "meta", Effect: "write", Description: "Writes"}},
		Catalog: []Tool{
			{Name: "no_endpoint", Group: "search", Effect: "read"},
			{Name: "many", Group: "search", Effect: "read", Endpoints: []string{"a/b", "a/c"}},
			{
				Name:      "missing_page",
				Group:     "search",
				Effect:    "read",
				Endpoints: []string{"search/nope"},
			},
			{Name: "odd_group", Group: "weird", Effect: "read", Description: "x"},
		},
	}

	_, err := renderSnippets(&tools, t.TempDir())
	if err == nil {
		t.Fatal("expected an error")
	}

	for _, want := range []string{
		"w changes data",
		"No tool for the ChatGPT table",
		"no_endpoint has no endpoint",
		"many has several endpoints",
		"missing_page: /doc/rest-api/search/nope doesn't exist",
		`odd_group is in group "weird"`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q:\n%v", want, err)
		}
	}
}

func TestPageTitle(t *testing.T) {
	dir := t.TempDir()

	tests := map[string]string{
		"---\ntitle: Plain title\n---\n":               "Plain title",
		"---\ntitle: 'Single'\n---\n":                  "Single",
		"---\nother: x\n---\n":                         "",
		"no frontmatter\ntitle: nope\n":                "",
		"---\r\ntitle: \"CRLF\"\r\n---\r\n":            "CRLF",
		"---\ndescription: d\ntitle: After\n---\nbody": "After",
	}

	for content, want := range tests {
		path := filepath.Join(dir, "page.mdx")
		writeFile(t, path, content)

		if got := pageTitle(path); got != want {
			t.Errorf("pageTitle(%q) = %q, want %q", content, got, want)
		}
	}

	if got := pageTitle(filepath.Join(dir, "missing.mdx")); got != "" {
		t.Errorf("pageTitle(missing) = %q, want empty", got)
	}
}

func newRunFixture(t *testing.T) (opts *Options, out string) {
	t.Helper()

	docs := t.TempDir()
	out = filepath.Join(t.TempDir(), "snippets")
	toolsFile := filepath.Join(t.TempDir(), "tools.json")

	writeFile(t, toolsFile, toolsJSON)
	writeFile(
		t,
		filepath.Join(docs, "doc/rest-api/abtesting/get-ab-test.mdx"),
		"---\ntitle: A/B\n---\n",
	)

	return &Options{OutputDir: out, ToolsFile: toolsFile, DocsDir: docs}, out
}

func TestRunDryRunDoesNotWrite(t *testing.T) {
	opts, out := newRunFixture(t)

	if err := runCommand(opts, newTestPrinter(t, true)); err != nil {
		t.Fatalf("dry run: %v", err)
	}

	if _, err := os.Stat(out); err == nil {
		t.Fatal("--dry-run must not create the output directory")
	}
}

func TestRunWritesSnippets(t *testing.T) {
	opts, out := newRunFixture(t)

	if err := runCommand(opts, newTestPrinter(t, false)); err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, name := range []string{"direct.mdx", "catalog.mdx", "chatgpt.mdx"} {
		if !strings.HasPrefix(readFile(t, filepath.Join(out, name)), header) {
			t.Errorf("%s: missing header", name)
		}
	}

	// A second run leaves up-to-date files alone and rewrites only the edited one.
	writeFile(t, filepath.Join(out, "chatgpt.mdx"), "changed")

	stale, err := staleSnippets(out, mustRender(t, opts))
	if err != nil || len(stale) != 1 || filepath.Base(stale[0]) != "chatgpt.mdx" {
		t.Fatalf("want only chatgpt.mdx stale, got %v (err %v)", stale, err)
	}
}

func mustRender(t *testing.T, opts *Options) map[string]string {
	t.Helper()

	tools, err := readTools(opts.ToolsFile)
	if err != nil {
		t.Fatal(err)
	}

	snippets, err := renderSnippets(tools, opts.DocsDir)
	if err != nil {
		t.Fatal(err)
	}

	return snippets
}

func TestRunDefaultsToolsFileToOutputDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tools.json"), toolsJSON)
	writeFile(
		t,
		filepath.Join(dir, "doc/rest-api/abtesting/get-ab-test.mdx"),
		"---\ntitle: A/B\n---\n",
	)

	opts := &Options{OutputDir: dir, DocsDir: dir}
	if err := runCommand(opts, newTestPrinter(t, false)); err != nil {
		t.Fatalf("run: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "direct.mdx")); err != nil {
		t.Fatalf("direct.mdx not written: %v", err)
	}
}
