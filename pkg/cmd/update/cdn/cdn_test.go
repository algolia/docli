package cdn

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestReadData(t *testing.T) {
	type want struct {
		packages []Package
		err      bool
	}

	tests := []struct {
		name     string
		yamlBody string
		want     want
	}{
		{
			name: "valid YAML with one package",
			yamlBody: `---
- file: /foo.js
  name: foo
`,
			want: want{
				packages: []Package{
					{
						File:        "/foo.js",
						Name:        "foo",
						PackageName: "foo",
					},
				},
				err: false,
			},
		},
		{
			name: "handle file name without slash",
			yamlBody: `---
- file: foo.js
  name: foo
`,
			want: want{
				packages: []Package{
					{
						File:        "/foo.js",
						Name:        "foo",
						PackageName: "foo",
					},
				},
				err: false,
			},
		},
		{
			name:     "nonexistent file",
			yamlBody: "",
			want: want{
				packages: nil,
				err:      true,
			},
		},
		{
			name: "invalid YAML",
			yamlBody: `---
- file: "missing-quote
  name: "bad-yaml"
`,
			want: want{
				packages: nil,
				err:      true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			filename := filepath.Join(dir, "data.yaml")

			if tt.yamlBody != "" {
				if err := os.WriteFile(filename, []byte(tt.yamlBody), 0o644); err != nil {
					t.Fatalf("can't write temp file: %v", err)
				}
			} else {
				filename = filepath.Join(dir, "does-not-exist.yaml")
			}

			got, err := readData(filename)
			if tt.want.err {
				if err == nil {
					t.Fatalf("expected an error, got %v", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			if len(got) != len(tt.want.packages) {
				t.Fatalf("expected %d packages, got %d", len(tt.want.packages), len(got))
			}

			for i := range got {
				wantedPkg := tt.want.packages[i]
				gotPkg := got[i]

				if gotPkg != wantedPkg {
					t.Errorf(
						"package #%d mismatch:\n expected: %+v\ngot: %+v",
						i,
						wantedPkg,
						gotPkg,
					)
				}
			}
		})
	}
}

func TestGetLatestVersion(t *testing.T) {
	fakeData := struct {
		Tags     map[string]string `json:"tags"`
		Versions []string          `json:"versions"`
	}{
		Tags:     map[string]string{"latest": "1.2.3"},
		Versions: []string{"1.0.0", "1.2.3"},
	}
	payload, _ := json.Marshal(fakeData)

	// Fake a server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(payload)
	}))
	defer server.Close()

	pkg := &Package{PackageName: "foo"}
	if err := getLatestVersion(server.URL, pkg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "1.2.3"
	if pkg.Version != want {
		t.Errorf("got Version=%q; want %q", pkg.Version, want)
	}
}

func TestGetIncludeLinksDefaultInclude(t *testing.T) {
	fakeData := struct {
		Default string `json:"default"`
		Files   []struct {
			Name string `json:"name"`
			Hash string `json:"hash"`
		} `json:"files"`
	}{
		Default: "/index.js",
		Files: []struct {
			Name string `json:"name"`
			Hash string `json:"hash"`
		}{
			{Name: "/index.js", Hash: "HASH_INDEX"},
			{Name: "/other.js", Hash: "HASH_OTHER"},
		},
	}
	payload, _ := json.Marshal(fakeData)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(payload)
	}))
	defer server.Close()

	pkg := &Package{
		PackageName: "foo",
		Version:     "1.2.3",
		Name:        "snippet1",
	}
	if err := getIncludeLinks(server.URL, pkg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After calling, pkg.File should be "/index.js"
	if pkg.File != "/index.js" {
		t.Errorf("got File=%q; want %q", pkg.File, "/index.js")
	}

	// Integrity should match the hash for "/index.js"
	if pkg.Integrity != "HASH_INDEX" {
		t.Errorf("got Integrity=%q; want %q", pkg.Integrity, "HASH_INDEX")
	}

	// Src should be formatted as JSDELIVR_CDN_URL + "/foo@1.2.3/index.js"
	expectedSrc := fmt.Sprintf(
		"%s/%s@%s%s",
		JSDELIVR_CDN_URL,
		pkg.PackageName,
		pkg.Version,
		pkg.File,
	)
	if pkg.Src != expectedSrc {
		t.Errorf("got Src=%q; want %q", pkg.Src, expectedSrc)
	}
}

