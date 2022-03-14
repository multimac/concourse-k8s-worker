package main

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/driver"
	"github.com/concourse/flag"
	"github.com/concourse/kubernetes-worker/pkg/baggageclaimcsi/api"
	"github.com/jessevdk/go-flags"
)

type Opts struct {
	Logger flag.Lager

	OverlayDir flag.Dir `long:"overlay-dir" required:"true" description:"Directory in which to store overlay data."`
	VolumesDir flag.Dir `long:"volume-dir" required:"true" description:"Directory in which to place volume data."`

	BindAddress string `long:"bind-address" default:"tcp://:7788" description:"Address on which to serve the Baggage Claim API."`
}

func main() {
	opts := &Opts{}
	parser := flags.NewParser(opts, flags.Default)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		if err != flags.ErrHelp {
			fmt.Printf("%s\n", err)
		}

		os.Exit(1)
	}

	logger, _ := opts.constructLogger()
	logger.Info("initializing")

	locker := volume.NewLockManager()
	filesystem, err := volume.NewFilesystem(
		driver.NewOverlayDriver(opts.OverlayDir.Path()),
		opts.VolumesDir.Path(),
	)
	if err != nil {
		logger.Fatal("failed-to-initialize-filesystem", err)
	}

	bindAddress, err := url.Parse(opts.BindAddress)
	if err != nil {
		logger.Fatal("failed-to-parse-baggage-claim-address", err)
	}

	apiHandler, err := api.NewApi(
		logger,
		api.Config{
			BindNetwork: bindAddress.Scheme,
			BindAddress: bindAddress.Host,
		},
		filesystem,
		locker,
	)
	if err != nil {
		logger.Fatal("failed-to-create-baggageclaim-api", err)
	}

	err = apiHandler.Start(context.Background())
	if err != nil {
		logger.Fatal("problem-running-api-handler", err)
	}
}

func (cmd *Opts) constructLogger() (lager.Logger, *lager.ReconfigurableSink) {
	logger, reconfigurableSink := cmd.Logger.Logger("baggageclaim")

	return logger, reconfigurableSink
}
