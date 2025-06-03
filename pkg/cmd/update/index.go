package update

import (
	"github.com/algolia/docli/pkg/cmd/update/cdn"
	"github.com/spf13/cobra"
)

func NewUpdateCmd() *cobra.Command {
	command := &cobra.Command{
		Use:     "update",
		Aliases: []string{"u"},
		Short:   "Update data for the docs",
	}

	command.AddCommand(cdn.NewCdnCommand())

	return command
}
