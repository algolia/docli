package apiclients

import (
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
	Acl              string
	ApiPath          string
	Description      string
	InputFilename    string
	OutputFilename   string
	OutputPath       string
	RequiresAdmin    bool
	ShortDescription string
	Summary          string
	Verb             string
}

const methodTemplate = `---
title: {{ .Summary }}
description: {{ .ShortDescription }}
---
{{- if .RequiresAdmin }}

**Requires admin API key**
{{- else if .Acl }}

**Required ACL:** {{ .Acl }}
{{- end }}

{{ .Description }}
`

// NewApiClientsCommand returns a new instance of the `generate openapi` command.
func NewApiClientsCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:     "apiclients",
		Aliases: []string{"sdk", "apiclient", "sdks"},
		Short:   "Generate method reference pages for the API clients",
		Long: heredoc.Doc(`
			This command generates the MDX files for the method references of the API clients.
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
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("Generating MDX files for spec: %s\n", opts.InputFileName)
	fmt.Printf("Writing output in: %s\n", opts.OutputDirectory)

	opts.SpecFile = specFile

	spec, err := utils.LoadSpec(opts.SpecFile)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	opData, err := apiOpData(spec, opts)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	tmpl := template.Must(template.New("method").Parse(methodTemplate))

	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	writeApiData(opData, tmpl)
}

// apiOpData generates the data for each OpenAPI operation in the spec.
func apiOpData(
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

			short, long := splitDescription(op.Description)

			data := OperationData{
				Acl:              utils.AclToString(acl),
				InputFilename:    strings.TrimPrefix(opts.InputFileName, "/"),
				OutputFilename:   utils.GetOutputFilename(op),
				OutputPath:       utils.GetOutputPath(op, prefix),
				RequiresAdmin:    false,
				Summary:          op.Summary,
				Description:      long,
				ShortDescription: short,
			}

			if data.Acl == "admin" {
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

// splitDescription splits the API description into two strings.
// The first sentence is the short description, the rest is the long description.
func splitDescription(description string) (string, string) {
	description = strings.TrimSpace(description)

	// Split by empty line
	parts := strings.SplitN(description, "\n\n", 2)
	if len(parts) > 1 && strings.TrimSpace(parts[0]) != "" {
		short := strings.TrimSpace(parts[0])
		long := strings.TrimSpace(parts[1])

		return short, long
	}

	// No empty line found: split after first period
	if idx := strings.Index(description, "."); idx != -1 {
		short := strings.TrimSpace(description[:idx+1])
		long := strings.TrimSpace(description[idx+1:])

		return short, long
	}

	// No period: can't split
	return description, description
}
