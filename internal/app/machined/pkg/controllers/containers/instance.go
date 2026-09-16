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
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	timeres "github.com/siderolabs/talos/pkg/machinery/resources/time"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// RestartInterval is how long to wait after an instance terminates before starting the next one.
const RestartInterval = 5 * time.Second

// InstanceController creates and replaces the ContainerInstanceSpec for each container, one
// generation per execution.
type InstanceController struct{}

// Name implements controller.Controller interface.
func (ctrl *InstanceController) Name() string {
	return "containers.InstanceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *InstanceController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerSpecType,
			Kind:      controller.InputWeak,
		},
		// Gates on the image having a usable digest.
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerImageStatusType,
			Kind:      controller.InputWeak,
		},
		// Gates on the declared mounts having been resolved.
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerMountStatusType,
			Kind:      controller.InputWeak,
		},
		// Gates on dependsOn.containers: another container's aggregated Health.
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerStatusType,
			Kind:      controller.InputWeak,
		},
		// Gate on dependsOn.networks.
		{
			Namespace: network.NamespaceName,
			Type:      network.StatusType,
			ID:        optional.Some(network.StatusID),
			Kind:      controller.InputWeak,
		},
		// Gate on dependsOn.time.
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      timeres.StatusType,
			ID:        optional.Some(timeres.StatusID),
			Kind:      controller.InputWeak,
		},
		// Restarts are paced off the previous instance's outcome.
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceStatusType,
			Kind:      controller.InputWeak,
		},
		// This controller's own output: DestroyReady, because an instance being replaced is torn
		// down here and must not be destroyed until RuntimeController has released it.
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceSpecType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *InstanceController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: containers.ContainerInstanceSpecType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: containers.ContainerDependencyStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *InstanceController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	return runWithWakeTimer(ctx, runtime, func(ctx context.Context, runtime controller.Runtime) (optional.Optional[time.Duration], error) {
		wakeAfter, err := ctrl.reconcile(ctx, runtime, logger)
		if err != nil {
			logger.Error("failed to reconcile container instances", zap.Error(err))
		}

		return wakeAfter, err
	})
}

// reconcile returns how long until the controller next needs to wake up on its own, if at all.
//
//nolint:gocyclo,cyclop
func (ctrl *InstanceController) reconcile(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) (optional.Optional[time.Duration], error) {
	runtime.StartTrackingOutputs()

	containerSpecs, err := safe.ReaderListAll[*containers.ContainerSpec](ctx, runtime)
	if err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to list container specs: %w", err)
	}

	containerInstanceSpecs, err := safe.ReaderListAll[*containers.ContainerInstanceSpec](ctx, runtime)
	if err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to list container instances: %w", err)
	}

	// Group instances by owning container so each container can be reasoned about independently.
	containerSpecIDToInstanceSpecs := map[string][]*containers.ContainerInstanceSpec{}

	for containerInstanceSpec := range containerInstanceSpecs.All() {
		containerSpecID := containerInstanceSpec.TypedSpec().ContainerID
		containerSpecIDToInstanceSpecs[containerSpecID] = append(containerSpecIDToInstanceSpecs[containerSpecID], containerInstanceSpec)
	}

	for _, containerInstanceSpecs := range containerSpecIDToInstanceSpecs {
		slices.SortFunc(containerInstanceSpecs, func(a, b *containers.ContainerInstanceSpec) int {
			return int(a.TypedSpec().Generation) - int(b.TypedSpec().Generation)
		})
	}

	// Informs the controller when to next wake up on its own.
	var wakeCtrlAfter optional.Optional[time.Duration]

	wantedContainers := map[string]struct{}{}

	for containerSpec := range containerSpecs.All() {
		wantedContainers[containerSpec.Metadata().ID()] = struct{}{}

		waitingFor, wakeAfter, err := ctrl.reconcileInstance(ctx, runtime, logger, containerSpec, containerSpecIDToInstanceSpecs[containerSpec.Metadata().ID()])
		if err != nil {
			return optional.None[time.Duration](), err
		}

		if err := ctrl.publishDependencyStatus(ctx, runtime, containerSpec.Metadata().ID(), waitingFor); err != nil {
			return optional.None[time.Duration](), err
		}

		wakeCtrlAfter = containers.EarliestWakeUpAfter(wakeCtrlAfter, wakeAfter)
	}

	if err := ctrl.destroyOrphanedInstances(ctx, runtime, logger, containerSpecIDToInstanceSpecs, wantedContainers); err != nil {
		return optional.None[time.Duration](), err
	}

	if err := safe.CleanupOutputs[*containers.ContainerDependencyStatus](ctx, runtime); err != nil {
		return optional.None[time.Duration](), fmt.Errorf("failed to clean up outputs: %w", err)
	}

	return wakeCtrlAfter, nil
}

