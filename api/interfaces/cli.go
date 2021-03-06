package interfaces

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"

	prompt "github.com/c-bata/go-prompt"
	"github.com/pkg/errors"
	"github.com/takashabe/btcli/api/application"
	"github.com/takashabe/btcli/api/config"
	"github.com/takashabe/btcli/api/infrastructure/bigtable"
)

// exit codes
const (
	ExitCodeOK = 0

	// Specific error codes. begin 10-
	ExitCodeError = 10 + iota
	ExitCodeParseError
	ExitCodeInvalidArgsError
)

// CLI is the command line interface object
type CLI struct {
	OutStream io.Writer
	ErrStream io.Writer

	Version  string
	Revision string
}

// Run invokes the CLI with the given arguments
func (c *CLI) Run(args []string) int {
	conf, err := c.loadConfig(args)
	if err != nil {
		fmt.Fprintf(c.ErrStream, "args parse error: %v\n", err)
		return ExitCodeParseError
	}

	histories := []string{}
	f, err := loadHistoryFile(conf)
	if err != nil {
		// NOTE: Continue processing even if an error occurred at open a file
		fmt.Fprintf(c.ErrStream, "failed to a history file open: %v\n", err)
	} else {
		// TODO: Read lines and set to histories
		s := bufio.NewScanner(f)
		for s.Scan() {
			histories = append(histories, s.Text())
		}
		defer f.Close()
	}

	p, err := c.preparePrompt(conf, f, histories)
	if err != nil {
		fmt.Fprintf(c.ErrStream, "failed to initialized prompt: %v\n", err)
		return ExitCodeError
	}

	fmt.Fprintf(c.OutStream, "btcli Version: %s(%s)\n", c.Version, c.Revision)
	fmt.Fprintf(c.OutStream, "Please use `exit` or `Ctrl-D` to exit this program.\n")
	p.Run()

	// TODO: This is dead code. Invoke os.Exit by the prompt.Run
	return ExitCodeOK
}

func (c *CLI) loadConfig(args []string) (*config.Config, error) {
	conf := config.NewConfig(c.ErrStream)
	err := conf.Load()
	if err != nil {
		return nil, err
	}

	flag.Usage = func() {
		usage(c.OutStream)
	}
	return conf, nil
}

func usage(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [flags] <command> ...\n", os.Args[0])
	flag.CommandLine.SetOutput(w)
	flag.CommandLine.PrintDefaults()
}

func (c *CLI) preparePrompt(conf *config.Config, writer io.Writer, histories []string) (*prompt.Prompt, error) {
	repository, err := bigtable.NewBigtableRepository(conf.Project, conf.Instance)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to initialized bigtable repository:%v", err)
	}
	tableInteractor := application.NewTableInteractor(repository)
	rowsInteractor := application.NewRowsInteractor(repository)

	executor := Executor{
		outStream: c.OutStream,
		errStream: c.ErrStream,
		history:   writer,

		rowsInteractor:  rowsInteractor,
		tableInteractor: tableInteractor,
	}
	completer := Completer{
		tableInteractor: tableInteractor,
	}

	return prompt.New(
		executor.Do,
		completer.Do,
		prompt.OptionHistory(histories),
		prompt.OptionPreviewSuggestionTextColor(prompt.Blue),
		prompt.OptionSelectedSuggestionBGColor(prompt.LightGray),
		prompt.OptionSuggestionBGColor(prompt.DarkGray),
	), nil
}

func loadHistoryFile(conf *config.Config) (*os.File, error) {
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	return os.OpenFile(u.HomeDir+"/.btcli_history", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
}
