package generate

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/algolia/docli/pkg/cmd/generate/cdn"
	"github.com/algolia/docli/pkg/cmd/generate/clients"
	"github.com/algolia/docli/pkg/cmd/generate/guides"
	"github.com/algolia/docli/pkg/cmd/generate/openapi"
	"github.com/algolia/docli/pkg/cmd/generate/sla"
	"github.com/algolia/docli/pkg/cmd/generate/snippets"
	"github.com/spf13/cobra"
)

func NewGenerateCmd() *cobra.Command {
	command := &cobra.Command{
		Use:     "generate",
		Aliases: []string{"gen", "g"},
		Short:   "Generate content from data files",
		Long: heredoc.Doc(`
			Each command reads data from a file,
			interpolates them into a template,
			and writes on or more MDX files.
			Most templates are built into the CLI,
			some can be provided at runtime.
			
			This is useful when running in CI whenever data files are updated.

			See the individual subcommands to learn what content you can generate.
		`),
	}

	command.AddCommand(clients.NewClientsCommand())
	command.AddCommand(openapi.NewOpenAPICommand())
	command.AddCommand(sla.NewSLACommand())
	command.AddCommand(snippets.NewSnippetsCommand())
	command.AddCommand(guides.NewGuidesCommand())
	command.AddCommand(cdn.NewCdnCommand())

	return command
}
