package main

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/flag"
	"github.com/concourse/kubernetes-worker/pkg/garden"
	"github.com/jessevdk/go-flags"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Opts struct {
	Logger flag.Lager

	BindAddress string `long:"bind-address" default:"tcp://:7777" description:"Address on which to serve the Garden API."`
	Namespace   string `long:"namespace" required:"true" description:"Kubernetes namespace to monitor for pod."`
	WorkerName  string `long:"worker-name" required:"true" description:"Name of this worker."`
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

	kubernetesConfig, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatal("failed-to-read-in-cluster-config", err)
	}

	kubernetesClient, err := kubernetes.NewForConfig(kubernetesConfig)
	if err != nil {
		logger.Fatal("failed-to-initialize-api-client", err)
	}

	gardenUrl, err := url.Parse(opts.BindAddress)
	if err != nil {
		logger.Fatal("failed-to-parse-baggage-claim-address", err)
	}

	gardenServer := garden.NewGardenServer(
		logger,
		garden.Config{
			BindNetwork: gardenUrl.Scheme,
			BindAddress: gardenUrl.Host,
			Namespace:   opts.Namespace,
			WorkerName:  opts.WorkerName,
		},
		kubernetesClient,
	)

	err = gardenServer.Start(context.Background())
	if err != nil {
		logger.Fatal("problem-running-garden-server", err)
	}
}

func (cmd *Opts) constructLogger() (lager.Logger, *lager.ReconfigurableSink) {
	logger, reconfigurableSink := cmd.Logger.Logger("garden")

	return logger, reconfigurableSink
}
