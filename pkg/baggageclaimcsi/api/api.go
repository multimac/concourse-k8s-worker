package api

import (
	"context"
	"net"
	"net/http"
	"sync"

	"code.cloudfoundry.org/lager"
	baggageclaim_api "github.com/concourse/concourse/worker/baggageclaim/api"
	"github.com/concourse/concourse/worker/baggageclaim/uidgid"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
)

// import "sigs.k8s.io/controller-runtime/pkg/manager"
// var _ manager.Runnable = &BaggageClaimApi{}

type BaggageClaimApi struct {
	logger lager.Logger
	config Config

	handler http.Handler
}

type Config struct {
	BindNetwork string
	BindAddress string
}

func NewApi(
	logger lager.Logger,
	cfg Config,
	filesystem volume.Filesystem,
	locker volume.LockManager,
) (*BaggageClaimApi, error) {
	volumeRepo := volume.NewRepository(
		filesystem,
		locker,
		uidgid.NoopNamespacer{},
		uidgid.NoopNamespacer{},
	)

	handler, err := baggageclaim_api.NewHandler(
		logger,
		volume.NewStrategerizer(),
		volumeRepo,
		nil,
		0,
		0,
	)
	if err != nil {
		return nil, err
	}

	return &BaggageClaimApi{
		logger: logger,
		config: cfg,

		handler: handler,
	}, nil
}

func (api *BaggageClaimApi) Start(ctx context.Context) error {
	api.logger.Info("starting-server", lager.Data{
		"network": api.config.BindNetwork,
		"address": api.config.BindAddress,
	})

	listener, err := net.Listen(api.config.BindNetwork, api.config.BindAddress)
	if err != nil {
		return err
	}

	server := http.Server{
		Handler: api.handler,
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		err = server.Serve(listener)
		if err != nil {
			api.logger.Error("failed-to-serve", err)
		}

		wg.Done()
	}()

	<-ctx.Done()
	api.logger.Info("stopping-server")
	server.Shutdown(context.Background())

	wg.Wait()
	api.logger.Info("stopped-server")

	return err
}
