package validate

import (
	"fmt"
	"os"
	"path/filepath"
)

func ExistingFile(path, label string) error {
	if path == "" {
		return fmt.Errorf("%s is required", label)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s %q not found: %w", label, path, err)
	}

	if info.IsDir() {
		return fmt.Errorf("%s %q is a directory", label, path)
	}

	return nil
}

func ExistingDir(path, label string) error {
	if path == "" {
		return fmt.Errorf("%s is required", label)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s %q not found: %w", label, path, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%s %q is not a directory", label, path)
	}

	return nil
}

func OutputDir(path, label string) error {
	if path == "" {
		return fmt.Errorf("%s is required", label)
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("stat %s %q: %w", label, path, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%s %q is not a directory", label, path)
	}

	return nil
}

func OutputFile(path, label string) error {
	if path == "" {
		return fmt.Errorf("%s is required", label)
	}

	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return fmt.Errorf("%s %q is a directory", label, path)
		}

		return nil
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s %q: %w", label, path, err)
	}

	parent := filepath.Dir(path)
	if parent == "." || parent == "" {
		return nil
	}

	info, err = os.Stat(parent)
	if err != nil {
		return fmt.Errorf("%s directory %q not found: %w", label, parent, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%s directory %q is not a directory", label, parent)
	}

	return nil
}

func OutputFileDryRun(path, label string) error {
	if path == "" {
		return fmt.Errorf("%s is required", label)
	}

	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return fmt.Errorf("%s %q is a directory", label, path)
		}

		return nil
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s %q: %w", label, path, err)
	}

	return nil
}
