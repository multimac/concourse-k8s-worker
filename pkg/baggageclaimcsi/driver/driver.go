package driver

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"sync"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/baggageclaim"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
)

// import "sigs.k8s.io/controller-runtime/pkg/manager"
// var _ manager.Runnable = &BaggageClaimDriver{}

type BaggageClaimDriver struct {
	logger lager.Logger
	config Config

	client baggageclaim.Client
}

type Config struct {
	DriverName  string
	InitBinPath string
	NodeId      string
	Version     string

	Endpoint string
	Socket   string
}

func NewBaggageClaimDriver(
	logger lager.Logger,
	cfg Config,
	client baggageclaim.Client,
) (*BaggageClaimDriver, error) {
	if cfg.DriverName == "" {
		return nil, errors.New("no driver name provided")
	}

	if cfg.Version == "" {
		return nil, errors.New("no driver version provided")
	}

	return &BaggageClaimDriver{
		logger: logger,
		config: cfg,

		client: client,
	}, nil
}

func (driver *BaggageClaimDriver) Start(ctx context.Context) error {
	driver.logger.Info("starting-server", lager.Data{
		"endpoint": driver.config.Endpoint,
		"socket":   driver.config.Socket,
	})

	if driver.config.Endpoint != "" && driver.config.Socket != "" {
		return errors.New("must specify either a endpoint or unix socket to listen on, not both")
	}

	var listener net.Listener
	var err error
	if driver.config.Endpoint != "" {
		listener, err = listenOnEndpoint(driver.config.Endpoint)
	} else {
		listener, err = listenOnSocket(driver.config.Socket)
	}

	if err != nil {
		return err
	}

	opts := []grpc.ServerOption{}
	server := grpc.NewServer(opts...)

	csi.RegisterIdentityServer(server, driver)
	csi.RegisterNodeServer(server, driver)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		err = server.Serve(listener)
		if err != nil {
			driver.logger.Error("failed-to-serve", err)
		}

		wg.Done()
	}()

	<-ctx.Done()
	driver.logger.Info("stopping-server")
	server.GracefulStop()

	wg.Wait()
	driver.logger.Info("stopped-server")

	return err
}

func listenOnEndpoint(endpoint string) (net.Listener, error) {
	url, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	return net.Listen(url.Scheme, url.Host)
}

func listenOnSocket(path string) (net.Listener, error) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return net.Listen("unix", path)
}
