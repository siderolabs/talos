// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

// inlineDataVolumeMountOptions are forced on every container mount of an inline data volume.
var inlineDataVolumeMountOptions = []string{"ro", "nosuid", "nodev", "noexec", "rprivate"}

// inlineDataVolumeDroppedOptions are removed from inline data volume mounts (MountsResolvedToOCI adds `rbind`).
var inlineDataVolumeDroppedOptions = []string{"rw", "rshared", "rbind"}

// reconcileInlineDataMount requests a read-only bind of an inline data volume and resolves the mount to it.
func (ctrl *MountController) reconcileInlineDataMount(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	containerSpecID, requestID, volumeID string,
	containerMountSpec containers.ContainerMountSpec,
	isLive bool,
) (containers.ResolvedMountSpec, string, error) {
	volumeName := containerMountSpec.VolumeName

	if err := safe.WriterModify(ctx, runtime, files.NewInlineDataVolumeMountRequest(files.NamespaceName, requestID),
		func(res *files.InlineDataVolumeMountRequest) error {
			res.Metadata().Labels().Set(containerLabel, containerSpecID)

			res.TypedSpec().VolumeName = volumeName
			res.TypedSpec().Requester = ctrl.Name()

			return nil
		},
	); err != nil {
		return containers.ResolvedMountSpec{}, "", fmt.Errorf("failed to write inline data volume mount request %q: %w", requestID, err)
	}

	mountStatus, err := safe.ReaderGetByID[*files.InlineDataVolumeMountStatus](ctx, runtime, requestID)
	if err != nil {
		if !state.IsNotFoundError(err) {
			return containers.ResolvedMountSpec{}, "", fmt.Errorf("failed to get inline data volume mount status %q: %w", requestID, err)
		}

		return containers.ResolvedMountSpec{}, fmt.Sprintf("waiting for the files of inline data volume %q to be rendered", volumeName), nil
	}

	if mountStatus.Metadata().Phase() != resource.PhaseRunning {
		reason, releaseErr := ctrl.releaseUnlessLive(
			ctx, runtime, logger, mountStatus.Metadata(), containerSpecID, fmt.Sprintf("inline data volume %q", volumeName), isLive,
		)

		return containers.ResolvedMountSpec{}, reason, releaseErr
	}

	if !mountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
		if err = runtime.AddFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
			return containers.ResolvedMountSpec{}, "", fmt.Errorf("failed to add finalizer on %q: %w", requestID, err)
		}

		logger.Info("holding inline data volume bind for container",
			zap.String("container", containerSpecID),
			zap.String("volume", volumeName),
			zap.String("target", mountStatus.TypedSpec().Target),
		)
	}

	return containers.ResolvedMountSpec{
		Kind:        containerMountSpec.Kind,
		Source:      mountStatus.TypedSpec().Target,
		Destination: containerMountSpec.Destination,
		Options:     containerMountSpec.Options,
		VolumeID:    volumeID,
		VolumeName:  volumeName,
	}, "", nil
}

func isInlineDataVolumeMount(mount containers.ResolvedMountSpec) bool {
	return mount.Kind == containers.MountKindVolume && strings.HasPrefix(mount.VolumeID, constants.InlineDataVolumePrefix)
}

// inlineDataVolumeOptions drops inlineDataVolumeDroppedOptions and adds inlineDataVolumeMountOptions.
func inlineDataVolumeOptions(mountOptions []string) []string {
	options := slices.DeleteFunc(slices.Clone(mountOptions), func(option string) bool {
		return slices.Contains(inlineDataVolumeDroppedOptions, option)
	})

	for _, option := range inlineDataVolumeMountOptions {
		if !slices.Contains(options, option) {
			options = append(options, option)
		}
	}

	return options
}
