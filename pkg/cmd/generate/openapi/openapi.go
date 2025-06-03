package openapi

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/MakeNowJust/heredoc"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
	RequiresAdmin  bool
	ApiPath        string
	InputFilename  string
	OutputFilename string
	OutputPath     string
	Verb           string
}

const stubTemplate = `---
openapi: /{{.InputFilename }} {{ .Verb }} {{ .ApiPath }}
---
{{- if .RequiresAdmin }}

**Requires admin API key**
{{- else if .Acl }}

**Required ACL:** {{ .Acl }}
{{- end }}
`

// NewOpenAPICommand returns a new instance of the `generate openapi` command.
func NewOpenAPICommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:     "openapi",
		Aliases: []string{"stubs"},
		Short:   "Generate HTTP API reference files from an OpenAPI spec",
		Long: heredoc.Doc(`
			This command reads an OpenAPI 3 spec file and generates one MDX file per operation.
			The command groups the operations into subdirectories by tags.
		`),
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			opts.InputFileName = args[0]
			opts.ApiName = getApiName(opts.InputFileName)

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

	spec, err := loadSpec(opts)
	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	opData, err := apiStubData(spec, opts)
	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	tmpl := template.Must(template.New("stub").Parse(stubTemplate))

	if err != nil {
		log.Fatalf("Error: %e", err)
	}

	writeApiData(opData, tmpl)
}

// loadSpec parses the file as OpenAPI 3 spec and returns the data model.
func loadSpec(opts *Options) (*libopenapi.DocumentModel[v3.Document], error) {
	doc, err := libopenapi.NewDocument(opts.SpecFile)
	if err != nil {
		return nil, err
	}

	docModel, errors := doc.BuildV3Model()
	if len(errors) > 0 {
		for i := range errors {
			fmt.Printf("error: %e\n", errors[i])
		}

		return nil, fmt.Errorf("cannot parse spec: %d errors.", len(errors))
	}

	return docModel, nil
}

// apiStubData generates the MDX stub data for each OpenAPI operation in the spec.
func apiStubData(
	doc *libopenapi.DocumentModel[v3.Document],
	opts *Options,
) ([]OperationData, error) {
	var result []OperationData
	count := 0

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

			acl, err := getAcl(op)
			if err != nil {
				return nil, err
			}

			data := OperationData{
				Acl:            strings.Join(acl, ","),
				ApiPath:        pathName,
				InputFilename:  strings.TrimPrefix(opts.InputFileName, "/"),
				OutputFilename: outputFilename(op),
				OutputPath:     outputPath(op, prefix),
				RequiresAdmin:  false,
				Verb:           opPairs.Key(),
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

// outputPath returns the output path for the MDX file for the given operation.
func outputPath(op *v3.Operation, prefix string) string {
	if len(op.Tags) > 0 {
		return fmt.Sprintf("%s/%s", prefix, toKebabCase(op.Tags[0]))
	}

	return fmt.Sprintf("%s", prefix)
}

// outputFilename generates the filename from the operationId.
func outputFilename(op *v3.Operation) string {
	return fmt.Sprintf("%s.mdx", toKebabCase(op.OperationId))
}

// toKebabCase turns a string into kebab-case.
func toKebabCase(s string) string {
	matchFirstCap := regexp.MustCompile(`(.)([A-Z][a-z]+)`)
	matchAllCap := regexp.MustCompile(`([a-z0-9])([A-Z])`)

	out := matchFirstCap.ReplaceAllString(s, `${1}-${2}`)
	out = matchAllCap.ReplaceAllString(out, `${1}-${2}`)
	out = regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(out, `-`)
	out = strings.Trim(out, `-`)

	return strings.ToLower(out)
}

// getAcl returns the ACL required to perform the given operation.
func getAcl(op *v3.Operation) ([]string, error) {
	node, ok := op.Extensions.Get("x-acl")

	// Operations can be without ACL
	if !ok {
		return nil, nil
	}

	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("expected a sequence node, got kind %d", node.Kind)
	}

	var result []string

	for _, child := range node.Content {
		if child.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("expected scalar nodes in sequence, got kind %d", child.Kind)
		}

		result = append(result, child.Value)
	}

	return result, nil
}

// getApiName returns the name of the YAML file without extension as API name.
func getApiName(path string) string {
	// Have to make an exception for the Analytics API
	base := filepath.Base(strings.ReplaceAll(path, "searchstats", "analytics"))

	return strings.TrimSuffix(base, filepath.Ext(base))
}
