package command

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/helper/hclfmt"
	"github.com/posener/complete"
)

type FormatCommand struct {
	Meta
}

var (
	check, diff, write bool
	path, file         string
)

func (*FormatCommand) Help() string {
	helpText := `
Usage: nomad fmt [options] [TEMPLATE]
  Rewrites all Nomad configuration files to a canonical format. 
  Configuration files (.nomad) are updated.
  JSON files (.json) are not modified.
  The given content must be in Nomad's HCL2 configuration language; JSON is
  not supported.
Options:
  -check        Check if the input is formatted. Exit status will be 0 if all
                 input is properly formatted and non-zero otherwise.
  -diff         Display diffs of formatting change
  -write=false  Don't write to source files
                (always disabled if using -check)
  -path			Directory if not "." current directory to read
  -file 		Name of file in current directory to read
`

	return strings.TrimSpace(helpText)
}

func (*FormatCommand) Synopsis() string {
	return "Rewrites HCL2 config files to canonical format"
}

func (*FormatCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{
		"-check": complete.PredictNothing,
		"-diff":  complete.PredictNothing,
		"-write": complete.PredictNothing,
		"-path":  complete.PredictNothing,
		"-file":  complete.PredictNothing,
	}
}

func (f *FormatCommand) Name() string { return "fmt" }

func (f *FormatCommand) Run(args []string) int {
	ctx := context.Background()
	ret := f.ParseArgs(args)
	if ret != 0 {
		return 1
	}

	if num, err := f.RunContext(ctx); num != 0 {
		f.Ui.Error(fmt.Sprintf("%s", err))
		return num
	}
	return 0
}

func (f *FormatCommand) ParseArgs(args []string) int {
	flags := f.Meta.FlagSet(f.Name(), FlagSetClient)

	flags.Usage = func() { f.Ui.Output(f.Help()) }
	flags.BoolVar(&check, "check", false, "")
	flags.BoolVar(&diff, "diff", false, "")
	flags.BoolVar(&write, "write", true, "")
	flags.StringVar(&file, "file", "", "")
	flags.StringVar(&path, "path", "", "")

	if err := flags.Parse(args); err != nil {
		f.Ui.Error("Unable to parse flags")
		return 1
	}

	// Check if we got into any arguments
	args = flags.Args()

	if l := len(args); l < 0 || l > 1 {
		f.Ui.Error("This command takes up to one argument")
		f.Ui.Error(commandErrorText(f))
		return 1
	}
	return 0
}

func (f *FormatCommand) RunContext(ctx context.Context) (int, error) {

	if check {
		write = false
	}

	formatter := hclfmt.HCL2Formatter{
		ShowDiff: diff,
		Write:    write,
		File:     file != "",
		Output:   os.Stdout,
	}

	//if file is passed, set it to path
	if file != "" {
		path = file
	}

	bytesModified, err := formatter.Format(path)
	if err != nil {
		return 1, fmt.Errorf("error parsing files: %s", err)
	}

	if check && bytesModified > 0 {
		// exit code taken from `terraform fmt` command
		return 3, nil
	}

	return 0, nil

}
