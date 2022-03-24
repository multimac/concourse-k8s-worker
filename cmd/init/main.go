package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/flag"
	"github.com/jessevdk/go-flags"
)

type Opts struct {
	Logger flag.Lager

	SignalMessage string        `long:"signal-message" default:"client-attached" description:"Message to wait to be sent via stdin to indicate the client has attached."`
	Interval      time.Duration `long:"interval" default:"5s" description:"Interval at which to print messages indicating the program is still waiting for a client to attach."`
	SkipWaiting   bool          `long:"skip-waiting" description:"Skip the process of waiting for a client to attach."`
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
		go func() {
			logWaiting := func() {
				logger.Info("waiting-for-attach", lager.Data{
					"expected-message": opts.SignalMessage,
				})
			}

			logWaiting()

			ticker := time.NewTicker(opts.Interval)
			for range ticker.C {
				logWaiting()
			}
		}()

		expectedMsg := []byte(opts.SignalMessage)

		// Minimum size of the reader buffer is 16 bytes, but we expect a newline
		// to be present so the minimum signal message length is actually 15 bytes
		if len(expectedMsg) < 15 {
			logger.Fatal("signal-message-too-short", nil, lager.Data{
				"minimum-signal-message-length": 15,
				"given-signal-message-length":   len(expectedMsg),
			})
		}

		reader := bufio.NewReaderSize(os.Stdin, len(expectedMsg)+1)
		if reader.Size() != len(expectedMsg)+1 {
			logger.Fatal("buffer-too-large-allocated", nil, lager.Data{
				"expected-size": len(expectedMsg) + 1,
				"actual-size":   reader.Size(),
			})
		}

		for {
			line, prefix, err := reader.ReadLine()
			if err != nil {
				logger.Fatal("error-reading-stdin", err)
			}

			if prefix {
				logger.Debug("partial-line-received", lager.Data{
					"line": line,
				})
				continue
			}

			if !reflect.DeepEqual(line, expectedMsg) {
				logger.Debug("unexpected-line-received", lager.Data{
					"line": line,
				})
				continue
			}

			break
		}

		logger.Debug("runtime-attached-executing-step", lager.Data{
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
