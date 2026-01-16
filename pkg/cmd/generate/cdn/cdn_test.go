package cdn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestReadData(t *testing.T) {
	type want struct {
		packages []PackageSpec
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
				packages: []PackageSpec{
					{
						File: "/foo.js",
						Name: "foo",
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
				packages: []PackageSpec{
					{
						File: "foo.js",
						Name: "foo",
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

			got, err := readCDNDataFile(filename)
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

func TestResolverResolveDefaultFile(t *testing.T) {
	npmPayload, _ := json.Marshal(packageMetadata{
		DistTags: map[string]string{"latest": "1.2.3"},
		Versions: map[string]packageVersion{
			"1.2.3": {JSDelivr: "index.js"},
		},
	})

	npmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(npmPayload)
	}))
	defer npmServer.Close()

	cdnPayload, _ := json.Marshal(struct {
		Files []struct {
			Name string `json:"name"`
			Hash string `json:"hash"`
		} `json:"files"`
	}{
		Files: []struct {
			Name string `json:"name"`
			Hash string `json:"hash"`
		}{
			{Name: "/index.js", Hash: "HASH_INDEX"},
		},
	})

	cdnServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(cdnPayload)
	}))
	defer cdnServer.Close()

	resolver := NewResolver(nil)
	resolver.npmRegistryURL = npmServer.URL
	resolver.jsDelivrDataURL = cdnServer.URL
	resolver.jsDelivrCdnURL = "https://cdn.example.test"

	pkg := PackageSpec{
		PackageName: "foo",
		Name:        "snippet1",
	}

	resolved, err := resolver.Resolve(pkg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Version != "1.2.3" {
		t.Errorf("got Version=%q; want %q", resolved.Version, "1.2.3")
	}

	if resolved.File != "/index.js" {
		t.Errorf("got File=%q; want %q", resolved.File, "/index.js")
	}

	if resolved.Integrity != "HASH_INDEX" {
		t.Errorf("got Integrity=%q; want %q", resolved.Integrity, "HASH_INDEX")
	}

	expectedSrc := fmt.Sprintf(
		"%s/%s@%s%s",
		resolver.jsDelivrCdnURL,
		resolved.PackageName,
		resolved.Version,
		resolved.File,
	)
	if resolved.Src != expectedSrc {
		t.Errorf("got Src=%q; want %q", resolved.Src, expectedSrc)
	}
}

func TestResolverResolveCustomFile(t *testing.T) {
	npmPayload, _ := json.Marshal(packageMetadata{
		DistTags: map[string]string{"latest": "1.2.3"},
		Versions: map[string]packageVersion{
			"1.2.3": {JSDelivr: "index.js"},
		},
	})

	npmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(npmPayload)
	}))
	defer npmServer.Close()

	cdnPayload, _ := json.Marshal(struct {
		Files []struct {
			Name string `json:"name"`
			Hash string `json:"hash"`
		} `json:"files"`
	}{
		Files: []struct {
			Name string `json:"name"`
			Hash string `json:"hash"`
		}{
			{Name: "/other.js", Hash: "HASH_OTHER"},
		},
	})

	cdnServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(cdnPayload)
	}))
	defer cdnServer.Close()

	resolver := NewResolver(nil)
	resolver.npmRegistryURL = npmServer.URL
	resolver.jsDelivrDataURL = cdnServer.URL
	resolver.jsDelivrCdnURL = "https://cdn.example.test"

	pkg := PackageSpec{
		PackageName: "bar",
		File:        "/other.js",
		Name:        "snippet2",
	}

	resolved, err := resolver.Resolve(pkg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.File != "/other.js" {
		t.Errorf("got File=%q; want %q", resolved.File, "/other.js")
	}

	if resolved.Integrity != "HASH_OTHER" {
		t.Errorf("got Integrity=%q; want %q", resolved.Integrity, "HASH_OTHER")
	}
}

func TestResolverResolveMetadataNotFound(t *testing.T) {
	npmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer npmServer.Close()

	resolver := NewResolver(nil)
	resolver.npmRegistryURL = npmServer.URL
	resolver.jsDelivrDataURL = npmServer.URL

	pkg := PackageSpec{
		PackageName: "typo",
		Name:        "snippet404",
	}

	err := func() error {
		_, err := resolver.Resolve(pkg)

		return err
	}()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected 404 error, got %v", err)
	}
}

func TestResolverResolveWithContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sawCanceled := false
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if errors.Is(req.Context().Err(), context.Canceled) {
				sawCanceled = true

				return nil, req.Context().Err()
			}

			return nil, fmt.Errorf("expected canceled context")
		}),
	}

	resolver := NewResolver(client)
	resolver.npmRegistryURL = "https://example.test"

	_, err := resolver.ResolveWithContext(ctx, PackageSpec{
		PackageName: "foo",
		Name:        "snippet",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}

	if !sawCanceled {
		t.Fatal("expected request context to be canceled")
	}
}

func TestResolverCachesMetadataAndCDNFiles(t *testing.T) {
	npmPayload, _ := json.Marshal(packageMetadata{
		DistTags: map[string]string{"latest": "1.0.0"},
		Versions: map[string]packageVersion{
			"1.0.0": {JSDelivr: "index.js"},
		},
	})

	cdnPayload, _ := json.Marshal(struct {
		Files []struct {
			Name string `json:"name"`
			Hash string `json:"hash"`
		} `json:"files"`
	}{
		Files: []struct {
			Name string `json:"name"`
			Hash string `json:"hash"`
		}{
			{Name: "/index.js", Hash: "HASH_INDEX"},
		},
	})

	npmURL := "https://registry.test"
	cdnURL := "https://data.test"
	npmCalls := 0
	cdnCalls := 0

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.HasPrefix(req.URL.String(), npmURL):
				npmCalls++

				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(string(npmPayload))),
				}, nil
			case strings.HasPrefix(req.URL.String(), cdnURL):
				cdnCalls++

				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(string(cdnPayload))),
				}, nil
			default:
				return nil, fmt.Errorf("unexpected URL: %s", req.URL.String())
			}
		}),
	}

	resolver := NewResolver(client)
	resolver.npmRegistryURL = npmURL
	resolver.jsDelivrDataURL = cdnURL
	resolver.jsDelivrCdnURL = "https://cdn.example.test"

	spec := PackageSpec{
		PackageName: "foo",
		Name:        "snippet",
	}

	if _, err := resolver.Resolve(spec); err != nil {
		t.Fatalf("first resolve failed: %v", err)
	}

	if _, err := resolver.Resolve(spec); err != nil {
		t.Fatalf("second resolve failed: %v", err)
	}

	if npmCalls != 1 {
		t.Fatalf("expected 1 npm request, got %d", npmCalls)
	}

	if cdnCalls != 1 {
		t.Fatalf("expected 1 CDN request, got %d", cdnCalls)
	}
}

func TestGetTemplateSuccess(t *testing.T) {
	dir := t.TempDir()

	opts := &Options{TemplateDir: dir}

	// Write a valid template file with the canonical name.
	templateFilename := filepath.Join(dir, "foo.mdx.tmpl")

	content := `{{define "test"}}Hello, {{.}}{{end}}`
	if err := os.WriteFile(templateFilename, []byte(content), 0o644); err != nil {
		t.Fatalf("unable to write template file: %v", err)
	}

	tmpl, err := getTemplate("foo", opts)
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
	opts := &Options{TemplateDir: dir}

	_, err := getTemplate("bar", opts)
	if err == nil {
		t.Fatal("expected error when no files match pattern, got nil")
	}
}

func TestGetTemplateInvalidTemplateSyntax(t *testing.T) {
	dir := t.TempDir()
	opts := &Options{TemplateDir: dir}

	// Write a file named "baz.mdx.tmpl" with broken syntax
	templateFilename := filepath.Join(dir, "baz.mdx.tmpl")
	// Missing closing braces → invalid syntax
	invalidContent := `{{define "bad"}}Unclosed...`
	if err := os.WriteFile(templateFilename, []byte(invalidContent), 0o644); err != nil {
		t.Fatalf("unable to write invalid template file: %v", err)
	}

	_, err := getTemplate("baz", opts)
	if err == nil {
		t.Fatal("expected parse error for invalid template syntax, got nil")
	}
}

func TestGetTemplateMultipleFiles(t *testing.T) {
	dir := t.TempDir()
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

	_, err := getTemplate("multi", opts)
	if err == nil {
		t.Fatal("expected error when multiple files match pattern, got nil")
	}
}
