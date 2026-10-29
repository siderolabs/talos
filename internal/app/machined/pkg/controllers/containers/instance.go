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
		// Gate on dependsOn.network.
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
		{
			Type: containers.ContainerRunStatusType,
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

	if err := runtime.CleanupOutputs(ctx,
		resource.NewMetadata(containers.NamespaceName, containers.ContainerDependencyStatusType, "", resource.VersionUndefined),
		resource.NewMetadata(containers.NamespaceName, containers.ContainerRunStatusType, "", resource.VersionUndefined),
	); err != nil {
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

	runStatus, err := ctrl.runStatus(ctx, runtime, containerSpecID)
	if err != nil {
		return nil, optional.None[time.Duration](), err
	}

	// a different execution configuration starts over with the restarts and the backoff
	if executionHash := containerSpec.TypedSpec().ExecutionHash(); runStatus.ExecutionHash != executionHash {
		runStatus = containers.ContainerRunStatusSpec{
			ExecutionHash:  executionHash,
			NextGeneration: runStatus.NextGeneration,
		}
	}

	// Babysit the existing instance until the spec changes.
	if currentContainerInstanceSpec != nil {
		runStatus.NextGeneration = max(runStatus.NextGeneration, currentContainerInstanceSpec.TypedSpec().Generation+1)

		wasDestroyed, waitingFor, wakeUpAfter, err := ctrl.reconcileExistingInstance(ctx, runtime, logger, containerSpec, currentContainerInstanceSpec, &runStatus)
		if err != nil {
			return nil, optional.None[time.Duration](), err
		}

		if !wasDestroyed {
			return waitingFor, wakeUpAfter, ctrl.writeRunStatus(ctx, runtime, containerSpecID, runStatus)
		}
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

		return waitingFor, wakeUpAfter, ctrl.writeRunStatus(ctx, runtime, containerSpecID, runStatus)
	}

	// We're good to create a new instance.
	imageDigest, err := containers.GetImageDigest(ctx, runtime, containerSpecID, containerSpec.TypedSpec().Image.Ref)
	if err != nil {
		return nil, optional.None[time.Duration](), err
	}

	// Checked again: the instance must carry exactly the resolution it was checked against.
	resolved, err := containers.GetResolvedMountStatus(ctx, runtime, containerSpecID)
	if err != nil {
		return nil, optional.None[time.Duration](), err
	}

	if waitingFor = containerSpec.TypedSpec().ResolvedWaitingFor(resolved); len(waitingFor) > 0 {
		return waitingFor, optional.None[time.Duration](), ctrl.writeRunStatus(ctx, runtime, containerSpecID, runStatus)
	}

	generation := runStatus.NextGeneration

	if err := ctrl.createInstanceSpec(ctx, runtime, containerSpec, generation, runStatus.ExecutionHash, imageDigest, resolved); err != nil {
		return nil, optional.None[time.Duration](), err
	}

	logger.Info("container instance created", zap.String("container", containerSpecID), zap.Uint64("generation", generation), zap.String("image", imageDigest))

	runStatus.NextGeneration = generation + 1

	return nil, optional.None[time.Duration](), ctrl.writeRunStatus(ctx, runtime, containerSpecID, runStatus)
}

// runStatus returns the restart accounting of a container, empty if there is none yet.
func (ctrl *InstanceController) runStatus(ctx context.Context, reader controller.Reader, containerSpecID string) (containers.ContainerRunStatusSpec, error) {
	runStatus, err := safe.ReaderGetByID[*containers.ContainerRunStatus](ctx, reader, containerSpecID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return containers.ContainerRunStatusSpec{}, nil
		}

		return containers.ContainerRunStatusSpec{}, fmt.Errorf("failed to get run status %q: %w", containerSpecID, err)
	}

	return *runStatus.TypedSpec(), nil
}

