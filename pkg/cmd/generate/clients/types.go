package clients

// Options represents configuration options and CLI flags for this command.
type Options struct {
	APIName         string
	InputFilename   string
	OutputDirectory string
}

// ExternalDocs holds an externalDocs reference.
type ExternalDocs struct {
	Description string
	URL         string
}

// OperationData represents relevant information about an API operation.
type OperationData struct {
	ACL              string
	APIName          string
	CodeSamples      []CodeSample
	Deprecated       bool
	Description      string
	ExternalDocs     ExternalDocs
	InputFilename    string
	OutputFilename   string
	OutputPath       string
	OperationIDKebab string
	Params           []Parameter
	Responses        []OperationResponse
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
	Name          string
	Description   string
	Deprecated    bool
	Required      bool
	Type          string
	In            string
	Children      []Parameter
	Variants      []ParameterVariant
	AllowedValues []string
}

type ParameterVariant struct {
	Title         string
	Description   string
	Type          string
	Children      []Parameter
	AllowedValues []string
}

type ResponseField struct {
	Name          string
	Description   string
	Deprecated    bool
	Required      bool
	Type          string
	Children      []ResponseField
	Variants      []ResponseVariant
	AllowedValues []string
}

type ResponseVariant struct {
	Title         string
	Description   string
	Type          string
	Children      []ResponseField
	AllowedValues []string
}

type OperationResponse struct {
	StatusCode   string
	Description  string
	Fields       []ResponseField
	SortPriority int
}