// publishDependencyStatus records what this container's next execution is waiting on, so that
// nothing downstream has to re-derive the same verdict from the same inputs.
func (ctrl *InstanceController) publishDependencyStatus(
	ctx context.Context,
	runtime controller.Runtime,
	containerSpecID string,
	waitingFor []string,
) error {
	if err := safe.WriterModify(ctx, runtime,
		containers.NewContainerDependencyStatus(containers.NamespaceName, containerSpecID),
		func(res *containers.ContainerDependencyStatus) error {
			res.Metadata().Labels().Set(containers.ContainerSpecIdLabel, containerSpecID)

			res.TypedSpec().WaitingFor = waitingFor

			return nil
		},
	); err != nil {
		return fmt.Errorf("failed to write dependency status %q: %w", containerSpecID, err)
	}

	return nil
}

// reconcileInstance returns the gates standing between this container and its next execution, and how long until the controller needs to look again.
//
//nolint:gocyclo,cyclop
func (ctrl *InstanceController) reconcileInstance(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	containerSpec *containers.ContainerSpec,
	containerInstanceSpecs []*containers.ContainerInstanceSpec,
) ([]string, optional.Optional[time.Duration], error) {
	var currentContainerInstanceSpec *containers.ContainerInstanceSpec
	if len(containerInstanceSpecs) > 0 {
		currentContainerInstanceSpec = containerInstanceSpecs[len(containerInstanceSpecs)-1]
	}

	containerSpecID := containerSpec.Metadata().ID()

	nextGeneration := uint64(0)

	// Babysit the existing instance until the spec changes.
	if currentContainerInstanceSpec != nil {
		wasDestroyed, waitingFor, wakeUpAfter, err := ctrl.reconcileExistingInstance(ctx, runtime, logger, containerSpec, currentContainerInstanceSpec)
		if err != nil {
			return nil, optional.None[time.Duration](), err
		}

		if !wasDestroyed {
			return waitingFor, wakeUpAfter, nil
		}

		nextGeneration = currentContainerInstanceSpec.TypedSpec().Generation + 1
	}

	// No container exists now, but dependencies may be unmet.
	waitingFor, wakeUpAfter, err := containerSpec.TypedSpec().Ready(ctx, runtime, containerSpecID)
	if err != nil {
		return nil, optional.None[time.Duration](), err
	}

	if len(waitingFor) > 0 {
		logger.Debug("container is waiting on dependencies",
			zap.String("container", containerSpecID),
			zap.Strings("waitingFor", waitingFor),
		)

		return waitingFor, wakeUpAfter, nil
	}

	// We're good to create a new instance.
	imageDigest, err := containers.GetImageDigest(ctx, runtime, containerSpecID, containerSpec.TypedSpec().Image.Ref)
	if err != nil {
		return nil, optional.None[time.Duration](), err
	}

	resolvedMounts, err := containerSpec.TypedSpec().GetResolvedMounts(ctx, runtime, containerSpecID)
	if err != nil {
		return nil, optional.None[time.Duration](), err
	}

	if err := ctrl.createInstanceSpec(ctx, runtime, containerSpec, nextGeneration, imageDigest, resolvedMounts); err != nil {
		return nil, optional.None[time.Duration](), err
	}

	logger.Info("container instance created", zap.String("container", containerSpecID), zap.Uint64("generation", nextGeneration), zap.String("image", imageDigest))

	return nil, optional.None[time.Duration](), nil
}

