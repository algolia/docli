package validate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")

	if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := ExistingFile(path, "data file"); err != nil {
		t.Fatalf("expected existing file to pass: %v", err)
	}

	if err := ExistingFile(dir, "data file"); err == nil {
		t.Fatal("expected directory to be rejected as file")
	}
}

func TestOutputFileValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	if err := OutputFile(path, "output file"); err != nil {
		t.Fatalf("expected output file in existing dir to pass: %v", err)
	}

	missingParent := filepath.Join(dir, "missing", "out.txt")
	if err := OutputFile(missingParent, "output file"); err == nil {
		t.Fatal("expected missing parent directory to error")
	}

	if err := OutputFileDryRun(missingParent, "output file"); err != nil {
		t.Fatalf("expected dry-run output file to skip parent check: %v", err)
	}
}

func TestOutputDirValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")

	if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := OutputDir(path, "output directory"); err == nil {
		t.Fatal("expected output directory validation to reject files")
	}

	missing := filepath.Join(dir, "missing")
	if err := OutputDir(missing, "output directory"); err != nil {
		t.Fatalf("expected missing output dir to pass: %v", err)
	}

	if err := ExistingDir(missing, "output directory"); err == nil {
		t.Fatal("expected missing dir to fail existing dir check")
	}

	if err := ExistingDir(path, "output directory"); err == nil {
		t.Fatal("expected file to fail existing dir check")
	}

	if err := ExistingDir(dir, "output directory"); err != nil {
		t.Fatalf("expected existing dir to pass: %v", err)
	}

	if err := OutputDir("", "output directory"); err == nil {
		t.Fatal("expected empty output dir to error")
	}

	if err := ExistingDir("", "output directory"); err == nil {
		t.Fatal("expected empty existing dir to error")
	}
}

func TestOutputFileRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := OutputFile(dir, "output file"); err == nil {
		t.Fatal("expected output file to reject directory path")
	}

	if err := OutputFileDryRun(dir, "output file"); err == nil {
		t.Fatal("expected dry-run output file to reject directory path")
	}

	if err := OutputFile("", "output file"); err == nil {
		t.Fatal("expected empty output file to error")
	}

	if err := OutputFileDryRun("", "output file"); err == nil {
		t.Fatal("expected empty dry-run output file to error")
	}
}

func TestExistingFileEmptyPath(t *testing.T) {
	if err := ExistingFile("", "data file"); err == nil {
		t.Fatal("expected empty existing file to error")
	}
}
