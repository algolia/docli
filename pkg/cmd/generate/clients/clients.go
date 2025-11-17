package clients

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
	"github.com/algolia/docli/pkg/dictionary"
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

// ExternalDocs holds an externalDocs reference.
type ExternalDocs struct {
	Description string
	Url         string
}

// OperationData represents relevant information about an API operation.
type OperationData struct {
	Acl              string
	ApiName          string
	CodeSamples      []CodeSample
	Deprecated       bool
	Description      string
	ExternalDocs     ExternalDocs
	InputFilename    string
	OutputFilename   string
	OutputPath       string
	OperationIdKebab string
	Params           []Parameter
	RequestBody      RequestBody
	RequiresAdmin    bool
	SeeAlso          bool
	ShortDescription string
	Summary          string
}

type CodeSample struct {
	Lang   string
	Label  string
	Source string
}

type Parameter struct {
	Name        string
	Description string
	Required    bool
}

type RequestBody struct {
	Name        string
	Description string
}

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

	tmpl := template.Must(template.New("method").Funcs(template.FuncMap{
		"trim": strings.TrimSpace,
	}).Parse(methodTemplate))

	writeApiData(opData, tmpl)
}

// getApiData reads the OpenAPI spec and parses the operation data.
//
//nolint:funlen
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

			short, long := splitDescription(op.Description)

			data := OperationData{
				Acl:              utils.AclToString(acl),
				ApiName:          opts.ApiName,
				CodeSamples:      getCodeSamples(op),
				Deprecated:       boolOrFalse(op.Deprecated),
				Description:      long,
				OutputFilename:   utils.GetOutputFilename(op),
				OutputPath:       prefix,
				OperationIdKebab: utils.ToKebabCase(op.OperationId),
				Params:           getParameters(op),
				RequiresAdmin:    false,
				RequestBody:      getRequestBody(op),
				ShortDescription: short,
				Summary:          op.Summary,
			}

			if data.Acl == "`admin`" {
				data.RequiresAdmin = true
			}

			if op.ExternalDocs != nil {
				desc := strings.TrimSpace(op.ExternalDocs.Description)
				data.ExternalDocs.Description = strings.TrimSuffix(desc, ".")
				data.ExternalDocs.Url = op.ExternalDocs.URL
			}

			if data.ExternalDocs.Description != "" && data.ExternalDocs.Url != "" {
				data.SeeAlso = true
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

func splitDescription(p string) (string, string) {
	p = strings.TrimSpace(p)

	// Split by empty line
	parts := strings.SplitN(p, "\n\n", 2)
	if len(parts) > 1 && strings.TrimSpace(parts[0]) != "" {
		short := strings.TrimSpace(parts[0])
		long := strings.TrimSpace(parts[1])

		// No extra newline characters in between
		short = strings.ReplaceAll(short, "\n", "")

		return short, long
	}

	// No empty line: find first period
	if idx := strings.Index(p, "."); idx != -1 {
		short := strings.TrimSpace(p[:idx+1])
		long := strings.TrimSpace(p[idx+1:])

		// No extra newline characters in between
		short = strings.ReplaceAll(short, "\n", "")

		return short, long
	}

	// No period: entire paragraph is the shortDescription
	return p, ""
}

func getCodeSamples(op *v3.Operation) []CodeSample {
	node, ok := op.Extensions.Get("x-codeSamples")
	// Operations can be without code samples
	if !ok {
		return nil
	}

	var result []CodeSample

	for _, child := range node.Content {
		var c CodeSample

		child.Decode(&c)

		c.Lang = dictionary.NormalizeLang(c.Lang)

		if strings.ToLower(c.Label) != "curl" {
			result = append(result, c)
		}
	}

	return result
}

func getParameters(op *v3.Operation) []Parameter {
	var result []Parameter

	for _, p := range op.Parameters {
		param := Parameter{
			Name:        p.Name,
			Description: p.Description,
		}
		if p.Required != nil {
			param.Required = *p.Required
		}

		result = append(result, param)
	}

	return result
}

func getRequestBody(op *v3.Operation) RequestBody {
	return RequestBody{
		Description: "Unknown",
	}
}

func boolOrFalse(val *bool) bool {
	if val == nil {
		return false
	} else {
		return *val
	}
}