func (ctrl *InstanceController) writeRunStatus(ctx context.Context, runtime controller.Runtime, containerSpecID string, runStatus containers.ContainerRunStatusSpec) error {
	if err := safe.WriterModify(ctx, runtime,
		containers.NewContainerRunStatus(containers.NamespaceName, containerSpecID),
		func(res *containers.ContainerRunStatus) error {
			*res.TypedSpec() = runStatus

			return nil
		},
	); err != nil {
		return fmt.Errorf("failed to write run status %q: %w", containerSpecID, err)
	}

	return nil
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

		allDestroyed := true

		for _, containerInstanceSpec := range containerInstanceSpecs {
			logger.Debug("removing instance of a deleted container",
				zap.String("container", containerSpecID),
				zap.String("instance", containerInstanceSpec.Metadata().ID()),
			)

			destroyed, err := ctrl.destroyInstance(ctx, runtime, logger, containerInstanceSpec)
			if err != nil {
				return err
			}

			allDestroyed = allDestroyed && destroyed
		}

		// the generations are kept until the last instance is gone, so a container declared again
		// meanwhile doesn't reuse one
		if !allDestroyed {
			runStatus, err := ctrl.runStatus(ctx, runtime, containerSpecID)
			if err != nil {
				return err
			}

			if err := ctrl.writeRunStatus(ctx, runtime, containerSpecID, runStatus); err != nil {
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
// left where it is for now: it matches the spec, it finished and isn't restarted (yet), or its
// replacement cannot start yet. A true proceed means it is gone: a replacement is only ever created
// once the instance it replaces has been destroyed, so a container has at most one instance at a
// time.
//
// waitingFor is populated only on the one path that evaluates the gates, i.e. a spec change whose
// replacement is blocked. Every other path leaves the running instance alone and so is not waiting
// on anything.
//
//nolint:gocyclo
func (ctrl *InstanceController) reconcileExistingInstance(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	containerSpec *containers.ContainerSpec,
	newestContainerInstanceSpec *containers.ContainerInstanceSpec,
	runStatus *containers.ContainerRunStatusSpec,
) (bool, []string, optional.Optional[time.Duration], error) {
	containerSpecID := containerSpec.Metadata().ID()

	// An instance already being torn down is finished regardless of the spec.
	if newestContainerInstanceSpec.Metadata().Phase() != resource.PhaseTearingDown {
		inSync, err := newestContainerInstanceSpec.TypedSpec().InSyncWithContainerSpec(ctx, runtime, containerSpec.TypedSpec())
		if err != nil {
			return false, nil, optional.None[time.Duration](), err
		}

		// only a change of the declared configuration replaces a finished instance, drift of what it
		// resolves to waits for the restart policy
		declaredChange := newestContainerInstanceSpec.TypedSpec().ExecutionHash != runStatus.ExecutionHash

		var finished *containers.ContainerInstanceStatus

		if !declaredChange {
			finished, err = finishedInstanceStatus(ctx, runtime, newestContainerInstanceSpec)
			if err != nil {
				return false, nil, optional.None[time.Duration](), err
			}
		}

		switch {
		case finished != nil:
			restartDue, wakeUpAfter, err := ctrl.checkRestartDue(ctx, runtime, containerSpec, finished, runStatus)
			if err != nil {
				return false, nil, optional.None[time.Duration](), err
			}

			if !restartDue {
				return false, nil, wakeUpAfter, nil
			}

			logger.Info("container finished, restarting it",
				zap.String("container", containerSpecID),
				zap.Uint64("generation", newestContainerInstanceSpec.TypedSpec().Generation),
				zap.Uint32("restarts", runStatus.Restarts),
			)
		case inSync && !declaredChange:
			return false, nil, optional.None[time.Duration](), nil
		default:
			// A spec change invalidates the existing instance.
			waitingFor, wakeUpAfter, err := containerSpec.TypedSpec().Ready(ctx, runtime, containerSpecID)
			if err != nil {
				return false, nil, optional.None[time.Duration](), err
			}

			// A raw volume the spec no longer hands to the container is given up right away, rather than
			// once the replacement is ready: the replacement may be waiting for a raw volume which this
			// instance's hold is keeping from another container.
			revoked := revokedDevices(containerSpec.TypedSpec(), newestContainerInstanceSpec.TypedSpec())

			if len(waitingFor) > 0 && len(revoked) == 0 {
				logger.Debug("container spec changed, but its replacement is waiting on dependencies",
					zap.String("container", containerSpecID),
					zap.Strings("waitingFor", waitingFor),
				)

				return false, waitingFor, wakeUpAfter, nil
			}

			if len(revoked) > 0 {
				logger.Info("raw volumes are no longer handed to the container, stopping the instance",
					zap.String("container", containerSpecID),
					zap.Uint64("generation", newestContainerInstanceSpec.TypedSpec().Generation),
					zap.Strings("volumes", revoked),
				)
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

// finishedInstanceStatus returns the status of an instance which has finished, nil while it hasn't.
func finishedInstanceStatus(
	ctx context.Context,
	reader controller.Reader,
	containerInstanceSpec *containers.ContainerInstanceSpec,
) (*containers.ContainerInstanceStatus, error) {
	containerInstanceStatus, err := safe.ReaderGetByID[*containers.ContainerInstanceStatus](ctx, reader, containerInstanceSpec.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get instance status %q: %w", containerInstanceSpec.Metadata().ID(), err)
	}

	if !containerInstanceStatus.TypedSpec().Phase.Done() {
		return nil, nil
	}

	return containerInstanceStatus, nil
}

// checkRestartDue reports whether a finished instance is to be replaced now, charging the restart.
//
// The restart is charged once, and recorded before the instance is destroyed: a pass which fails or
// waits on gates after that doesn't charge it again. A false result with a wake time means the
// restart is waiting out its delay, one without means the restart policy doesn't restart it.
func (ctrl *InstanceController) checkRestartDue(
	ctx context.Context,
	runtime controller.Runtime,
	containerSpec *containers.ContainerSpec,
	containerInstanceStatus *containers.ContainerInstanceStatus,
	runStatus *containers.ContainerRunStatusSpec,
) (bool, optional.Optional[time.Duration], error) {
	instanceStatus := *containerInstanceStatus.TypedSpec()

	if runStatus.RestartCharged(instanceStatus.Generation) {
		return true, optional.None[time.Duration](), nil
	}

	restart := containerSpec.TypedSpec().Restart

	if !restart.RestartPermitted(*runStatus, instanceStatus) {
		return false, optional.None[time.Duration](), nil
	}

	step, delay := restart.RestartBackoff(*runStatus, instanceStatus)

	if remaining := delay - time.Since(instanceStatus.FinishedAt); remaining > 0 {
		return false, optional.Some(remaining), nil
	}

	runStatus.Restarts++
	runStatus.BackoffStep = step + 1
	runStatus.Restarted = true
	runStatus.RestartedGeneration = instanceStatus.Generation

	return true, optional.None[time.Duration](), ctrl.writeRunStatus(ctx, runtime, containerSpec.Metadata().ID(), *runStatus)
}

// createInstanceSpec creates a new ContainerInstanceSpec with all fields populated from the spec and resolved values.
func (ctrl *InstanceController) createInstanceSpec(
	ctx context.Context,
	runtime controller.Runtime,
	containerSpec *containers.ContainerSpec,
	generation uint64,
	executionHash string,
	imageDigest string,
	resolved containers.ContainerMountStatusSpec,
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
			instanceSpec.Mounts = resolved.Mounts
			instanceSpec.Devices = resolved.Devices
			instanceSpec.Security = containerSpec.TypedSpec().Security
			instanceSpec.Network = containerSpec.TypedSpec().Network
			instanceSpec.Resources = containerSpec.TypedSpec().Resources
			instanceSpec.ExecutionHash = executionHash

			return nil
		},
	)
}

// revokedDevices returns the raw volumes the instance uses which the spec no longer hands to the container.
func revokedDevices(containerSpec *containers.ContainerSpecSpec, instanceSpec *containers.ContainerInstanceSpecSpec) []string {
	var revoked []string

	for _, device := range instanceSpec.Devices {
		if !slices.ContainsFunc(containerSpec.Devices, func(declared containers.ContainerDeviceSpec) bool {
			return declared.Kind == device.Kind && declared.VolumeName == device.VolumeName
		}) {
			revoked = append(revoked, device.VolumeName)
		}
	}

	return revoked
}
