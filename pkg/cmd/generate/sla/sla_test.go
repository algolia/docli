package sla

import (
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
				SlaStatus:   "eligible",
				SlaEndDate:  "",
			},
			"2.0": VersionInfo{
				ReleaseDate: "today",
				SlaStatus:   "eligible",
				SlaEndDate:  "tomorrow",
			},
		},
		"go": Version{
			"1.0": VersionInfo{
				ReleaseDate: "today",
				SlaStatus:   "eligible",
				SlaEndDate:  "",
			},
			"2.0": VersionInfo{
				ReleaseDate: "today",
				SlaStatus:   "eligible",
				SlaEndDate:  "tomorrow",
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
