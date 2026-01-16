package output

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

const (
	FlagVerbose = "verbose"
	FlagQuiet   = "quiet"
	FlagDryRun  = "dry-run"
)

type Printer struct {
	cmd     *cobra.Command
	verbose bool
	quiet   bool
	dryRun  bool
}

func New(cmd *cobra.Command) (*Printer, error) {
	verbose, err := cmd.Flags().GetBool(FlagVerbose)
	if err != nil {
		return nil, err
	}

	quiet, err := cmd.Flags().GetBool(FlagQuiet)
	if err != nil {
		return nil, err
	}

	dryRun, err := cmd.Flags().GetBool(FlagDryRun)
	if err != nil {
		return nil, err
	}

	if verbose && quiet {
		return nil, fmt.Errorf("cannot use --%s and --%s together", FlagVerbose, FlagQuiet)
	}

	return &Printer{
		cmd:     cmd,
		verbose: verbose,
		quiet:   quiet,
		dryRun:  dryRun,
	}, nil
}

func (p *Printer) Infof(format string, args ...any) {
	if p.quiet {
		return
	}

	p.cmd.Printf(format, args...)
}

func (p *Printer) Verbosef(format string, args ...any) {
	if p.quiet || !p.verbose {
		return
	}

	p.cmd.Printf(format, args...)
}

func (p *Printer) Errf(format string, args ...any) {
	p.cmd.PrintErrf(format, args...)
}

func (p *Printer) IsVerbose() bool {
	return p.verbose
}

func (p *Printer) IsQuiet() bool {
	return p.quiet
}

func (p *Printer) IsDryRun() bool {
	return p.dryRun
}

func (p *Printer) WriteFile(path string, write func(io.Writer) error) error {
	if p.dryRun {
		if err := write(io.Discard); err != nil {
			return fmt.Errorf("render dry-run %s: %w", path, err)
		}

		p.Infof("Dry run: would write %s\n", path)

		return nil
	}

	output, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}

	if err := write(output); err != nil {
		_ = output.Close()

		return fmt.Errorf("write %s: %w", path, err)
	}

	if err := output.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}

	return nil
}
