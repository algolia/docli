package root

import (
	"log"
	"strings"
	"text/template"

	"github.com/MakeNowJust/heredoc"
	"github.com/algolia/docli/pkg/cmd/generate"
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	// Register an indent function for using in the help template.
	cobra.AddTemplateFuncs(template.FuncMap{
		"indent": func(n int, s string) string {
			pad := strings.Repeat(" ", n)
			s = strings.TrimSpace(s)

			lines := strings.Split(s, "\n")
			for i, l := range lines {
				if strings.TrimSpace(l) != "" {
					lines[i] = pad + l
				}
			}

			return strings.Join(lines, "\n")
		},
	})

	// Define the root command
	cmd := &cobra.Command{
		Use:     "docli",
		Version: version,
		Short:   "Generate content for the Algolia docs on Mintlify",
		Long: heredoc.Doc(`
			Not all of Algolia's docs are handwritten.
			Some content is generated from data files.
			This CLI helps with that.
			
			See the individual commands to learn what you can do with it.
		`),
	}
	cmd.SetHelpTemplate(helpTemplate())
	// Capitalize help message to make it consistent
	cmd.PersistentFlags().BoolP("help", "h", false, "Help for this command")

	cmd.AddCommand(generate.NewGenerateCmd())

	return cmd
}

func Execute() {
	rootCmd := NewRootCmd()

	err := rootCmd.Execute()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func helpTemplate() string {
	return heredoc.Doc(`
		{{ .Short }}
		{{ if .Long }}
		{{ .Long | trimTrailingWhitespaces }}
		{{- end }}

		Usage:
		  {{ .UseLine }}
		{{- if .Aliases }}

		Aliases:
		  {{ .NameAndAliases }}
		{{- end }}
		{{- if .HasAvailableSubCommands }}

		Available commands:
		{{- range .Commands }}
		{{- if (or .IsAvailableCommand (eq .Name "help")) }}
    {{ rpad .Name .NamePadding }} {{ .Short }}
		{{- end }}
		{{- end }}
		{{- end }}
	  {{- if .HasExample }}

		Examples:
	 {{ indent 2 .Example | trimTrailingWhitespaces }}
		{{- end }}
	  {{- if .HasAvailableFlags }}

		Flags:
	 {{ .Flags.FlagUsages | trimTrailingWhitespaces }}
		{{- end }}
	 `)
}
