package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/algolia/docli/pkg/cmd/root"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func main() {
	out := "README.md"

	usage, err := os.ReadFile("./internal/docs/usage.md")
	if err != nil {
		log.Fatalf("Can't find usage doc `internal/docs/usage.md`: %v", err)
	}

	root := root.NewRootCmd()
	root.DisableAutoGenTag = true

	// Append command reference to usage info
	usage = append(usage, renderAll(root)...)

	if err := os.WriteFile(out, usage, 0o644); err != nil {
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
	rec(c, 2)
}

func writeCommandSection(buf *bytes.Buffer, c *cobra.Command, depth int) {
	// Heading level
	h := strings.Repeat("#", depth)
	// Skip writing command name for root command
	if depth > 2 {
		fmt.Fprintf(buf, "%s `%s`\n\n", h, c.CommandPath())
	}

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
		// Somehow c.Flags() doesn't include the help tag
		fmt.Fprintf(buf, "**Flags**\n\n")

		allFlags := MergeFlags(c)
		writeFlags(buf, allFlags)
		fmt.Fprint(buf, "\n")
	}

	// Subcommands
	writeSubcommands(buf, c.Commands(), depth)
}

func MergeFlags(cmd *cobra.Command) *pflag.FlagSet {
	fs := pflag.NewFlagSet(cmd.Name(), pflag.ContinueOnError)

	// Add local flags
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		fs.AddFlag(f)
	})

	// Add inherited flags (skip duplicates)
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		if fs.Lookup(f.Name) == nil {
			fs.AddFlag(f)
		}
	})

	return fs
}

func writeSubcommands(buf *bytes.Buffer, commands [](*cobra.Command), depth int) {
	visible := commands[:0]

	for _, s := range commands {
		if !s.Hidden {
			visible = append(visible, s)
		}
	}

	if len(visible) > 0 {
		sort.Slice(visible, func(i, j int) bool { return visible[i].Name() < visible[j].Name() })

		if depth == 2 {
			fmt.Fprintf(buf, "**Commands:** %s\n\n", joinNames(visible))
		} else {
			fmt.Fprintf(buf, "**Subcommands:** %s\n\n", joinNames(visible))
		}
	}
}

func writeFlags(buf *bytes.Buffer, flags *pflag.FlagSet) {
	w := tabwriter.NewWriter(buf, 0, 0, 2, ' ', tabwriter.StripEscape)
	defer w.Flush()

	flags.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}

		signature := flagSignature(f)
		usage := strings.TrimSpace(f.Usage)

		// Append default value
		if f.DefValue != "false" && f.DefValue != "" {
			usage += fmt.Sprintf(" (default: `%s`)", f.DefValue)
		}

		fmt.Fprintf(w, "%s\t%s\n\n", signature, usage)
	})
}

func flagSignature(f *pflag.Flag) string {
	sh, name := f.Shorthand, f.Name
	typ := ""

	if f.Value != nil && f.Value.Type() != "bool" {
		typ = " " + f.Value.Type()
	}

	if sh != "" {
		// \xFF ... \xFF sections are not counted for alignment
		return fmt.Sprintf("\xFF`-%s, --%s%s`\xFF", sh, name, typ)
	}

	return fmt.Sprintf("\xFF`--%s%s`\xFF", name, typ)
}

func joinNames(cmds []*cobra.Command) string {
	names := make([]string, 0, len(cmds))
	for _, c := range cmds {
		names = append(names, "`"+c.Name()+"`")
	}

	return strings.Join(names, ", ")
}
