package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/baggageclaim/client"
	"github.com/concourse/flag"
	"github.com/concourse/kubernetes-worker/pkg/baggageclaimcsi/driver"
	"github.com/jessevdk/go-flags"
)

type Opts struct {
	Logger flag.Lager

	BaggageClaimAddress string `long:"baggage-claim-address" required:"true" description:"Address on which the Baggage Claim API for this node is being served."`
	CsiDriverName       string `long:"csi-driver-name" default:"baggageclaim.worker.k8s.concourse-ci.org" description:"Name of the CSI driver."`
	CsiSocket           string `long:"csi-socket" default:"/tmp/csi.sock" description:"Unix socket to listen on for CSI gRPC service."`
	InitBinPath         string `long:"init-bin-path" default:"/usr/local/concourse/bin/init" description:"Path to the 'init' binary used to keep a Pod alive while commands are being executed on it."`
	NodeId              string `long:"node-id" required:"true" description:"ID of the node running the driver."`
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

	baggageClaimClient := client.NewWithHTTPClient(
		opts.BaggageClaimAddress,

		// ensure we don't use baggageclaim's default retryhttp client; all
		// traffic should be local, so any failures are unlikely to be transient.
		// we don't want a retry loop to block up sweeping and prevent the worker
		// from existing.
		&http.Client{
			Transport: &http.Transport{
				// don't let a slow (possibly stuck) baggageclaim server slow down
				// sweeping too much
				ResponseHeaderTimeout: 1 * time.Minute,
			},
			// we've seen destroy calls to baggageclaim hang and lock gc
			// gc is periodic so we don't need to retry here, we can rely
			// on the next sweeper tick.
			Timeout: 5 * time.Minute,
		},
	)

	baggageClaimDriver, err := driver.NewBaggageClaimDriver(
		logger,
		driver.Config{
			DriverName:  opts.CsiDriverName,
			InitBinPath: opts.InitBinPath,
			Socket:      opts.CsiSocket,
			NodeId:      opts.NodeId,
			Version:     "dev",
		},
		baggageClaimClient,
	)
	if err != nil {
		logger.Fatal("failed-to-create-baggageclaim-csi-driver", err)
	}

	err = baggageClaimDriver.Start(context.Background())
	if err != nil {
		logger.Fatal("problem-running-driver", err)
	}
}

func (cmd *Opts) constructLogger() (lager.Logger, *lager.ReconfigurableSink) {
	logger, reconfigurableSink := cmd.Logger.Logger("csi-driver")

	return logger, reconfigurableSink
}
