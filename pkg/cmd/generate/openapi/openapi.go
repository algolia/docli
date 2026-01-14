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
	APIName         string
	InputFileName   string
	OutputDirectory string
}

// ExternalDocs holds an externalDocs reference.
type ExternalDocs struct {
	Description string
	URL         string
}

// OverviewData holds relevant data for the entire spec.
type OverviewData struct {
	Description      string
	OutputFilename   string
	OutputPath       string
	ShortDescription string
	Title            string
}

// OperationData holds data relevant to a single API operation stub file.
type OperationData struct {
	ACL            string
	APIPath        string
	ExternalDocs   ExternalDocs
	InputFilename  string
	OutputFilename string
	OutputPath     string
	RequiresAdmin  bool
	SeeAlso        bool
	Title          string
	Verb           string
}

//go:embed overview.mdx.tmpl
var overviewTemplate string

//go:embed stub.mdx.tmpl
var stubTemplate string

// NewOpenAPICommand returns a new instance of the `generate openapi` command.
func NewOpenAPICommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:     "openapi <spec>",
		Aliases: []string{"stubs"},
		Short:   "Generate MDX files for the HTTP API reference",
		Long: heredoc.Doc(`
			This command reads an OpenAPI 3 spec and generates one MDX file per API operation.
			Useful when adding new operations or changing operation summaries.
			It doesn't delete MDX files. If you remove or rename an operation,
			you need to update or delete its MDX file manually.
		`),
		Example: heredoc.Doc(`
  		# Run from root of algolia/docs-new
			docli gen stubs specs/search.yml -o doc/rest-api
    `),
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			opts.InputFileName = args[0]
			opts.APIName = utils.GetAPIName(opts.InputFileName)

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

	spec, err := utils.LoadSpec(specFile)
	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	overviewData, err := getAPIOverviewData(spec, opts)
	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	ovTmpl := template.Must(template.New("overview").Parse(overviewTemplate))

	err = writeOverviewData(overviewData, ovTmpl)
	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	opData, err := getAPIData(spec, opts)
	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	tmpl := template.Must(template.New("stub").Parse(stubTemplate))

	err = writeAPIData(opData, tmpl)
	if err != nil {
		log.Fatalf("Error: %e", err)
	}
}

// getAPIOverviewData generates MDX stub data for the API spec.
func getAPIOverviewData(
	doc *libopenapi.DocumentModel[v3.Document],
	opts *Options,
) (OverviewData, error) {
	result := OverviewData{
		OutputFilename:   fmt.Sprintf("%s.mdx", opts.APIName),
		OutputPath:       opts.OutputDirectory,
		Title:            doc.Model.Info.Title,
		ShortDescription: doc.Model.Info.Summary,
		Description:      doc.Model.Info.Description,
	}

	return result, nil
}

// getAPIData generates the MDX stub data for each OpenAPI operation in the spec.
func getAPIData(
	doc *libopenapi.DocumentModel[v3.Document],
	opts *Options,
) ([]OperationData, error) {
	var result []OperationData

	count := 0

	prefix := fmt.Sprintf("%s/%s", opts.OutputDirectory, opts.APIName)

	for pathPairs := doc.Model.Paths.PathItems.First(); pathPairs != nil; pathPairs = pathPairs.Next() {
		pathName := pathPairs.Key()
		// Ignore custom HTTP requests
		if pathName == "/{path}" {
			continue
		}

		pathItem := pathPairs.Value()

		for opPairs := pathItem.GetOperations().First(); opPairs != nil; opPairs = opPairs.Next() {
			op := opPairs.Value()

			acl, err := utils.GetACL(op)
			if err != nil {
				return nil, err
			}

			data := OperationData{
				ACL:            utils.AclToString(acl),
				APIPath:        pathName,
				InputFilename:  normalizePath(opts.InputFileName),
				OutputFilename: utils.GetOutputFilename(op),
				OutputPath:     prefix,
				RequiresAdmin:  false,
				Title:          strings.TrimSpace(op.Summary),
				Verb:           opPairs.Key(),
			}

			if data.ACL == "`admin`" {
				data.RequiresAdmin = true
			}

			if op.ExternalDocs != nil {
				desc := strings.TrimSpace(op.ExternalDocs.Description)
				data.ExternalDocs.Description = strings.TrimSuffix(desc, ".")
				data.ExternalDocs.URL = op.ExternalDocs.URL
			}

			if data.ExternalDocs.Description != "" && data.ExternalDocs.URL != "" {
				data.SeeAlso = true
			}

			result = append(result, data)
			count++
		}
	}

	fmt.Printf("Spec %s has %d operations.\n", opts.InputFileName, count)

	return result, nil
}

// writeOverviewData writes the API's overview data into an MDX file.
func writeOverviewData(data OverviewData, template *template.Template) error {
	err := os.MkdirAll(data.OutputPath, 0o700)
	if err != nil {
		return err
	}

	fullPath := filepath.Join(data.OutputPath, data.OutputFilename)

	out, err := os.Create(fullPath)
	if err != nil {
		return err
	}

	err = template.Execute(out, data)
	if err != nil {
		return err
	}

	return nil
}

// writeAPIData writes the OpenAPI data of a single operation to an MDX stub file.
func writeAPIData(data []OperationData, template *template.Template) error {
	for _, item := range data {
		err := os.MkdirAll(item.OutputPath, 0o700)
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
