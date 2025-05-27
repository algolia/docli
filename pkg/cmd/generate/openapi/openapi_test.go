package openapi

import (
	"testing"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

func TestGetApiName(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"api.yaml", "api"},
		{"/some/path/to/file.yaml", "file"},
		{"searchstats.yaml", "analytics"},
	}

	for _, tt := range tests {
		got := getApiName(tt.input)
		if got != tt.expected {
			t.Errorf("got %q; want %q", got, tt.expected)
		}
	}
}

func TestToKebab(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"getUserData", "get-user-data"},
		{"SimpleTest", "simple-test"},
		{"test123Test", "test123-test"},
		{"API keys", "api-keys"},
	}

	for _, tt := range tests {
		got := toKebabCase(tt.input)
		if got != tt.expected {
			t.Errorf("got %q; want %q", got, tt.expected)
		}
	}
}

func TestOutputFilename(t *testing.T) {
	op := &v3.Operation{OperationId: "getUserData"}
	expected := "get-user-data.mdx"

	if got := outputFilename(op); got != expected {
		t.Errorf("got %q; want %q", got, expected)
	}
}

func TestOutputPath(t *testing.T) {
	op := &v3.Operation{Tags: []string{"UserData"}}
	prefix := "out/api"
	expected := "out/api/user-data"

	if got := outputPath(op, prefix); got != expected {
		t.Errorf("got %q; want %q", got, expected)
	}

	opNoTags := &v3.Operation{}
	expected = prefix

	if got := outputPath(opNoTags, prefix); got != expected {
		t.Errorf("got %q; want %q", got, expected)
	}
}
