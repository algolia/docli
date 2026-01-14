# DOCLI

A command line tool for generating content for the Algolia documentation on Mintlify.

## Installation

Go to the [GitHub releases](https://github.com/algolia/docli/releases/latest) page
and download the latest release that's suitable for your computer.

Then, unpack the `tar.gz` file, for example, with `tar xvf docli_*tar.gz`.

Optional: if you're using command completion, run `./docli completion --help`
for more information about activating it for your shell.

## Development

> [!NOTE]
> If you're using [devbox](https://www.jetify.com/devbox) and [direnv](https://direnv.net/),
> setting up the development environment is automated.

1. Clone the `github.com/algolia/docli` repository.
1. Change into the cloned repository: `cd docli`.

   - **With devbox and direnv:** the dependencies are installed automatically.
   - **With devbox:** run `devbox shell` to install the dependencies and activate the environment.
   - **Without devbox:** manually install the dependencies listed in `devbox.json`.

1. Build the project, by running `task build`.
   See the other available tasks by running `task -l`.

## Reference

<!-- auto-generated -->
```sh
docli
```

Generate content for the Algolia docs on Mintlify.

Not all of Algolia's docs are handwritten.
Some content is generated from data files.
This CLI helps with that.

See the individual commands to learn what you can do with it.

**Commands:** `generate`

### `docli generate`

```sh
docli generate
```

Generate content from data files.

Each command reads data from a file,
interpolates them into a template,
and writes on or more MDX files.
Most templates are built into the CLI,
some can be provided at runtime.

This is useful when running in CI whenever data files are updated.

See the individual subcommands to learn what content you can generate.

**Aliases:** `gen`, `g`

**Subcommands:** `cdn`, `clients`, `guides`, `openapi`, `sla`, `snippets`

#### `docli generate cdn`

```sh
docli generate cdn [flags]
```

Generate HTML import snippets with latest versions.

This command generates import snippets with version numbers.

When documenting code with HTML <link> or <script> tags for remote resources,
it's best to specify a specific version and the matching SRI hash.

The command reads a data file (default: cdn.yml),
iterates over the entries,
and applies matching templates from the templates directory.
Each package name in cdn.yml must match a template name.
For example, if the package is autocomplete_js,
the command looks for the template file autocomplete_js.mdx.tmpl.

**Examples**

```sh
# Run from the root of algolia/docs-new
docli gen cdn -o snippets/autocomplete/includes -d cdn.yml -t templates
```

**Flags**

`-d, --data string`  Data file with package information. (default: `cdn.yml`)

`-h, --help`  Help for this command

`-o, --output string`  Output directory for generated files (default: `out`)

`-t, --templates string`  Directory with template files for interpolation. (default: `templates`)


#### `docli generate clients`

```sh
docli generate clients [flags]
```

Generate MDX files for the API client method references.

This command reads an OpenAPI 3 spec file and generates one MDX file per operation.
It writes an API reference with usage information specific to API clients,
which may follow different conventions depending on the programming language used.
This command doesn't delete MDX files. If you remove or rename an operation,
you need to update or delete its MDX file manually.

**Aliases:** `c`

**Examples**

```sh
# Run from root of algolia/docs-new
docli gen clients specs/search.yml -o doc/libraries/sdk/methods
```

**Flags**

`-h, --help`  Help for this command

`-o, --output string`  Output directory for generated MDX files (default: `out`)


#### `docli generate guides`

```sh
docli generate guides <guides> [flags]
```

Generate guide snippets from a JSON file.

This command reads a data file with guide snippets.
It generates an MDX file for each guide.

**Examples**

```sh
# Run from root of algolia/docs-new
docli gen guides guides.json -o openapi-snippets/guides
```

**Flags**

`-h, --help`  Help for this command

`-o, --output string`  Output directory for generated MDX files (default: `out`)


#### `docli generate openapi`

```sh
docli generate openapi <spec> [flags]
```

Generate MDX files for the HTTP API reference.

This command reads an OpenAPI 3 spec and generates one MDX file per API operation.
Useful when adding new operations or changing operation summaries.
It doesn't delete MDX files. If you remove or rename an operation,
you need to update or delete its MDX file manually.

**Aliases:** `stubs`

**Examples**

```sh
# Run from root of algolia/docs-new
docli gen stubs specs/search.yml -o doc/rest-api
```

**Flags**

`-h, --help`  Help for this command

`-o, --output string`  Output directory for generated MDX files (default: `out`)


#### `docli generate sla`

```sh
docli generate sla <data> [flags]
```

Generate page with SLA information for API clients.

This command reads a data file with API client versions and SLA status,
then generates an MDX file listing supported versions.

Use --versions-snippets-file to also generate a snippet file,
so you can include the latest client version in the docs.

**Examples**

```sh
# Run from root of algolia/docs-new
docli gen sla specs/versions-history-with-sla-and-support-policy.json \
 	-o doc/libraries/sdk/versions.mdx \
	--versions-snippets-file snippets/sdk/versions.mdx
```

**Flags**

`-h, --help`  Help for this command

`-o, --output string`  MDX file for listing the supported versions

`--versions-snippets-file string`  Snippet file with latest released version numbers


#### `docli generate snippets`

```sh
docli generate snippets <snippets> [flags]
```

Generate API client example snippets from an OpenAPI snippet file.

This command reads a data file with API client usage snippets.
It generates an MDX file for each snippet so you can include them in the docs.

**Examples**

```sh
# Run from root of algolia/docs-new
docli gen snippets specs/search-snippets.json -o openapi-snippets/search
```

**Flags**

`-h, --help`  Help for this command

`-o, --output string`  Output directory for generated MDX files (default: `out`)


