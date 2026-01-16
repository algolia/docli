package output

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestNewRejectsVerboseQuiet(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().BoolP(FlagVerbose, "v", false, "verbose")
	cmd.Flags().BoolP(FlagQuiet, "q", false, "quiet")
	cmd.Flags().Bool(FlagDryRun, false, "dry run")

	if err := cmd.Flags().Set(FlagVerbose, "true"); err != nil {
		t.Fatalf("set verbose: %v", err)
	}

	if err := cmd.Flags().Set(FlagQuiet, "true"); err != nil {
		t.Fatalf("set quiet: %v", err)
	}

	_, err := New(cmd)
	if err == nil {
		t.Fatal("expected error when both verbose and quiet are set")
	}
}

func TestWriteFileDryRunSkipsCreation(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().BoolP(FlagVerbose, "v", false, "verbose")
	cmd.Flags().BoolP(FlagQuiet, "q", false, "quiet")
	cmd.Flags().Bool(FlagDryRun, false, "dry run")

	if err := cmd.Flags().Set(FlagDryRun, "true"); err != nil {
		t.Fatalf("set dry run: %v", err)
	}

	printer, err := New(cmd)
	if err != nil {
		t.Fatalf("new printer: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	err = printer.WriteFile(path, func(w io.Writer) error {
		_, err := w.Write([]byte("content"))

		return err
	})
	if err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, err := os.Stat(path); err == nil {
		t.Fatal("expected no file to be created in dry run")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected stat error: %v", err)
	}
}

func TestWriteFileWrapsWriteErrors(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().BoolP(FlagVerbose, "v", false, "verbose")
	cmd.Flags().BoolP(FlagQuiet, "q", false, "quiet")
	cmd.Flags().Bool(FlagDryRun, false, "dry run")

	printer, err := New(cmd)
	if err != nil {
		t.Fatalf("new printer: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	expected := errors.New("write failed")

	err = printer.WriteFile(path, func(w io.Writer) error {
		return expected
	})
	if err == nil {
		t.Fatal("expected error from write")
	}

	if !errors.Is(err, expected) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}