// destroyOrphanedInstances removes instances for containers whose spec no longer exists.
func (ctrl *InstanceController) destroyOrphanedInstances(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	containerSpecIDToInstanceSpecs map[string][]*containers.ContainerInstanceSpec,
	wantedContainers map[string]struct{},
) error {
	for containerSpecID, containerInstanceSpecs := range containerSpecIDToInstanceSpecs {
		if _, exists := wantedContainers[containerSpecID]; exists {
			continue
		}

		for _, containerInstanceSpec := range containerInstanceSpecs {
			logger.Debug("removing instance of a deleted container",
				zap.String("container", containerSpecID),
				zap.String("instance", containerInstanceSpec.Metadata().ID()),
			)

			if _, err := ctrl.destroyInstance(ctx, runtime, logger, containerInstanceSpec); err != nil {
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
	runtime controller.Runtime,
	logger *zap.Logger,
	containerInstanceSpec *containers.ContainerInstanceSpec,
) (bool, error) {
	instanceSpecID := containerInstanceSpec.Metadata().ID()

	okToDestroy, err := runtime.Teardown(ctx, containerInstanceSpec.Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return true, nil
		}

		return false, fmt.Errorf("failed to tear down instance %q: %w", instanceSpecID, err)
	}

	if !okToDestroy {
		// Something still holds a finalizer, i.e. it is stopping the task. Come back when it
		// releases, which the InputDestroyReady input will wake us for.
		logger.Debug("waiting for the container instance to stop", zap.String("instance", instanceSpecID))

		return false, nil
	}

	if err := runtime.Destroy(ctx, containerInstanceSpec.Metadata()); err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to destroy instance %q: %w", instanceSpecID, err)
	}

	logger.Debug("container instance destroyed", zap.String("instance", instanceSpecID))

	return true, nil
}

// reconcileExistingInstance checks whether the next generation should be created now.
//
// Returns (proceed, waitingFor, wakeUpAfter, error). A false proceed means the instance is to be
// left where it is for now, either because it matches the spec, because it terminated but the
// restart interval has not elapsed, or because its replacement cannot start yet. A true proceed
// means it is gone: a replacement is only ever created once the instance it replaces has been
// destroyed, so a container has at most one instance at a time and no terminated ones are kept
// around.
//
// waitingFor is populated only on the one path that evaluates the gates, i.e. a spec change whose
// replacement is blocked. Every other path leaves the running instance alone and so is not waiting
// on anything.
func (ctrl *InstanceController) reconcileExistingInstance(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	containerSpec *containers.ContainerSpec,
	newestContainerInstanceSpec *containers.ContainerInstanceSpec,
) (bool, []string, optional.Optional[time.Duration], error) {
	containerSpecID := containerSpec.Metadata().ID()

	// An instance already being torn down is finished regardless of the spec.
	if newestContainerInstanceSpec.Metadata().Phase() != resource.PhaseTearingDown {
		inSync, err := newestContainerInstanceSpec.TypedSpec().InSyncWithContainerSpec(ctx, runtime, containerSpec.TypedSpec())
		if err != nil {
			return false, nil, optional.None[time.Duration](), err
		}

		if inSync {
			restartDue, wakeUpAfter, err := ctrl.checkRestartDue(ctx, runtime, newestContainerInstanceSpec)
			if err != nil {
				return false, nil, optional.None[time.Duration](), err
			}

			if !restartDue {
				return false, nil, wakeUpAfter, nil
			}

			logger.Info("container terminated, restart interval elapsed, replacing the instance",
				zap.String("container", containerSpecID),
				zap.Uint64("generation", newestContainerInstanceSpec.TypedSpec().Generation),
			)
		} else {
			// A spec change invalidates the existing instance.
			waitingFor, wakeUpAfter, err := containerSpec.TypedSpec().Ready(ctx, runtime, containerSpecID)
			if err != nil {
				return false, nil, optional.None[time.Duration](), err
			}

			if len(waitingFor) > 0 {
				logger.Debug("container spec changed, but its replacement is waiting on dependencies",
					zap.String("container", containerSpecID),
					zap.Strings("waitingFor", waitingFor),
				)

				return false, waitingFor, wakeUpAfter, nil
			}

			logger.Info("container spec changed, replacing the instance",
				zap.String("container", containerSpecID),
				zap.Uint64("generation", newestContainerInstanceSpec.TypedSpec().Generation),
			)
		}
	}

	destroyed, err := ctrl.destroyInstance(ctx, runtime, logger, newestContainerInstanceSpec)
	if err != nil {
		return false, nil, optional.None[time.Duration](), err
	}

	if !destroyed {
		// Still tearing down. InputDestroyReady wakes us when it is gone, and this pass repeats
		// with the same outcome until then.
		return false, nil, optional.None[time.Duration](), nil
	}

	return true, nil, optional.None[time.Duration](), nil
}

// checkRestartDue reports whether a terminated instance should be replaced now.
//
// A false result with no wake time means the instance has no status yet, or has one that is not
// done, i.e. it is still starting or running. A false result with a wake time means it terminated,
// but RestartInterval has not yet elapsed since it did.
func (ctrl *InstanceController) checkRestartDue(
	ctx context.Context,
	reader controller.Reader,
	containerInstanceSpec *containers.ContainerInstanceSpec,
) (bool, optional.Optional[time.Duration], error) {
	containerInstanceStatus, err := safe.ReaderGetByID[*containers.ContainerInstanceStatus](ctx, reader, containerInstanceSpec.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, optional.None[time.Duration](), nil
		}

		return false, optional.None[time.Duration](), fmt.Errorf("failed to get instance status %q: %w", containerInstanceSpec.Metadata().ID(), err)
	}

	if !containerInstanceStatus.TypedSpec().Phase.Done() {
		return false, optional.None[time.Duration](), nil
	}

	if wakeAfter, ok := containerInstanceStatus.TypedSpec().RestartWindowWakeAfter(RestartInterval).Get(); ok {
		return false, optional.Some(wakeAfter), nil
	}

	return true, optional.None[time.Duration](), nil
}

