package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "preview the next tag without creating it")

	flag.Parse()

	if err := run(*dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(dryRun bool) error {
	if err := ensureClean(); err != nil {
		return err
	}

	latestTag, err := latestVersionTag()
	if err != nil {
		return err
	}

	baseVersion := "0.0.0"
	rangeSpec := "HEAD"

	if latestTag != "" {
		if !strings.HasPrefix(latestTag, "v") {
			return fmt.Errorf("latest tag %q does not start with v", latestTag)
		}

		baseVersion = strings.TrimPrefix(latestTag, "v")
		rangeSpec = fmt.Sprintf("%s..HEAD", latestTag)
	}

	if err := ensureCommits(rangeSpec, latestTag); err != nil {
		return err
	}

	bump, err := detectBump(rangeSpec)
	if err != nil {
		return err
	}

	newTag, err := nextTag(baseVersion, bump)
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Printf("Dry run: would create tag %s\n", newTag)
		fmt.Printf("To push: git push origin %s\n", newTag)

		return nil
	}

	if err := runGit("tag", "-a", newTag, "-m", newTag); err != nil {
		return err
	}

	fmt.Printf("Created tag %s\n", newTag)
	fmt.Printf("To push: git push origin %s\n", newTag)

	return nil
}

func ensureClean() error {
	status, err := runGitOutput("status", "--porcelain")
	if err != nil {
		return err
	}

	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("working tree is dirty; commit or stash changes before tagging")
	}

	return nil
}

func latestVersionTag() (string, error) {
	out, err := runGitOutput("tag", "-l", "v*", "--sort=-v:refname")
	if err != nil {
		return "", err
	}

	lines := splitLines(out)
	if len(lines) == 0 {
		return "", nil
	}

	return lines[0], nil
}

func ensureCommits(rangeSpec, latestTag string) error {
	out, err := runGitOutput("log", "--format=%s", rangeSpec)
	if err != nil {
		return err
	}

	if strings.TrimSpace(out) == "" {
		if latestTag == "" {
			return fmt.Errorf("no commits found to tag")
		}

		return fmt.Errorf("no commits found since %s", latestTag)
	}

	return nil
}

func detectBump(rangeSpec string) (string, error) {
	subjects, err := runGitOutput("log", "--format=%s", rangeSpec)
	if err != nil {
		return "", err
	}

	bodies, err := runGitOutput("log", "--format=%B", rangeSpec)
	if err != nil {
		return "", err
	}

	breakingHeader := regexp.MustCompile(`^[a-zA-Z]+(\([^)]+\))?!:`)
	featHeader := regexp.MustCompile(`^feat(\([^)]+\))?:`)

	for _, subject := range splitLines(subjects) {
		if breakingHeader.MatchString(subject) {
			return "major", nil
		}
	}

	if strings.Contains(bodies, "BREAKING CHANGE") {
		return "major", nil
	}

	for _, subject := range splitLines(subjects) {
		if featHeader.MatchString(subject) {
			return "minor", nil
		}
	}

	return "patch", nil
}

func nextTag(baseVersion, bump string) (string, error) {
	major, minor, patch, err := parseVersion(baseVersion)
	if err != nil {
		return "", err
	}

	switch bump {
	case "major":
		major++
		minor = 0
		patch = 0
	case "minor":
		minor++
		patch = 0
	case "patch":
		patch++
	default:
		return "", fmt.Errorf("unknown bump type %q", bump)
	}

	return fmt.Sprintf("v%d.%d.%d", major, minor, patch), nil
}

func parseVersion(version string) (int, int, int, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("version %q must be in x.y.z format", version)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version %q: %w", parts[0], err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minor version %q: %w", parts[1], err)
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid patch version %q: %w", parts[2], err)
	}

	return major, minor, patch, nil
}

func runGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}

	return nil
}

func runGitOutput(args ...string) (string, error) {
	var out bytes.Buffer

	var errBuf bytes.Buffer

	cmd := exec.Command("git", args...)
	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg != "" {
			return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), msg)
		}

		return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}

	return out.String(), nil
}

func splitLines(input string) []string {
	lines := strings.Split(strings.TrimSpace(input), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}

	return lines
}
