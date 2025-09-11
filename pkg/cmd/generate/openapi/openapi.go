package openapi

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/MakeNowJust/heredoc"
	"github.com/algolia/docli/pkg/cmd/generate/utils"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/spf13/cobra"
)

// Options represents the options and flags for this command.
type Options struct {
	ApiName         string
	InputFileName   string
	OutputDirectory string
	SpecFile        []byte
}

// OperationData holds data relevant to a single API operation stub file.
type OperationData struct {
	Acl            string
	ApiPath        string
	InputFilename  string
	OutputFilename string
	OutputPath     string
	RequiresAdmin  bool
	Title          string
	Verb           string
}

//go:embed stub.mdx.tmpl
var stubTemplate string

// NewOpenApiCommand returns a new instance of the `generate openapi` command.
func NewOpenApiCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:     "openapi",
		Aliases: []string{"stubs"},
		Short:   "Generate HTTP API reference files from an OpenAPI spec",
		Long: heredoc.Doc(`
			This command reads an OpenAPI 3 spec file and generates one MDX file per operation.
			The command groups the operations into subdirectories by tags.
		`),
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			opts.InputFileName = args[0]
			opts.ApiName = utils.GetApiName(opts.InputFileName)

			runCommand(opts)
		},
	}

	cmd.Flags().
		StringVarP(&opts.OutputDirectory, "output", "o", "out", "Output directory for generated MDX files")

	return cmd
}

// runCommand runs the `generate openapi` command.
func runCommand(opts *Options) {
	specFile, err := os.ReadFile(opts.InputFileName)
	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	fmt.Printf("Generating MDX stub files for spec: %s\n", opts.InputFileName)
	fmt.Printf("Writing output in: %s\n", opts.OutputDirectory)

	opts.SpecFile = specFile

	spec, err := utils.LoadSpec(opts.SpecFile)
	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	opData, err := getApiData(spec, opts)
	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	tmpl := template.Must(template.New("stub").Parse(stubTemplate))

	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	writeApiData(opData, tmpl)
}

// getApiData generates the MDX stub data for each OpenAPI operation in the spec.
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
				ApiPath:        pathName,
				InputFilename:  normalizePath(opts.InputFileName),
				OutputFilename: utils.GetOutputFilename(op),
				OutputPath:     utils.GetOutputPath(op, prefix),
				RequiresAdmin:  false,
				Title:          strings.TrimSpace(op.Summary),
				Verb:           opPairs.Key(),
			}

			if data.Acl == "`admin`" {
				data.RequiresAdmin = true
			}

			result = append(result, data)
			count++
		}
	}

	fmt.Printf("Spec %s has %d operations.\n", opts.InputFileName, count)

	return result, nil
}

// writeApiData writes the OpenAPI data of a single operation to an MDX stub file.
func writeApiData(data []OperationData, template *template.Template) error {
	for _, item := range data {
		err := os.MkdirAll(item.OutputPath, 0o755)
		if err != nil {
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

// normalizePath strips any leading character from the input string and returns it with a leading slash.
func normalizePath(input string) string {
	input = strings.TrimPrefix(input, "./")
	input = strings.TrimPrefix(input, "/")

	return "/" + input
}
