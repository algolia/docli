# DOCLI

A command line tool for generating content for the Algolia documentation on Mintlify.

## Installation

Go to the [GitHub releases](https://github.com/algolia/docli/releases/latest) page
and download the latest release that's suitable for your computer.

Then, unpack the `tar.gz` file, for example, with `tar xvf docli_*tar.gz`.

> [!TIP]
> Before you can use the CLI on your Mac,
> you might need to run `xattr -d com.apple.quarantine docli`.

To enable command completion, run `./docli completion --help`
for more information about activating it for your shell.

## Development

> [!NOTE]
> If you're using [mise](https://mise.jdx.dev), setting up the development environment is automated.

1. Clone the `github.com/algolia/docli` repository.
1. Change into the cloned repository: `cd docli`.

   - **With mise:** run `mise install` to install the dependencies and activate the environment.
   - **Without mise:** manually install the dependencies listed in `mise.toml`.

1. Build the project, by running `mise run build`.
   See the other available tasks by running `mise tasks`.

## Reference

<!-- auto-generated -->
