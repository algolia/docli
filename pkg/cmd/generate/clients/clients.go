package clients

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/MakeNowJust/heredoc"
	"github.com/algolia/docli/pkg/cmd/generate/utils"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/spf13/cobra"
)

// Options represents configuration options and CLI flags for this command.
type Options struct {
	ApiName         string
	InputFilename   string
	OutputDirectory string
}

// OperationData represents relevant information about an API operation.
type OperationData struct {
	Acl              string
	Summary          string
	ShortDescription string
	Description      string
	RequiresAdmin    bool
	InputFilename    string
	OutputFilename   string
	OutputPath       string
}

//go:embed method.mdx.tmpl
var methodTemplate string

func NewClientsCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:     "clients",
		Aliases: []string{"c"},
		Short:   "Generate API client reference pages from the OpenAPI spec",
		Long: heredoc.Doc(`
			This command reads an OpenAPI 3 spec file and generates one MDX file per operation.
		`),
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			opts.InputFilename = args[0]
			opts.ApiName = utils.GetApiName(opts.InputFilename)
			runCommand(opts)
		},
	}

	cmd.Flags().
		StringVarP(&opts.OutputDirectory, "output", "o", "out", "Output directory for generated MDX files")

	return cmd
}

func runCommand(opts *Options) {
	specFile, err := os.ReadFile(opts.InputFilename)
	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	fmt.Printf("Generating API client references for spec: %s\n", opts.InputFilename)
	fmt.Printf("Writing output in: %s\n", opts.OutputDirectory)

	spec, err := utils.LoadSpec(specFile)
	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	opData, err := getApiData(spec, opts)
	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	tmpl := template.Must(template.New("method").Parse(methodTemplate))

	writeApiData(opData, tmpl)
}

// getApiData reads the OpenAPI spec and parses the operation data.
func getApiData(
	doc *libopenapi.DocumentModel[v3.Document],
	opts *Options,
) ([]OperationData, error) {
	var result []OperationData

	count := 0

	prefix := fmt.Sprintf("%s/%s", opts.OutputDirectory, opts.ApiName)

	for pathPairs := doc.Model.Paths.PathItems.First(); pathPairs != nil; pathPairs = pathPairs.Next() {
		pathName := pathPairs.Key()
		// Ignore custom HTTP requests
		if pathName == "/{path}" {
			continue
		}

		pathItem := pathPairs.Value()

		for opPairs := pathItem.GetOperations().First(); opPairs != nil; opPairs = opPairs.Next() {
			op := opPairs.Value()

			acl, err := utils.GetAcl(op)
			if err != nil {
				return nil, err
			}

			data := OperationData{
				Acl:            utils.AclToString(acl),
				Summary:        op.Summary,
				OutputFilename: utils.GetOutputFilename(op),
				OutputPath:     utils.GetOutputPath(op, prefix),
				RequiresAdmin:  false,
			}

			if data.Acl == "`admin`" {
				data.RequiresAdmin = true
			}

			result = append(result, data)
			count++
		}
	}

	fmt.Printf("Spec %s has %d operations.\n", opts.InputFilename, count)

	return result, nil
}

// writeApiData writes the OpenAPI data to MDX files.
func writeApiData(data []OperationData, template *template.Template) error {
	for _, item := range data {
		if err := os.MkdirAll(item.OutputPath, 0o755); err != nil {
			return err
		}

		fullPath := filepath.Join(item.OutputPath, item.OutputFilename)

		out, err := os.Create(fullPath)
		if err != nil {
			return err
		}

		err = template.Execute(out, item)
		if err != nil {
			return err
		}
	}

	return nil
}