// createInstanceSpec creates a new ContainerInstanceSpec with all fields populated from the spec and resolved values.
func (ctrl *InstanceController) createInstanceSpec(
	ctx context.Context,
	runtime controller.Runtime,
	containerSpec *containers.ContainerSpec,
	generation uint64,
	imageDigest string,
	resolvedMounts []containers.ResolvedMountSpec,
) error {
	containerSpecID := containerSpec.Metadata().ID()
	instanceID := containers.InstanceID(containerSpecID, generation)

	return safe.WriterModify(ctx, runtime,
		containers.NewContainerInstanceSpec(containers.NamespaceName, instanceID),
		func(res *containers.ContainerInstanceSpec) error {
			res.Metadata().Labels().Set(containers.ContainerSpecIdLabel, containerSpecID)

			instanceSpec := res.TypedSpec()
			instanceSpec.ContainerID = containerSpecID
			instanceSpec.Generation = generation
			instanceSpec.Image = imageDigest
			instanceSpec.Entrypoint = containerSpec.TypedSpec().Entrypoint
			instanceSpec.Args = containerSpec.TypedSpec().Args
			instanceSpec.WorkingDir = containerSpec.TypedSpec().WorkingDir
			instanceSpec.RunAs = containerSpec.TypedSpec().RunAs
			instanceSpec.Environment = containerSpec.TypedSpec().Environment
			instanceSpec.Mounts = resolvedMounts
			instanceSpec.Security = containerSpec.TypedSpec().Security
			instanceSpec.Network = containerSpec.TypedSpec().Network
			instanceSpec.Resources = containerSpec.TypedSpec().Resources

			return nil
		},
	)
}
