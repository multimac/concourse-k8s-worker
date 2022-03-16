package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/flag"
	"github.com/concourse/kubernetes-worker/pkg/garden"
	"github.com/jessevdk/go-flags"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Opts struct {
	Logger flag.Lager

	BindAddress     string `long:"bind-address" default:"tcp://:7777" description:"Address on which to serve the Garden API."`
	Namespace       string `long:"namespace" required:"true" description:"Kubernetes namespace to monitor for pod."`
	PodName         string `long:"pod-name" required:"true" description:"Name of this pod."`
	WorkerName      string `long:"worker-name" required:"true" description:"Name of this worker."`
	WorkerLabelName string `long:"worker-label-name" default:"baggageclaim.worker.k8s.concourse-ci.org/name" description:"Name of the label to add to the pod containing the worker's name"`
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

	gardenUrl, err := url.Parse(opts.BindAddress)
	if err != nil {
		logger.Fatal("failed-to-parse-baggage-claim-address", err)
	}

	kubernetesConfig, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatal("failed-to-read-in-cluster-config", err)
	}

	kubernetesClient, err := kubernetes.NewForConfig(kubernetesConfig)
	if err != nil {
		logger.Fatal("failed-to-initialize-api-client", err)
	}

	if err := patchWorkerLabel(kubernetesClient, opts); err != nil {
		logger.Fatal("failed-to-patch-worker-label", err)
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

func patchWorkerLabel(client *kubernetes.Clientset, opts *Opts) error {
	podClient := client.CoreV1().Pods(opts.Namespace)

	pod, err := podClient.Get(context.Background(), opts.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	originalPodJson, err := json.Marshal(pod)
	if err != nil {
		return err
	}

	pod.ObjectMeta.Labels[opts.WorkerLabelName] = opts.WorkerName

	updatedPodJson, err := json.Marshal(pod)
	if err != nil {
		return err
	}

	patchData, err := strategicpatch.CreateTwoWayMergePatch(originalPodJson, updatedPodJson, corev1.Pod{})
	if err != nil {
		return err
	}

	_, err = podClient.Patch(
		context.Background(),
		opts.PodName,
		types.StrategicMergePatchType,
		patchData,
		metav1.PatchOptions{},
	)

	return err
}
