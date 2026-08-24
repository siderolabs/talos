// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// RestartInterval is how long to wait after an instance terminates before starting the next one.
const RestartInterval = 5 * time.Second

type InstanceController struct{}

// Name implements controller.Controller interface.
func (ctrl *InstanceController) Name() string {
	return "containers.InstanceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *InstanceController) Inputs() []controller.Input {
	return append(containerCreationGateInputs(),
		// Restarts are paced off the previous instance's outcome.
		controller.Input{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceStatusType,
			Kind:      controller.InputWeak,
		},
		// This controller's own output: DestroyReady, because an instance being replaced is torn
		// down here and must not be destroyed until RuntimeController has released it.
		controller.Input{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceSpecType,
			Kind:      controller.InputDestroyReady,
		},
	)
}

// Outputs implements controller.Controller interface.
func (ctrl *InstanceController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: containers.ContainerInstanceSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *InstanceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	return runWithWakeTimer(ctx, r, func(ctx context.Context, r controller.Runtime) (optional.Optional[time.Duration], error) {
		wakeAfter, err := ctrl.reconcile(ctx, r, logger)
		if err != nil {
			logger.Error("failed to reconcile container instances", zap.Error(err))
		}

		return wakeAfter, err
	})
}

// reconcile returns how long until the controller next needs to wake up on its own, if at all.
//
//nolint:gocyclo,cyclop
func (ctrl *InstanceController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) (optional.Optional[time.Duration], error) {
	containerSpecs, err := safe.ReaderListAll[*containers.ContainerSpec](ctx, r)
	if err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to list container specs: %w", err)
	}

	constainerInstanceSpecs, err := safe.ReaderListAll[*containers.ContainerInstanceSpec](ctx, r)
	if err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to list container instances: %w", err)
	}

	// Group instances by owning container so each container can be reasoned about independently.
	idToInstanceSpecs := map[string][]*containers.ContainerInstanceSpec{}

	for instanceSpec := range constainerInstanceSpecs.All() {
		containerID := instanceSpec.TypedSpec().ContainerID
		idToInstanceSpecs[containerID] = append(idToInstanceSpecs[containerID], instanceSpec)
	}

	for _, instanceSpecs := range idToInstanceSpecs {
		slices.SortFunc(instanceSpecs, func(a, b *containers.ContainerInstanceSpec) int {
			return int(a.TypedSpec().Generation) - int(b.TypedSpec().Generation)
		})
	}

	// Informs the controller when to next wake up on its own.
	var wakeCtrlAfter optional.Optional[time.Duration]

	wantedContainers := map[string]struct{}{}

	for containerSpec := range containerSpecs.All() {
		wantedContainers[containerSpec.Metadata().ID()] = struct{}{}

		wakeAfter, err := ctrl.reconcileInstance(ctx, r, logger, containerSpec, idToInstanceSpecs[containerSpec.Metadata().ID()])
		if err != nil {
			return optional.None[time.Duration](), err
		}

		wakeCtrlAfter = minOptionalDuration(wakeCtrlAfter, wakeAfter)
	}

	if err := ctrl.destroyOrphanedInstances(ctx, r, logger, idToInstanceSpecs, wantedContainers); err != nil {
		return optional.None[time.Duration](), err
	}

	return wakeCtrlAfter, nil
}

