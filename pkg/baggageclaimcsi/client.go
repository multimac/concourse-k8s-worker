package baggageclaimcsi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/concourse/concourse/worker/baggageclaim"
)

func LookupVolumeById(client baggageclaim.Client, id string) (baggageclaim.Volume, bool, error) {
	volumes, err := client.ListVolumes(context.Background(), baggageclaim.VolumeProperties{
		"baggageclaim.worker.k8s.concourse-ci.org/volume-id": id,
	})

	if err != nil {
		return nil, false, err
	}

	if len(volumes) > 1 {
		return nil, false, errors.New("multiple volumes found")
	}

	return volumes[0], true, nil
}

func LookupVolumeByPath(client baggageclaim.Client, path string) (baggageclaim.Volume, string, bool, error) {
	volumes, err := client.ListVolumes(context.Background(), nil)

	if err != nil {
		return nil, "", false, err
	}

	for _, vol := range volumes {
		if vol.Path() == path {
			return vol, "", true, nil
		}

		if subPath, relative, err := isSubPath(vol.Path(), path); err == nil && subPath {
			return vol, relative, true, nil
		}
	}

	return nil, "", false, nil
}

func isSubPath(parent, sub string) (bool, string, error) {
	up := ".." + string(os.PathSeparator)

	rel, err := filepath.Rel(parent, sub)
	if err != nil {
		return false, "", err
	}

	if !strings.HasPrefix(rel, up) && rel != ".." {
		return true, rel, nil
	}

	return false, "", nil
}