func TestGetIncludeLinksCustomFile(t *testing.T) {
	fakeData := struct {
		Default string `json:"default"`
		Files   []struct {
			Name string `json:"name"`
			Hash string `json:"hash"`
		} `json:"files"`
	}{
		Default: "/index.js",
		Files: []struct {
			Name string `json:"name"`
			Hash string `json:"hash"`
		}{
			{Name: "/index.js", Hash: "HASH_INDEX"},
			{Name: "/other.js", Hash: "HASH_OTHER"},
		},
	}
	payload, _ := json.Marshal(fakeData)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(payload)
	}))
	defer server.Close()

	// Set pkg.File explicitly to "/other.js"
	pkg := &Package{
		PackageName: "bar",
		File:        "/other.js",
		Name:        "snippet2",
	}
	if err := getIncludeLinks(server.URL, pkg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should remain "/other.js"
	if pkg.File != "/other.js" {
		t.Errorf("got File=%q; want %q", pkg.File, "/other.js")
	}

	// Integrity should match the hash for "/other.js"
	if pkg.Integrity != "HASH_OTHER" {
		t.Errorf("got Integrity=%q; want %q", pkg.Integrity, "HASH_OTHER")
	}

	// Src should be formatted correctly
	expectedSrc := fmt.Sprintf(
		"%s/%s@%s%s",
		JSDELIVR_CDN_URL,
		pkg.PackageName,
		pkg.Version,
		pkg.File,
	)
	if pkg.Src != expectedSrc {
		t.Errorf("got Src=%q; want %q", pkg.Src, expectedSrc)
	}
}

func TestGetTemplateSuccess(t *testing.T) {
	dir := t.TempDir()

	p := Package{Name: "foo"}
	opts := &Options{TemplateDir: dir}

	// Write a valid template file whose name matches "foo*"
	templateFilename := filepath.Join(dir, "foo_example.tmpl")

	content := `{{define "test"}}Hello, {{.}}{{end}}`
	if err := os.WriteFile(templateFilename, []byte(content), 0o644); err != nil {
		t.Fatalf("unable to write template file: %v", err)
	}

	tmpl, err := getTemplate(p, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After parsing, the named template "test" should exist
	if lookup := tmpl.Lookup("test"); lookup == nil {
		t.Errorf("expected to find defined template 'test', but got nil")
	}
}

func TestGetTemplateNoMatchingFiles(t *testing.T) {
	// Empty temp dir → no files match "bar*"
	dir := t.TempDir()
	p := Package{Name: "bar"}
	opts := &Options{TemplateDir: dir}

	_, err := getTemplate(p, opts)
	if err == nil {
		t.Fatal("expected error when no files match pattern, got nil")
	}
}

func TestGetTemplateInvalidTemplateSyntax(t *testing.T) {
	dir := t.TempDir()
	p := Package{Name: "baz"}
	opts := &Options{TemplateDir: dir}

	// Write a file named "baz_invalid.tmpl" with broken syntax
	templateFilename := filepath.Join(dir, "baz_invalid.tmpl")
	// Missing closing braces → invalid syntax
	invalidContent := `{{define "bad"}}Unclosed...`
	if err := os.WriteFile(templateFilename, []byte(invalidContent), 0o644); err != nil {
		t.Fatalf("unable to write invalid template file: %v", err)
	}

	_, err := getTemplate(p, opts)
	if err == nil {
		t.Fatal("expected parse error for invalid template syntax, got nil")
	}
}

func TestGetTemplateMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	p := Package{Name: "multi"}
	opts := &Options{TemplateDir: dir}

	// Create two files matching "multi*"
	f1 := filepath.Join(dir, "multi_one.tmpl")
	f2 := filepath.Join(dir, "multi_two.tmpl")

	if err := os.WriteFile(f1, []byte(`{{define "one"}}One{{end}}`), 0o644); err != nil {
		t.Fatalf("unable to write first template: %v", err)
	}

	if err := os.WriteFile(f2, []byte(`{{define "two"}}Two{{end}}`), 0o644); err != nil {
		t.Fatalf("unable to write second template: %v", err)
	}

	tmpl, err := getTemplate(p, opts)
	if err != nil {
		t.Fatalf("unexpected error parsing multiple files: %v", err)
	}

	if tmpl.Lookup("one") == nil {
		t.Errorf("expected to find defined template 'one', but got nil")
	}

	if tmpl.Lookup("two") == nil {
		t.Errorf("expected to find defined template 'two', but got nil")
	}
}