//nolint:gocyclo,cyclop
func (ctrl *InstanceController) reconcileInstance(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	containerSpec *containers.ContainerSpec,
	instances []*containers.ContainerInstanceSpec,
) (optional.Optional[time.Duration], error) {
	var currentInstance *containers.ContainerInstanceSpec
	if len(instances) > 0 {
		currentInstance = instances[len(instances)-1]
	}

	containerSpecID := containerSpec.Metadata().ID()

	nextGeneration := uint64(0)

	// Babysit the existing instance until the spec changes.
	if currentInstance != nil {
		wasDestroyed, wakeUpAfter, err := ctrl.reconcileExistingInstance(ctx, r, logger, containerSpec, currentInstance)
		if err != nil {
			return optional.None[time.Duration](), err
		}

		if !wasDestroyed {
			return wakeUpAfter, nil
		}

		nextGeneration = currentInstance.TypedSpec().Generation + 1
	}

	// No container exists now, but dependencies may be unmet.
	waitingFor, wakeUpAfter, err := containerSpec.TypedSpec().Ready(ctx, r, containerSpecID)
	if err != nil {
		return optional.None[time.Duration](), err
	}

	if len(waitingFor) > 0 {
		logger.Debug("container is waiting on dependencies",
			zap.String("container", containerSpecID),
			zap.Strings("waitingFor", waitingFor),
		)

		return wakeUpAfter, nil
	}

	// We're good to create a new instance.
	imageDigest, err := containers.GetImageDigest(ctx, r, containerSpecID, containerSpec.TypedSpec().Image.Ref)
	if err != nil {
		return optional.None[time.Duration](), err
	}

	resolvedMounts, err := containerSpec.TypedSpec().GetResolvedMounts(ctx, r, containerSpecID)
	if err != nil {
		return optional.None[time.Duration](), err
	}

	if err := ctrl.createInstanceSpec(ctx, r, containerSpec, nextGeneration, imageDigest, resolvedMounts); err != nil {
		return optional.None[time.Duration](), err
	}

	logger.Info("container instance created", zap.String("container", containerSpecID), zap.Uint64("generation", nextGeneration), zap.String("image", imageDigest))

	return optional.None[time.Duration](), nil
}

// destroyOrphanedInstances removes instances for containers whose spec no longer exists.
func (ctrl *InstanceController) destroyOrphanedInstances(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	idToInstanceSpecs map[string][]*containers.ContainerInstanceSpec,
	wantedContainers map[string]struct{},
) error {
	for containerID, list := range idToInstanceSpecs {
		if _, exists := wantedContainers[containerID]; exists {
			continue
		}

		for _, instance := range list {
			logger.Debug("removing instance of a deleted container",
				zap.String("container", containerID),
				zap.String("instance", instance.Metadata().ID()),
			)

			if _, err := ctrl.destroyInstance(ctx, r, logger, instance); err != nil {
				return err
			}
		}
	}

	return nil
}

// destroyInstance tears down an instance, reporting whether it is now gone.
//
// A false return means something still holds a finalizer on it, i.e. it is being stopped.
func (ctrl *InstanceController) destroyInstance(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	instance *containers.ContainerInstanceSpec,
) (bool, error) {
	id := instance.Metadata().ID()

	okToDestroy, err := r.Teardown(ctx, instance.Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return true, nil
		}

		return false, fmt.Errorf("failed to tear down instance %q: %w", id, err)
	}

	if !okToDestroy {
		// Something still holds a finalizer, i.e. it is stopping the task. Come back when it
		// releases, which the InputDestroyReady input will wake us for.
		logger.Debug("waiting for the container instance to stop", zap.String("instance", id))

		return false, nil
	}

	if err := r.Destroy(ctx, instance.Metadata()); err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to destroy instance %q: %w", id, err)
	}

	logger.Debug("container instance destroyed", zap.String("instance", id))

	return true, nil
}

// reconcileExistingInstance checks whether the next generation should be created now.
//
// Returns (proceed, wakeUpAfter, error). A false proceed means the instance is to be left where it
// is for now, either because it matches the spec, because it terminated but the restart interval has
// not elapsed, or because its replacement cannot start yet. A true proceed means it is gone: a
// replacement is only ever created once the instance it replaces has been destroyed, so a container
// has at most one instance at a time and no terminated ones are kept around.
func (ctrl *InstanceController) reconcileExistingInstance(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	spec *containers.ContainerSpec,
	newestInstance *containers.ContainerInstanceSpec,
) (bool, optional.Optional[time.Duration], error) {
	containerID := spec.Metadata().ID()

	// An instance already being torn down is finished regardless of the spec.
	if newestInstance.Metadata().Phase() != resource.PhaseTearingDown {
		inSync, err := newestInstance.TypedSpec().InSyncWithContainerSpec(ctx, r, spec.TypedSpec())
		if err != nil {
			return false, optional.None[time.Duration](), err
		}

		if inSync {
			restartDue, wakeUpAfter, err := ctrl.checkRestartDue(ctx, r, newestInstance)
			if err != nil {
				return false, optional.None[time.Duration](), err
			}

			if !restartDue {
				return false, wakeUpAfter, nil
			}

			logger.Info("container terminated, restart interval elapsed, replacing the instance",
				zap.String("container", containerID),
				zap.Uint64("generation", newestInstance.TypedSpec().Generation),
			)
		} else {
			// A spec change invalidates the existing instance.
			waitingFor, wakeUpAfter, err := spec.TypedSpec().Ready(ctx, r, containerID)
			if err != nil {
				return false, optional.None[time.Duration](), err
			}

			if len(waitingFor) > 0 {
				logger.Debug("container spec changed, but its replacement is waiting on dependencies",
					zap.String("container", containerID),
					zap.Strings("waitingFor", waitingFor),
				)

				return false, wakeUpAfter, nil
			}

			logger.Info("container spec changed, replacing the instance",
				zap.String("container", containerID),
				zap.Uint64("generation", newestInstance.TypedSpec().Generation),
			)
		}
	}

	destroyed, err := ctrl.destroyInstance(ctx, r, logger, newestInstance)
	if err != nil {
		return false, optional.None[time.Duration](), err
	}

	if !destroyed {
		// Still tearing down. InputDestroyReady wakes us when it is gone, and this pass repeats
		// with the same outcome until then.
		return false, optional.None[time.Duration](), nil
	}

	return true, optional.None[time.Duration](), nil
}

