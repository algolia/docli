package main

import (
	"reflect"
	"testing"
)

func TestLatestVersionTagUsesTagsMergedIntoHEAD(t *testing.T) {
	t.Parallel()

	original := runGitOutputFn

	t.Cleanup(func() {
		runGitOutputFn = original
	})

	var calls [][]string

	runGitOutputFn = func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))

		return "v2.0.0\nv1.5.0\n", nil
	}

	tag, err := latestVersionTag()
	if err != nil {
		t.Fatalf("latestVersionTag() error = %v", err)
	}

	if tag != "v2.0.0" {
		t.Fatalf("latestVersionTag() = %q, want %q", tag, "v2.0.0")
	}

	want := []string{"tag", "--merged", "HEAD", "--list", "v*", "--sort=-v:refname"}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], want) {
		t.Fatalf("git args = %#v, want %#v", calls, want)
	}
}

func TestDetectBumpIgnoresBreakingChangeInProse(t *testing.T) {
	t.Parallel()

	original := runGitOutputFn

	t.Cleanup(func() {
		runGitOutputFn = original
	})

	runGitOutputFn = func(args ...string) (string, error) {
		switch args[1] {
		case "--format=%s":
			return "fix: update docs\n", nil
		case "--format=%B":
			return "This is not a BREAKING CHANGE for users.\n", nil
		default:
			return "", nil
		}
	}

	bump, err := detectBump("v1.0.0..HEAD")
	if err != nil {
		t.Fatalf("detectBump() error = %v", err)
	}

	if bump != "patch" {
		t.Fatalf("detectBump() = %q, want %q", bump, "patch")
	}
}

func TestDetectBumpMatchesBreakingChangeTrailer(t *testing.T) {
	t.Parallel()

	original := runGitOutputFn

	t.Cleanup(func() {
		runGitOutputFn = original
	})

	runGitOutputFn = func(args ...string) (string, error) {
		switch args[1] {
		case "--format=%s":
			return "fix: update docs\n", nil
		case "--format=%B":
			return "Some context.\n\nBREAKING CHANGE: remove legacy endpoint\n", nil
		default:
			return "", nil
		}
	}

	bump, err := detectBump("v1.0.0..HEAD")
	if err != nil {
		t.Fatalf("detectBump() error = %v", err)
	}

	if bump != "major" {
		t.Fatalf("detectBump() = %q, want %q", bump, "major")
	}
}
