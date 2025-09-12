package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/algolia/docli/pkg/cmd/root"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func main() {
	out := "README.md"

	root := root.NewRootCmd()
	root.DisableAutoGenTag = true

	md := renderAll(root)

	if err := os.WriteFile(out, md, 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Wrote docs to %s\n", out)
}

func renderAll(root *cobra.Command) []byte {
	var buf bytes.Buffer

	walk(root, func(c *cobra.Command, depth int) {
		writeCommandSection(&buf, c, depth)
	})

	return buf.Bytes()
}

func walk(c *cobra.Command, visit func(*cobra.Command, int)) {
	var rec func(*cobra.Command, int)
	rec = func(cmd *cobra.Command, depth int) {
		if cmd.Hidden {
			return
		}

		visit(cmd, depth)

		children := cmd.Commands()
		sort.Slice(children, func(i, j int) bool {
			return children[i].Name() < children[j].Name()
		})

		for _, sc := range children {
			rec(sc, depth+1)
		}
	}
	rec(c, 1)
}

func writeCommandSection(buf *bytes.Buffer, c *cobra.Command, depth int) {
	// Heading level
	h := strings.Repeat("#", depth) // root -> ##, child -> ###, etc.
	fmt.Fprintf(buf, "%s `%s`\n\n", h, c.CommandPath())

	// Usage
	if c.Use != "" {
		fmt.Fprintf(buf, "```sh\n%s\n```\n\n", c.UseLine())
	}

	// Short and long descriptions
	if s := strings.TrimSpace(c.Short); s != "" {
		fmt.Fprintf(buf, "%s.\n\n", s)
	}

	if l := strings.TrimSpace(c.Long); l != "" && l != c.Short {
		fmt.Fprintf(buf, "%s\n\n", l)
	}

	// Aliases
	if len(c.Aliases) > 0 {
		fmt.Fprintf(buf, "**Aliases:** `%s`\n\n", strings.Join(c.Aliases, "`, `"))
	}

	// Examples
	if ex := strings.TrimSpace(c.Example); ex != "" {
		fmt.Fprintf(buf, "**Examples**\n\n```sh\n%s\n```\n\n", ex)
	}

	// Flags
	if c.Flags().HasFlags() {
		fmt.Fprintf(buf, "**Flags**\n\n")
		writeFlags(buf, c.Flags())
		// Somehow c.Flags() doesn't include the help flag
		writeFlags(buf, c.InheritedFlags())
		fmt.Fprint(buf, "\n")
	}

	// Subcommands
	sub := c.Commands()
	visible := sub[:0]

	for _, s := range sub {
		if !s.Hidden {
			visible = append(visible, s)
		}
	}

	if len(visible) > 0 {
		sort.Slice(visible, func(i, j int) bool { return visible[i].Name() < visible[j].Name() })

		if depth == 1 {
			fmt.Fprintf(buf, "**Commands:** %s\n\n", joinNames(visible))
		} else {
			fmt.Fprintf(buf, "**Subcommands:** %s\n\n", joinNames(visible))
		}
	}
}

func writeFlags(buf *bytes.Buffer, flags *pflag.FlagSet) {
	var lines []string

	flags.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}

		var signature strings.Builder

		signature.WriteString("`")

		sh := f.Shorthand
		if sh != "" {
			signature.WriteString("-")
			signature.WriteString(sh)
			signature.WriteString(", ")
		}

		signature.WriteString("--")
		signature.WriteString(f.Name)

		// Show value hint for non-bool flags
		if f.Value != nil && f.Value.Type() != "bool" {
			signature.WriteString(" ")
			signature.WriteString(valueHint(f))
		}

		signature.WriteString("`")

		usage := strings.TrimSpace(f.Usage)

		// Default value (omit when empty/false unless explicitly set)
		if strings.TrimSpace(f.DefValue) == "" || f.DefValue == "false" {
			lines = append(lines, fmt.Sprintf("- %s — %s", signature.String(), usage))
		} else {
			lines = append(
				lines,
				fmt.Sprintf("- %s — %s (default: `%s`)", signature.String(), usage, f.DefValue),
			)
		}
	})

	if len(lines) == 0 {
		return
	}

	for _, l := range lines {
		fmt.Fprintln(buf, l)
	}
}

func valueHint(f *pflag.Flag) string {
	t := f.Value.Type()
	switch t {
	case "stringSlice":
		return "string..."
	case "intSlice":
		return "int..."
	case "stringArray":
		return "string..."
	case "duration":
		return "duration"
	case "count":
		return "n"
	default:
		// For bool, we don't add a value (handled by caller); for others show <type>
		if t == "bool" {
			return ""
		}

		return t
	}
}

func joinNames(cmds []*cobra.Command) string {
	names := make([]string, 0, len(cmds))
	for _, c := range cmds {
		names = append(names, "`"+c.Name()+"`")
	}

	return strings.Join(names, ", ")
}
