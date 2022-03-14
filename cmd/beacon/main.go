package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	garden_client "github.com/concourse/concourse/atc/worker/gardenruntime/gclient"
	"github.com/concourse/concourse/worker"
	baggageclaim_client "github.com/concourse/concourse/worker/baggageclaim/client"
	"github.com/concourse/concourse/worker/workercmd"
	"github.com/concourse/flag"
	"github.com/concourse/kubernetes-worker/pkg/ifrit"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit/grouper"
)

type Opts struct {
	Logger flag.Lager

	Worker workercmd.WorkerConfig
	TSA    worker.TSAConfig `group:"TSA Configuration" namespace:"tsa"`

	BuiltInResources    []flag.File `long:"built-in-resource" description:"Path to a JSON metadata file for a built-in resourse provided by the worker."`
	BaggageClaimAddress string      `long:"baggage-claim-address" required:"true" description:"Address on which the Baggage Claim API for this node is being served."`
	GardenAddress       string      `long:"garden-address" required:"true" description:"Address on which the Garden API for this node is being served."`

	ConnectionDrainTimeout time.Duration `long:"connection-drain-timeout" default:"1h" description:"Duration after which a worker should give up draining forwarded connections on shutdown."`
	GardenRequestTimeout   time.Duration `long:"garden-request-timeout" default:"5m" description:"Duration after which requests to the Garden API should be cancelled."`
	RebalanceInterval      time.Duration `long:"rebalance-interval" default:"4h" description:"Duration after which the registration should be swapped to another random SSH gateway."`

	SweepInterval               time.Duration `long:"sweep-interval" default:"30s" description:"Interval on which containers and volumes will be garbage collected from the worker."`
	ContainerSweeperMaxInFlight uint16        `long:"container-sweeper-max-in-flight" default:"5" description:"Maximum number of containers which can be swept in parallel."`
	VolumeSweeperMaxInFlight    uint16        `long:"volume-sweeper-max-in-flight" default:"3" description:"Maximum number of volumes which can be swept in parallel."`
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

	atcWorker := opts.Worker.Worker()
	atcWorker.Platform = "linux"

	atcWorker.ResourceTypes = make([]atc.WorkerResourceType, len(opts.BuiltInResources))
	for i, file := range opts.BuiltInResources {
		metadata, err := ioutil.ReadFile(file.Path())
		if err != nil {
			logger.Fatal("failed-to-read-resource-type-metadata", err, lager.Data{
				"file": file.Path(),
			})
		}

		var resourceType atc.WorkerResourceType
		err = json.Unmarshal(metadata, &resourceType)
		if err != nil {
			logger.Fatal("failed-to-unmarshal-resource-type-metadata", err)
		}

		resourceType.Image = filepath.Join(filepath.Dir(file.Path()), "image.tar.gz")
		atcWorker.ResourceTypes[i] = resourceType
	}

	if len(atcWorker.ResourceTypes) > 0 {
		logger.Info("found-built-in-resource-types", lager.Data{
			"resource-types": atcWorker.ResourceTypes,
		})
	} else {
		logger.Info("no-built-in-resource-types-found")
	}

	tsaClient := opts.TSA.Client(atcWorker)

	baggageClaimClient := baggageclaim_client.NewWithHTTPClient(
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

	gardenClient := garden_client.BasicGardenClientWithRequestTimeout(
		logger.Session("garden-connection"),
		opts.GardenRequestTimeout,
		opts.GardenAddress,
	)

	baggageClaimUrl, err := url.Parse(opts.BaggageClaimAddress)
	if err != nil {
		logger.Fatal("failed-to-parse-baggage-claim-address", err)
	}

	gardenUrl, err := url.Parse(opts.GardenAddress)
	if err != nil {
		logger.Fatal("failed-to-parse-garden-address", err)
	}

	beacon := worker.NewBeaconRunner(
		logger.Session("beacon"),
		tsaClient,
		opts.RebalanceInterval,
		opts.ConnectionDrainTimeout,
		gardenUrl.Host,
		baggageClaimUrl.Host,
	)

	containerSweeper := worker.NewContainerSweeper(
		logger.Session("container-sweeper"),
		opts.SweepInterval,
		tsaClient,
		gardenClient,
		opts.ContainerSweeperMaxInFlight,
	)

	volumeSweeper := worker.NewVolumeSweeper(
		logger.Session("volume-sweeper"),
		opts.SweepInterval,
		tsaClient,
		baggageClaimClient,
		opts.VolumeSweeperMaxInFlight,
	)

	runner := ifrit.NewRunnable(
		grouper.NewParallel(os.Interrupt, grouper.Members{
			{Name: "beacon", Runner: beacon},
			{Name: "container-sweeper", Runner: containerSweeper},
			{Name: "volume-sweeper", Runner: volumeSweeper},
		}),
	)

	err = runner.Start(context.Background())
	if err != nil {
		logger.Fatal("problem-running-beacon", err)
	}
}

func (cmd *Opts) constructLogger() (lager.Logger, *lager.ReconfigurableSink) {
	logger, reconfigurableSink := cmd.Logger.Logger("beacon")

	return logger, reconfigurableSink
}
