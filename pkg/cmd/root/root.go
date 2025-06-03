package root

import (
	"log"

	"github.com/algolia/docli/pkg/cmd/generate"
	"github.com/algolia/docli/pkg/cmd/update"
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "docli",
		Version: "0.0.1",
		Short:   "Manage the Algolia documentation",
	}

	return cmd
}

func Execute() {
	rootCmd := NewRootCmd()
	rootCmd.AddCommand(generate.NewGenerateCmd())
	rootCmd.AddCommand(update.NewUpdateCmd())

	err := rootCmd.Execute()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}
