// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package containers implements controllers for containers declared via ContainerConfig.
package containers

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/dustin/go-humanize"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// ConfigController projects ContainerConfig documents into ContainerSpec resources.
//
// This controller owns no side effects: it is a pure function of the machine configuration. All it
// does is validate, apply defaults and resolve names, so that every controller downstream works
// against a fully resolved spec and never has to re-read the configuration.
type ConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *ConfigController) Name() string {
	return "containers.ConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: containers.ContainerSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *ConfigController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		}

		if err := ctrl.reconcile(ctx, runtime, logger); err != nil {
			logger.Error("failed to project container configuration", zap.Error(err))

			return err
		}

		runtime.ResetRestartBackoff()
	}
}

//nolint:gocyclo
func (ctrl *ConfigController) reconcile(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	// Read the specs already published, so that appearing and disappearing containers can be told
	// apart from the steady state and logged only once each.
	publishedSpecs, err := safe.ReaderListAll[*containers.ContainerSpec](ctx, runtime)
	if err != nil {
		return fmt.Errorf("failed to list container specs: %w", err)
	}

	stale := map[string]struct{}{}

	for containerSpec := range publishedSpecs.All() {
		stale[containerSpec.Metadata().ID()] = struct{}{}
	}

	runtime.StartTrackingOutputs()

	machineConfig, err := safe.ReaderGetByID[*config.MachineConfig](ctx, runtime, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to get machine config: %w", err)
	}

	if machineConfig != nil {
		for _, containerConfig := range machineConfig.Config().ContainerConfigs() {
			containerSpecID := containerConfig.Name()

			if _, known := stale[containerSpecID]; !known {
				logger.Info(
					"container declared",
					zap.String("container", containerSpecID),
					zap.String("image", containerConfig.Image()),
				)
			}

			delete(stale, containerSpecID)

			if err = safe.WriterModify(
				ctx, runtime,
				containers.NewContainerSpec(containers.NamespaceName, containerSpecID),
				func(res *containers.ContainerSpec) error {
					return applyConfig(res.TypedSpec(), containerConfig)
				},
			); err != nil {
				return fmt.Errorf("failed to write container spec %q: %w", containerSpecID, err)
			}

			logger.Debug("container spec projected", zap.String("container", containerSpecID))
		}
	}

	for containerSpecID := range stale {
		logger.Info("container removed from the machine configuration", zap.String("container", containerSpecID))
	}

	if err := safe.CleanupOutputs[*containers.ContainerSpec](ctx, runtime); err != nil {
		return fmt.Errorf("failed to clean up outputs: %w", err)
	}

	return nil
}

// applyConfig resolves a ContainerConfig document into a ContainerSpec.
func applyConfig(containerSpecSpec *containers.ContainerSpecSpec, containerConfig configcfg.ContainerConfig) error {
	// Image() already returns the canonical form: normalizing in the config layer rather than
	// downstream keeps the reference identical to what the pull records, which is what image
	// garbage collection matches against.
	containerSpecSpec.Image = containers.ContainerImageSpec{Ref: containerConfig.Image()}

	containerSpecSpec.Entrypoint = containerConfig.Entrypoint()
	containerSpecSpec.Args = containerConfig.Args()
	containerSpecSpec.WorkingDir = containerConfig.WorkingDir()

	runAs := containerConfig.RunAs()
	containerSpecSpec.RunAs = containers.ContainerRunAsSpec{
		UID: runAs.UID().Ptr(),
		GID: runAs.GID().Ptr(),
	}

	containerSpecSpec.Environment = containerConfig.Environment()

	containerMountSpecs, err := resolveMounts(containerConfig.Mounts())
	if err != nil {
		return err
	}

	containerSpecSpec.Mounts = containerMountSpecs

	security := containerConfig.Security()
	containerSpecSpec.Security = containers.ContainerSecuritySpec{
		Privileged:       security.Profile() == configcfg.ContainerSecurityProfilePrivileged,
		CapabilitiesAdd:  security.CapabilitiesAdd(),
		CapabilitiesDrop: security.CapabilitiesDrop(),
		MachinedAccess:   security.MachinedAccess(),
	}

	containerSpecSpec.Network = containers.ContainerNetworkSpec{
		HostNetwork: containerConfig.Network().Mode() == configcfg.ContainerNetworkModeHost,
	}

	resources := containerConfig.Resources()
	containerSpecSpec.Resources = containers.ContainerResourcesSpec{
		MemoryLimit: resources.MemoryLimit().ValueOrZero(),
		CPULimit:    resources.CPULimit().ValueOrZero(),
	}

	dependsOn := containerConfig.DependsOn()
	containerSpecSpec.DependsOn = containers.ContainerDependsOnSpec{
		Paths:      dependsOn.Paths(),
		Networks:   dependsOn.Networks(),
		Time:       dependsOn.Time(),
		Containers: dependsOn.Containers(),
	}

	return nil
}

// resolveMounts turns typed configuration mounts into resolved mount specs.
//
// A user volume is resolved to its block volume ID here rather than to a host path: the path is
// only known once the volume is actually mounted, which is ContainerMountController's job.
func resolveMounts(configMounts []configcfg.ContainerMountConfig) ([]containers.ContainerMountSpec, error) {
	if len(configMounts) == 0 {
		return nil, nil
	}

	containerMountSpecs := make([]containers.ContainerMountSpec, 0, len(configMounts))

	for i, containerMountConfig := range configMounts {
		switch {
		case containerMountConfig.UserVolume().IsPresent():
			userVolume, _ := containerMountConfig.UserVolume().Get()

			containerMountSpecs = append(containerMountSpecs, containers.ContainerMountSpec{
				Kind:        containers.MountKindUserVolume,
				VolumeID:    constants.UserVolumePrefix + userVolume.Name(),
				Destination: userVolume.Destination(),
				Options:     userVolume.MountOptions(),
			})
		case containerMountConfig.Tmpfs().IsPresent():
			tmpfs, _ := containerMountConfig.Tmpfs().Get()

			var size uint64

			if tmpfs.Size() != "" {
				var err error

				if size, err = humanize.ParseBytes(tmpfs.Size()); err != nil {
					// Validation has already rejected this, so reaching here means the document
					// bypassed validation somehow; fail loudly rather than silently dropping the size.
					return nil, fmt.Errorf("mounts[%d]: invalid tmpfs size %q: %w", i, tmpfs.Size(), err)
				}
			}

			containerMountSpecs = append(containerMountSpecs, containers.ContainerMountSpec{
				Kind:        containers.MountKindTmpfs,
				Destination: tmpfs.Destination(),
				Size:        size,
				Options:     tmpfs.MountOptions(),
			})
		case containerMountConfig.HostPath().IsPresent():
			hostPath, _ := containerMountConfig.HostPath().Get()

			containerMountSpecs = append(containerMountSpecs, containers.ContainerMountSpec{
				Kind:        containers.MountKindHostPath,
				Source:      hostPath.Source(),
				Destination: hostPath.Destination(),
				Options:     hostPath.MountOptions(),
			})
		default:
			return nil, fmt.Errorf("mounts[%d]: no containerMountConfig source set", i)
		}
	}

	return containerMountSpecs, nil
}
