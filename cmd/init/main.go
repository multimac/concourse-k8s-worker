package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/flag"
	"github.com/jessevdk/go-flags"
)

type Opts struct {
	Logger flag.Lager

	Sleep bool `long:"sleep"`

	Dir  string   `long:"dir"`
	Env  []string `long:"env"`
	User string   `long:"user"`
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

	if opts.Sleep {
		exitSignal := make(chan os.Signal, 1)
		signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM)

		logger.Info("waiting-for-signal")
		<-exitSignal

		logger.Info("signal-received")
		return
	}

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

	if opts.User != "" {
		u, err := user.Lookup(opts.User)
		if err != nil {
			logger.Fatal("lookup-user", err)
		}

		uid, err := strconv.Atoi(u.Uid)
		if err != nil {
			logger.Fatal("parse-uid", err)
		}

		err = syscall.Setuid(uid)
		if err != nil {
			logger.Fatal("set-uid", err)
		}
	}

	if opts.Dir != "" {
		err := syscall.Chdir(opts.Dir)
		if err != nil {
			logger.Fatal("chdir", err)
		}
	}

	err = syscall.Exec(args[0], args, append(os.Environ(), opts.Env...))
	logger.Fatal("exec-failed", err)
}

func (cmd *Opts) constructLogger() (lager.Logger, *lager.ReconfigurableSink) {
	logger, reconfigurableSink := cmd.Logger.Logger("init")
	return logger, reconfigurableSink
}