// checkRestartDue reports whether a terminated instance should be replaced now.
//
// A false result with no wake time means the instance has no status yet, or has one that is not
// done, i.e. it is still starting or running. A false result with a wake time means it terminated,
// but RestartInterval has not yet elapsed since it did.
func (ctrl *InstanceController) checkRestartDue(
	ctx context.Context,
	r controller.Reader,
	instance *containers.ContainerInstanceSpec,
) (bool, optional.Optional[time.Duration], error) {
	status, err := safe.ReaderGetByID[*containers.ContainerInstanceStatus](ctx, r, instance.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, optional.None[time.Duration](), nil
		}

		return false, optional.None[time.Duration](), fmt.Errorf("failed to get instance status %q: %w", instance.Metadata().ID(), err)
	}

	if !status.TypedSpec().Phase.Done() {
		return false, optional.None[time.Duration](), nil
	}

	if remaining := RestartInterval - time.Since(status.TypedSpec().FinishedAt); remaining > 0 {
		return false, optional.Some(remaining), nil
	}

	return true, optional.None[time.Duration](), nil
}

// createInstanceSpec creates a new ContainerInstanceSpec with all fields populated from the spec and resolved values.
func (ctrl *InstanceController) createInstanceSpec(
	ctx context.Context,
	runtime controller.Runtime,
	containerSpec *containers.ContainerSpec,
	generation uint64,
	digest string,
	mounts []containers.ResolvedMountSpec,
) error {
	containerSpecID := containerSpec.Metadata().ID()
	instanceID := containers.InstanceID(containerSpecID, generation)

	return safe.WriterModify(ctx, runtime,
		containers.NewContainerInstanceSpec(containers.NamespaceName, instanceID),
		func(res *containers.ContainerInstanceSpec) error {
			instanceSpec := res.TypedSpec()
			instanceSpec.ContainerID = containerSpecID
			instanceSpec.Generation = generation
			instanceSpec.Image = digest
			instanceSpec.Entrypoint = containerSpec.TypedSpec().Entrypoint
			instanceSpec.Args = containerSpec.TypedSpec().Args
			instanceSpec.WorkingDir = containerSpec.TypedSpec().WorkingDir
			instanceSpec.RunAs = containerSpec.TypedSpec().RunAs
			instanceSpec.Environment = containerSpec.TypedSpec().Environment
			instanceSpec.Mounts = mounts
			instanceSpec.Security = containerSpec.TypedSpec().Security
			instanceSpec.Network = containerSpec.TypedSpec().Network
			instanceSpec.Resources = containerSpec.TypedSpec().Resources

			return nil
		},
	)
}

// minOptionalDuration returns the smaller of two optional durations, ignoring any that are not set.
func minOptionalDuration(a, b optional.Optional[time.Duration]) optional.Optional[time.Duration] {
	av, aok := a.Get()
	bv, bok := b.Get()

	switch {
	case !aok:
		return b
	case !bok:
		return a
	case bv < av:
		return b
	default:
		return a
	}
}
