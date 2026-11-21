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

	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

// inlineDataVolumeMountOptions are forced on every container mount of an inline data volume.
var inlineDataVolumeMountOptions = []string{"ro", "nosuid", "nodev", "noexec"}

// reconcileInlineDataMount requests a read-only bind of an inline data volume and resolves the mount to it.
func (ctrl *MountController) reconcileInlineDataMount(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	containerSpecID string,
	containerMountSpec containers.ContainerMountSpec,
	backing configcfg.BackingVolume,
	isLive bool,
	wanted map[string]struct{},
) (containers.ResolvedMountSpec, string, error) {
	volumeName := containerMountSpec.VolumeName
	requestID := configcfg.InlineDataVolumeMountID(containerSpecID, volumeName)

	request := files.NewInlineDataVolumeMountRequest(files.NamespaceName, requestID)
	wanted[requestKey(request.Metadata())] = struct{}{}

	if err := safe.WriterModify(ctx, runtime, request,
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
		reason, releaseErr := ctrl.releaseInlineDataMount(ctx, runtime, logger, containerSpecID, volumeName, mountStatus, isLive)

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
		VolumeID:    backing.ID,
		VolumeName:  volumeName,
	}, "", nil
}

// releaseInlineDataMount releases a bind being torn down, unless the container is still running.
func (ctrl *MountController) releaseInlineDataMount(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	containerSpecID, volumeName string,
	mountStatus *files.InlineDataVolumeMountStatus,
	isLive bool,
) (string, error) {
	if !mountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
		return fmt.Sprintf("inline data volume %q is being unmounted", volumeName), nil
	}

	// isLive was read at the start of the pass, so an instance started since would be missed.
	if !isLive {
		live, err := ctrl.liveContainers(ctx, runtime)
		if err != nil {
			return "", err
		}

		_, isLive = live[containerSpecID]
	}

	if isLive {
		return fmt.Sprintf("inline data volume %q is pending unmount, waiting for the container to stop", volumeName), nil
	}

	if err := ctrl.releaseFinalizer(ctx, runtime, logger, mountStatus.Metadata()); err != nil {
		return "", err
	}

	return fmt.Sprintf("inline data volume %q is being unmounted", volumeName), nil
}

func isInlineDataVolumeMount(mount containers.ResolvedMountSpec) bool {
	return mount.Kind == containers.MountKindVolume && strings.HasPrefix(mount.VolumeID, constants.InlineDataVolumePrefix)
}

// inlineDataVolumeOptions drops `rw` and adds inlineDataVolumeMountOptions.
func inlineDataVolumeOptions(mountOptions []string) []string {
	options := slices.DeleteFunc(slices.Clone(mountOptions), func(option string) bool {
		return option == "rw"
	})

	for _, option := range inlineDataVolumeMountOptions {
		if !slices.Contains(options, option) {
			options = append(options, option)
		}
	}

	return options
}
