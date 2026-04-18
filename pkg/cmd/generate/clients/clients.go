package clients

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

//go:embed method.mdx.tmpl
var methodTemplate string

func NewClientsCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:     "clients",
		Aliases: []string{"c"},
		Short:   "Generate MDX files for the API client method references",
		Long: heredoc.Doc(`
			This command reads an OpenAPI 3 spec file and generates one MDX file per operation.
			It writes an API reference with usage information specific to API clients,
			which may follow different conventions depending on the programming language used.
			This command doesn't delete MDX files. If you remove or rename an operation,
			you need to update or delete its MDX file manually.
		`),
		Example: heredoc.Doc(`
			# Run from root of algolia/docs-new
			docli gen clients specs/search.yml -o doc/libraries/sdk/methods
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.InputFilename = args[0]
			opts.APIName = utils.GetAPIName(opts.InputFilename)

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

func runCommand(opts *Options, printer *output.Printer) error {
	if err := validate.ExistingFile(opts.InputFilename, "spec file"); err != nil {
		return err
	}

	if err := validate.OutputDir(opts.OutputDirectory, "output directory"); err != nil {
		return err
	}

	specFile, err := os.ReadFile(opts.InputFilename)
	if err != nil {
		return fmt.Errorf("read spec file %s: %w", opts.InputFilename, err)
	}

	printer.Infof("Generating API client references for spec: %s\n", opts.InputFilename)
	printer.Infof("Writing output in: %s\n", opts.OutputDirectory)

	spec, err := utils.LoadSpec(specFile)
	if err != nil {
		return fmt.Errorf("load spec %s: %w", opts.InputFilename, err)
	}

	opData, err := getAPIData(spec, opts)
	if err != nil {
		return fmt.Errorf("parse spec %s: %w", opts.InputFilename, err)
	}

	printer.Verbosef("Spec %s has %d operations.\n", opts.InputFilename, len(opData))

	tmpl := template.Must(template.New("method").Funcs(template.FuncMap{
		"frontmatterString": utils.QuoteFrontmatterString,
		"mintFieldType":     mintFieldType,
		"renderParamFields": renderParamFields,
		"renderResponses":   renderResponses,
		"trim":              strings.TrimSpace,
	}).Parse(methodTemplate))

	if err := writeAPIData(opData, tmpl, printer); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}

// getAPIData reads the OpenAPI spec and parses the operation data.
func getAPIData(
	doc *libopenapi.DocumentModel[v3.Document],
	opts *Options,
) ([]OperationData, error) {
	var result []OperationData

	if doc == nil || doc.Model.Paths == nil {
		return nil, nil
	}

	prefix := filepath.Join(opts.OutputDirectory, opts.APIName)

	for pathPairs := doc.Model.Paths.PathItems.First(); pathPairs != nil; pathPairs = pathPairs.Next() {
		pathName := pathPairs.Key()
		if pathName == "/{path}" {
			continue
		}

		pathItem := pathPairs.Value()
		for opPairs := pathItem.GetOperations().First(); opPairs != nil; opPairs = opPairs.Next() {
			data, err := buildClientOperationData(
				opPairs.Key(),
				pathName,
				pathItem,
				opPairs.Value(),
				opts,
				prefix,
			)
			if err != nil {
				return nil, err
			}

			result = append(result, data)
		}
	}

	return result, nil
}

func buildClientOperationData(
	verb string,
	pathName string,
	pathItem *v3.PathItem,
	op *v3.Operation,
	opts *Options,
	prefix string,
) (OperationData, error) {
	acl, err := utils.GetACL(op)
	if err != nil {
		return OperationData{}, fmt.Errorf("get ACL for %s %s: %w", verb, pathName, err)
	}

	operationID, err := utils.RequireOperationID(op.OperationId, verb, pathName)
	if err != nil {
		return OperationData{}, err
	}

	short, long := utils.SplitDescription(op.Description)
	short = utils.StripMarkdown(short)

	params, err := getParameters(pathItem, op)
	if err != nil {
		return OperationData{}, fmt.Errorf("get parameters for %s %s: %w", verb, pathName, err)
	}

	responses, err := getResponses(op)
	if err != nil {
		return OperationData{}, fmt.Errorf("get responses for %s %s: %w", verb, pathName, err)
	}

	data := OperationData{
		ACL:              utils.AclToString(acl),
		APIName:          opts.APIName,
		CodeSamples:      getCodeSamples(op),
		Deprecated:       boolOrFalse(op.Deprecated),
		Description:      long,
		OutputFilename:   utils.GetOutputFilenameForOperationID(operationID),
		OutputPath:       prefix,
		OperationIDs:     utils.OperationIDVariants(operationID),
		OperationIDKebab: utils.ToKebabCase(operationID),
		Params:           sortParameters(pruneParameters(params)),
		Responses:        sortOperationResponses(responses),
		ShortDescription: short,
		Summary:          op.Summary,
	}

	if data.ACL == "`admin`" {
		data.RequiresAdmin = true
	}

	applyExternalDocs(&data, op)

	return data, nil
}

func applyExternalDocs(data *OperationData, op *v3.Operation) {
	if data == nil || op == nil || op.ExternalDocs == nil {
		return
	}

	desc := strings.TrimSpace(op.ExternalDocs.Description)
	data.ExternalDocs.Description = strings.TrimSuffix(desc, ".")
	data.ExternalDocs.URL = op.ExternalDocs.URL
	data.SeeAlso = data.ExternalDocs.Description != "" && data.ExternalDocs.URL != ""
}

func writeAPIData(data []OperationData, tmpl *template.Template, printer *output.Printer) error {
	for _, item := range data {
		if !printer.IsDryRun() {
			if err := os.MkdirAll(item.OutputPath, 0o700); err != nil {
				return err
			}
		}

		fullPath := filepath.Join(item.OutputPath, item.OutputFilename)
		if err := printer.WriteFile(fullPath, func(w io.Writer) error {
			return tmpl.Execute(w, item)
		}); err != nil {
			return err
		}
	}

	return nil
}
