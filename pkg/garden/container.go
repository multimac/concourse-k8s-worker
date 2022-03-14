package garden

import (
	"context"
	"io"
	"time"

	"code.cloudfoundry.org/garden"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Container struct {
	client *kubernetes.Clientset
	pod    corev1.Pod
}

var _ garden.Container = &Container{}

func (c Container) Handle() string {
	return c.pod.Name
}

func (c Container) Stop(kill bool) error {
	err := c.client.CoreV1().
		Pods(c.pod.Namespace).
		Delete(context.Background(), c.pod.Name, metav1.DeleteOptions{})

	if !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (c Container) Info() (garden.ContainerInfo, error) {
	return garden.ContainerInfo{}, ErrUnsupported
}

func (c Container) StreamIn(spec garden.StreamInSpec) error {
	return ErrUnsupported
}

func (c Container) StreamOut(spec garden.StreamOutSpec) (io.ReadCloser, error) {
	return nil, ErrUnsupported
}

func (c Container) CurrentBandwidthLimits() (garden.BandwidthLimits, error) {
	return garden.BandwidthLimits{}, ErrUnsupported
}

func (c Container) CurrentCPULimits() (garden.CPULimits, error) {
	return garden.CPULimits{}, ErrUnsupported
}

func (c Container) CurrentDiskLimits() (garden.DiskLimits, error) {
	return garden.DiskLimits{}, ErrUnsupported
}

func (c Container) CurrentMemoryLimits() (garden.MemoryLimits, error) {
	return garden.MemoryLimits{}, ErrUnsupported
}

func (c Container) NetIn(hostPort, containerPort uint32) (uint32, uint32, error) {
	return 0, 0, ErrUnsupported
}

func (c Container) NetOut(netOutRule garden.NetOutRule) error {
	return ErrUnsupported
}

func (c Container) BulkNetOut(netOutRules []garden.NetOutRule) error {
	return ErrUnsupported
}

func (c Container) Run(garden.ProcessSpec, garden.ProcessIO) (garden.Process, error) {
	return nil, ErrUnsupported
}

func (c Container) Attach(processID string, io garden.ProcessIO) (garden.Process, error) {
	return nil, ErrUnsupported
}

func (c Container) Metrics() (garden.Metrics, error) {
	return garden.Metrics{}, ErrUnsupported
}

func (c Container) SetGraceTime(graceTime time.Duration) error {
	return ErrUnsupported
}

func (c Container) Properties() (garden.Properties, error) {
	return nil, ErrUnsupported
}

func (c Container) Property(name string) (string, error) {
	return "", ErrUnsupported
}

func (c Container) SetProperty(name string, value string) error {
	return ErrUnsupported
}

func (c Container) RemoveProperty(name string) error {
	return ErrUnsupported
}
