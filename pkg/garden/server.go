package garden

import (
	"context"
	"net"
	"sync"

	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager"
	"k8s.io/client-go/kubernetes"
)

// import "sigs.k8s.io/controller-runtime/pkg/manager"
// var _ manager.Runnable = &GardenServer{}

type GardenServer struct {
	logger lager.Logger
	config Config

	server *server.GardenServer
}

func NewGardenServer(
	logger lager.Logger,
	cfg Config,
	client *kubernetes.Clientset,
) *GardenServer {
	backend := NewGardenBackend(cfg, client)
	return &GardenServer{
		logger: logger,
		config: cfg,

		server: server.New(
			cfg.BindNetwork,
			cfg.BindAddress,
			0,
			&backend,
			logger,
		),
	}
}

func (backend *GardenServer) Start(ctx context.Context) error {
	backend.logger.Info("starting-server", lager.Data{
		"network": backend.config.BindNetwork,
		"address": backend.config.BindAddress,
	})

	if err := backend.server.SetupBomberman(); err != nil {
		return err
	}

	listener, err := net.Listen(
		backend.config.BindNetwork,
		backend.config.BindAddress,
	)
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		err = backend.server.Serve(listener)
		if err != nil {
			backend.logger.Error("failed-to-serve", err)
		}

		wg.Done()
	}()

	<-ctx.Done()
	backend.logger.Info("stopping-server")
	backend.server.Stop()

	wg.Wait()
	backend.logger.Info("stopped-server")

	return err
}
