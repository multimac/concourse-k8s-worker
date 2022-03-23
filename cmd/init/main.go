package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/flag"
	"github.com/jessevdk/go-flags"
)

type Opts struct {
	Logger flag.Lager

	Interval    time.Duration `long:"interval" default:"5s" description:"Interval at which to print messages indicating the program is still waiting for a client to attach."`
	SkipWaiting bool          `long:"skip-waiting" description:"Skip the process of waiting for a client to attach."`
}

func main() {
	opts := &Opts{}
	parser := flags.NewParser(opts, flags.Default)

	args, err := parser.Parse()
	if err != nil {
		if err != flags.ErrHelp {
			fmt.Printf("%s\n", err)
		}

		os.Exit(1)
	}

	logger, _ := opts.constructLogger()

	if len(args) == 0 {
		logger.Fatal("no-command-given", nil)
	}

	program := args[0]
	args[0], err = exec.LookPath(program)

	if err != nil {
		logger.Fatal("could-not-resolve-executable", err, lager.Data{
			"program": program,
		})
	}

	if !opts.SkipWaiting {
		logger.Info("waiting-for-attach")
		go func() {
			ticker := time.NewTicker(opts.Interval)
			for range ticker.C {
				logger.Info("waiting-for-attach")
			}
		}()

		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			if scanner.Text() == "attached" {
				break
			}

			logger.Debug("unknown-line-received", lager.Data{
				"line": scanner.Text(),
			})
		}

		logger.Info("runtime-attached-executing-step", lager.Data{
			"command": args,
		})
	}

	err = syscall.Exec(args[0], args, os.Environ())
	logger.Fatal("exec-failed", err)
}

func (cmd *Opts) constructLogger() (lager.Logger, *lager.ReconfigurableSink) {
	logger, reconfigurableSink := cmd.Logger.Logger("init")
	return logger, reconfigurableSink
}
