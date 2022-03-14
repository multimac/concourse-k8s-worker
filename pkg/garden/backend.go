package garden

import (
	"context"
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/garden"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	ErrUnsupported = errors.New("not supported")
)

type GardenBackend struct {
	config Config
	client *kubernetes.Clientset
}

type Config struct {
	BindNetwork string
	BindAddress string

	Namespace  string
	WorkerName string
}

var _ garden.Backend = &GardenBackend{}

func NewGardenBackend(
	cfg Config,
	client *kubernetes.Clientset,
) GardenBackend {
	return GardenBackend{
		config: cfg,
		client: client,
	}
}

func (backend *GardenBackend) Start() error {
	return nil
}

func (backend *GardenBackend) Stop() {
}

func (backend *GardenBackend) GraceTime(container garden.Container) time.Duration {
	return 0
}

func (backend *GardenBackend) Ping() error {
	return nil
}

func (backend *GardenBackend) Capacity() (garden.Capacity, error) {
	return garden.Capacity{}, nil
}

func (backend *GardenBackend) Create(spec garden.ContainerSpec) (garden.Container, error) {
	return nil, ErrUnsupported
}

func (backend *GardenBackend) Destroy(handle string) error {
	err := backend.client.CoreV1().
		Pods(backend.config.Namespace).
		Delete(context.Background(), handle, metav1.DeleteOptions{})

	if !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (backend *GardenBackend) Containers(filter garden.Properties) ([]garden.Container, error) {
	selector := fmt.Sprintf("atc.k8s.concourse-ci.org/worker=%s", backend.config.WorkerName)
	pods, err := backend.client.CoreV1().
		Pods(backend.config.Namespace).
		List(context.Background(), metav1.ListOptions{LabelSelector: selector})

	if err != nil {
		return nil, err
	}

	containers := make([]garden.Container, 0, len(pods.Items))
	for _, pod := range pods.Items {
		containers = append(containers, Container{
			pod:    pod,
			client: backend.client,
		})
	}

	return containers, nil
}

func (backend *GardenBackend) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	return nil, nil
}

func (backend *GardenBackend) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	return nil, nil
}

func (backend *GardenBackend) Lookup(handle string) (garden.Container, error) {
	pod, err := backend.client.CoreV1().
		Pods(backend.config.Namespace).
		Get(context.Background(), handle, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	return Container{
		pod:    *pod,
		client: backend.client,
	}, nil
}
