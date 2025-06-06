package generate

import (
	"github.com/algolia/docli/pkg/cmd/generate/openapi"
	"github.com/algolia/docli/pkg/cmd/generate/sla"
	"github.com/spf13/cobra"
)

func NewGenerateCmd() *cobra.Command {
	command := &cobra.Command{
		Use:     "generate",
		Aliases: []string{"gen", "g"},
		Short:   "Generate API reference docs",
	}

	command.AddCommand(openapi.NewOpenApiCommand())
	command.AddCommand(sla.NewSlaCommand())

	return command
}
