package driver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/mount"
)

func (driver *BaggageClaimDriver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	caps := []*csi.NodeServiceCapability{}
	return &csi.NodeGetCapabilitiesResponse{Capabilities: caps}, nil
}

func (driver *BaggageClaimDriver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability missing in request")
	}

	if req.GetVolumeCapability().GetMount() == nil {
		return nil, status.Error(codes.InvalidArgument, "driver only supports block access type")
	}

	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "target path missing in request")
	}
	targetPath := req.GetTargetPath()

	mounter := mount.New("")
	if notMount, _ := mount.IsNotMountPoint(mounter, targetPath); !notMount {
		return &csi.NodePublishVolumeResponse{}, nil
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("unable to create mount directory: %s", err))
	}

	stat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err = os.Mkdir(targetPath, 0750); err != nil {
				return nil, fmt.Errorf("unable to create mount point '%s': %w", targetPath, err)
			}
		} else {
			return nil, fmt.Errorf("failed to check target path: %w", err)
		}
	} else if !stat.IsDir() {
		return nil, fmt.Errorf("target path is not a directory, '%s'", targetPath)
	}

	var sourcePath string
	if handle, found := req.GetVolumeContext()["baggageclaim.k8s.concourse-ci.org/handle"]; found {
		vol, found, err := driver.client.LookupVolume(ctx, handle)
		if err != nil {
			driver.logger.Error("failed-to-lookup-volume", err)
			return nil, fmt.Errorf("failed to lookup volume: %w", err)
		}

		if !found {
			driver.logger.Info("volume-not-found")
			return nil, status.Error(codes.NotFound, "volume does not exist")
		}

		sourcePath = vol.Path()
	} else if _, found := req.GetVolumeContext()["baggageclaim.k8s.concourse-ci.org/init-binary"]; found {
		sourcePath = driver.config.InitBinPath
	} else {
		return nil, status.Error(codes.InvalidArgument, "missing 'handle' or 'init-binary' keys in volume context")
	}

	driver.logger.Debug("binding-path-to-pod", lager.Data{
		"source": sourcePath,
		"target": targetPath,
	})

	options := []string{"bind"}
	if err := mounter.Mount(sourcePath, targetPath, "", options); err != nil {
		var errList strings.Builder
		errList.WriteString(err.Error())

		return nil, fmt.Errorf("failed to mount device: %s at %s: %s", sourcePath, targetPath, errList.String())
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (driver *BaggageClaimDriver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "target path missing in request")
	}
	targetPath := req.GetTargetPath()

	// Unmount only if the target path is really a mount point.
	mounter := mount.New("")
	if notMnt, err := mount.IsNotMountPoint(mounter, targetPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("check target path: %w", err)
		}
	} else if !notMnt {
		err = mounter.Unmount(targetPath)
		if err != nil {
			return nil, fmt.Errorf("unmount target path: %w", err)
		}
	}

	// Delete the mount point.
	// Does not return error for non-existent path, repeated calls OK for idempotency.
	if err := os.RemoveAll(targetPath); err != nil {
		return nil, fmt.Errorf("remove target path: %w", err)
	}

	driver.logger.Debug("volume has been unpublished.", lager.Data{
		"path": targetPath,
	})

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (driver *BaggageClaimDriver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "UNIMPLEMENTED")
}

func (driver *BaggageClaimDriver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "UNIMPLEMENTED")
}

func (driver *BaggageClaimDriver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: driver.config.NodeId,
	}, nil
}

func (driver *BaggageClaimDriver) NodeGetVolumeStats(ctx context.Context, in *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "UNIMPLEMENTED")
}

// NodeExpandVolume is only implemented so the driver can be used for e2e testing.
func (driver *BaggageClaimDriver) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "UNIMPLEMENTED")
}
