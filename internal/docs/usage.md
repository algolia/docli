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
