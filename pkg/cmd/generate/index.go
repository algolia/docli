package generate

import (
	"github.com/algolia/docli/pkg/cmd/generate/openapi"
	"github.com/spf13/cobra"
)

func NewGenerateCmd() *cobra.Command {
	command := &cobra.Command{
		Use:     "generate",
		Aliases: []string{"gen", "g"},
		Short:   "Generate API reference docs",
	}

	command.AddCommand(openapi.NewOpenAPICommand())

	return command
}
