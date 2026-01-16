package openapi

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/MakeNowJust/heredoc"
	"github.com/algolia/docli/pkg/cmd/generate/utils"
	"github.com/algolia/docli/pkg/output"
	"github.com/algolia/docli/pkg/validate"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.InputFileName = args[0]
			opts.APIName = utils.GetAPIName(opts.InputFileName)

			printer, err := output.New(cmd)
			if err != nil {
				return err
			}

			return runCommand(opts, printer)
		},
	}

	cmd.Flags().
		StringVarP(&opts.OutputDirectory, "output", "o", "out", "Output directory for generated MDX files")

	return cmd
}

// runCommand runs the `generate openapi` command.
func runCommand(opts *Options, printer *output.Printer) error {
	if err := validate.ExistingFile(opts.InputFileName, "spec file"); err != nil {
		return err
	}

	if err := validate.OutputDir(opts.OutputDirectory, "output directory"); err != nil {
		return err
	}

	specFile, err := os.ReadFile(opts.InputFileName)
	if err != nil {
		return fmt.Errorf("read spec file %s: %w", opts.InputFileName, err)
	}

	printer.Infof("Generating MDX stub files for spec: %s\n", opts.InputFileName)
	printer.Infof("Writing output in: %s\n", opts.OutputDirectory)

	spec, err := utils.LoadSpec(specFile)
	if err != nil {
		return fmt.Errorf("load spec %s: %w", opts.InputFileName, err)
	}

	overviewData, err := getAPIOverviewData(spec, opts)
	if err != nil {
		return fmt.Errorf("build overview data for %s: %w", opts.InputFileName, err)
	}

	ovTmpl := template.Must(template.New("overview").Parse(overviewTemplate))

	err = writeOverviewData(overviewData, ovTmpl, printer)
	if err != nil {
		return fmt.Errorf("write overview: %w", err)
	}

	opData, err := getAPIData(spec, opts)
	if err != nil {
		return fmt.Errorf("build operation data for %s: %w", opts.InputFileName, err)
	}

	printer.Verbosef("Spec %s has %d operations.\n", opts.InputFileName, len(opData))

	tmpl := template.Must(template.New("stub").Parse(stubTemplate))

	err = writeAPIData(opData, tmpl, printer)
	if err != nil {
		return fmt.Errorf("write operations: %w", err)
	}

	return nil
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
				return nil, fmt.Errorf("get ACL for %s %s: %w", opPairs.Key(), pathName, err)
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

	return result, nil
}

// writeOverviewData writes the API's overview data into an MDX file.
func writeOverviewData(
	data OverviewData,
	template *template.Template,
	printer *output.Printer,
) error {
	if !printer.IsDryRun() {
		if err := os.MkdirAll(data.OutputPath, 0o700); err != nil {
			return err
		}
	}

	fullPath := filepath.Join(data.OutputPath, data.OutputFilename)

	return printer.WriteFile(fullPath, func(w io.Writer) error {
		return template.Execute(w, data)
	})
}

// writeAPIData writes the OpenAPI data of a single operation to an MDX stub file.
func writeAPIData(
	data []OperationData,
	template *template.Template,
	printer *output.Printer,
) error {
	for _, item := range data {
		if !printer.IsDryRun() {
			if err := os.MkdirAll(item.OutputPath, 0o700); err != nil {
				return err
			}
		}

		fullPath := filepath.Join(item.OutputPath, item.OutputFilename)

		if err := printer.WriteFile(fullPath, func(w io.Writer) error {
			return template.Execute(w, item)
		}); err != nil {
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
