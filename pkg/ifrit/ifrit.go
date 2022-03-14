package ifrit

import (
	"context"

	"github.com/tedsuo/ifrit"
)

// import "sigs.k8s.io/controller-runtime/pkg/manager"
// var _ manager.Runnable = &Runnable{}

type Runnable struct {
	runner ifrit.Runner
}

func NewRunnable(runner ifrit.Runner) Runnable {
	return Runnable{runner: runner}
}

func (runnable *Runnable) Start(ctx context.Context) error {
	return <-ifrit.Invoke(runnable.runner).Wait()
}
