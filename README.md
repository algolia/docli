# DOCLI

The CLI for working with Algolia's docs.

## Commands

### `docli generate openapi`

Generates OpenAPI stub files for each API endpoint,
augmented with ACL information.

### `docli generate sla`

Generates page with SLA information for API clients.

### `docli generate snippets`

Generate usage snippets files for API methods.

### `docli update cdn`

Updates HTML include snippets to show the latest available version.

Define a list of packages in a data file, by default `cdn.yml`.
Each package can have these fields:

- `name` (required). The name of the snippet.
- `pkg`. The NPM package name. If omitted, the `name` field is used.
- `file`. The file to include. If omitted, the default import of the package is used.

Each include snippet is defined as a Go template in a directory, by default `templates/`.
The filename of the template (minus extension) must match the `name` field of the package
defined in the data file.

When you run `docli update cdn`, the tool gets the latest released version for the package
and generates a snippet for each template in an output directory, by default `snippets/`.
