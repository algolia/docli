package sla

import (
	"bytes"
	"reflect"
	"testing"
)

func TestParseVersions(t *testing.T) {
	testData := `{
	  "csharp": {
	    "1.0": {"releaseDate": "today", "slaStatus": "eligible"},
			"2.0": {"releaseDate": "today", "slaStatus": "eligible", "slaEndDate": "tomorrow"}
	  },
	  "go": {
	    "1.0": {"releaseDate": "today", "slaStatus": "eligible"},
			"2.0": {"releaseDate": "today", "slaStatus": "eligible", "slaEndDate": "tomorrow"}
	  }
	}`

	expected := Clients{
		"csharp": Version{
			"1.0": VersionInfo{
				ReleaseDate: "today",
				SLAStatus:   "eligible",
				SLAEndDate:  "",
			},
			"2.0": VersionInfo{
				ReleaseDate: "today",
				SLAStatus:   "eligible",
				SLAEndDate:  "tomorrow",
			},
		},
		"go": Version{
			"1.0": VersionInfo{
				ReleaseDate: "today",
				SLAStatus:   "eligible",
				SLAEndDate:  "",
			},
			"2.0": VersionInfo{
				ReleaseDate: "today",
				SLAStatus:   "eligible",
				SLAEndDate:  "tomorrow",
			},
		},
	}

	got, err := parseVersions([]byte(testData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("parsed mismatch.\nGot: %#v\nExpected: %#v", got, expected)
	}
}

func TestRenderVersionsFile(t *testing.T) {
	tests := []struct {
		name     string
		input    []ClientEntry
		expected string
	}{
		{
			name: "multiple majors and patches",
			input: []ClientEntry{
				{
					Language: "csharp",
					Versions: []VersionEntry{
						{Version: "7.2.3"},
						{Version: "7.1.0"},
						{Version: "6.5.1"},
						{Version: "6.4.0"},
					},
				},
				{
					Language: "dart",
					Versions: []VersionEntry{
						{Version: "1.3.0"},
						{Version: "1.2.0"},
					},
				},
			},
			expected: `export const sdkVersions = {
  csharp: {
    v7: "7.2.3",
    v6: "6.5.1",
  },
  dart: {
    v1: "1.3.0",
  },
};
`,
		},
		{
			name: "single major only",
			input: []ClientEntry{
				{
					Language: "go",
					Versions: []VersionEntry{
						{Version: "2.0.0"},
						{Version: "2.0.0-beta"},
					},
				},
			},
			expected: `export const sdkVersions = {
  go: {
    v2: "2.0.0",
  },
};
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			renderVersionsFile(&buf, tt.input)

			if got := buf.String(); got != tt.expected {
				t.Errorf("renderVersionsFile() =\n%q\nwant\n%q", got, tt.expected)
			}
		})
	}
}
