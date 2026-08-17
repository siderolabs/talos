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

// InstanceController decides when a container should be running.
//
// It owns no side effects and holds no containerd client or goroutine: given a spec and the gating
// statuses, it decides whether a ContainerInstanceSpec should exist. That makes dependency gating
// testable without any infrastructure.
//
// Three gates the RFD describes are not yet enforced here: a userVolume mount (no MountController
// yet to resolve its host path, so a container declaring one simply stays pending),
// dependsOn.containers (no aggregated ContainerStatus yet to check a peer's health against), and
// restart-after-termination (no ContainerInstanceStatus producer yet, so a generation only advances
// on a spec change). All three land with their respective controllers.
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
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerImageStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceSpecType,
			Kind:      controller.InputDestroyReady,
		},
		// Needed to check whether dependsOn is satisfied.
		{
			Namespace: network.NamespaceName,
			Type:      network.StatusType,
			ID:        optional.Some(network.StatusID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      timeres.StatusType,
			ID:        optional.Some(timeres.StatusID),
			Kind:      controller.InputWeak,
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
	}
}

// Run implements controller.Controller interface.
func (ctrl *InstanceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// A single timer serves both the restart delay and path polling: it is reset each pass to the
	// earliest deadline anything is waiting on, so an idle node does no work at all.
	timer := time.NewTimer(0)
	defer timer.Stop()

	if !timer.Stop() {
		<-timer.C
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-timer.C:
		}

		wakeAfter, err := ctrl.reconcile(ctx, r, logger)
		if err != nil {
			logger.Error("failed to reconcile container instances", zap.Error(err))

			return err
		}

		if !timer.Stop() {
			// Drain a timer that fired while we were reconciling, so the next Reset is honored.
			select {
			case <-timer.C:
			default:
			}
		}

		if duration, ok := wakeAfter.Get(); ok {
			timer.Reset(duration)
		}

		r.ResetRestartBackoff()
	}
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
	spec *containers.ContainerSpec,
	instances []*containers.ContainerInstanceSpec,
) (optional.Optional[time.Duration], error) {
	var currentInstance *containers.ContainerInstanceSpec
	if len(instances) > 0 {
		currentInstance = instances[len(instances)-1]
	}

	containerID := spec.Metadata().ID()

	nextGeneration := uint64(0)

	// Babysit the existing instance until the spec changes.
	if currentInstance != nil {
		wasDestroyed, wakeUpAfter, err := ctrl.reconcileExistingInstance(ctx, r, logger, spec, currentInstance)
		if err != nil {
			return optional.None[time.Duration](), err
		}

		if !wasDestroyed {
			return wakeUpAfter, nil
		}

		nextGeneration = currentInstance.TypedSpec().Generation + 1
	}

	// No container exists now, but dependencies may be unmet.
	waitingFor, wakeUpAfter, err := spec.TypedSpec().Ready(ctx, r, containerID)
	if err != nil {
		return optional.None[time.Duration](), err
	}

	if len(waitingFor) > 0 {
		logger.Debug("container is waiting on dependencies",
			zap.String("container", containerID),
			zap.Strings("waitingFor", waitingFor),
		)

		return wakeUpAfter, nil
	}

	// We're good to create a new instance.
	imageDigest, err := containers.GetImageDigest(ctx, r, containerID, spec.TypedSpec().Image.Ref)
	if err != nil {
		return optional.None[time.Duration](), err
	}

	mounts, _ := containers.ResolveInstanceMounts(spec.TypedSpec().Mounts)

	if err := ctrl.createInstanceSpec(ctx, r, containerID, nextGeneration, spec, imageDigest, mounts); err != nil {
		return optional.None[time.Duration](), err
	}

	logger.Info("container instance created", zap.String("container", containerID), zap.Uint64("generation", nextGeneration), zap.String("image", imageDigest))

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

// reconcileExistingInstance checks whether an existing instance is still the one to be running, and
// tears it down if it is not.
//
// Returns (wasDestroyed, wakeUpAfter, error). A false wasDestroyed means the instance is to be left
// where it is for now, either because it matches the spec or because its replacement cannot start
// yet.
func (ctrl *InstanceController) reconcileExistingInstance(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	spec *containers.ContainerSpec,
	newestInstance *containers.ContainerInstanceSpec,
) (bool, optional.Optional[time.Duration], error) {
	containerID := spec.Metadata().ID()

	// An instance already being torn down is finished off whatever the spec now says: the stop was
	// decided in an earlier pass and the container is on its way down, so a spec change that happens
	// to restore the old values cannot take it back. Comparing it against the spec here would report
	// it in sync and strand it tearing down forever.
	if newestInstance.Metadata().Phase() != resource.PhaseTearingDown {
		inSync, err := newestInstance.TypedSpec().InSyncWithContainerSpec(ctx, r, spec.TypedSpec())
		if err != nil {
			return false, optional.None[time.Duration](), err
		}

		if inSync {
			return false, optional.None[time.Duration](), nil
		}

		// A spec change invalidates the existing instance: a running container is never mutated in
		// place, it is replaced. Its replacement has to be startable first, though, otherwise a spec
		// edit made while a gate is unmet stops a healthy container for as long as the gate stays
		// unmet, which for a userVolume mount is forever.
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

// createInstanceSpec creates a new ContainerInstanceSpec with all fields populated from the spec and resolved values.
func (ctrl *InstanceController) createInstanceSpec(
	ctx context.Context,
	r controller.Runtime,
	containerID string,
	generation uint64,
	spec *containers.ContainerSpec,
	digest string,
	mounts []containers.ResolvedMountSpec,
) error {
	instanceID := containers.InstanceID(containerID, generation)

	return safe.WriterModify(ctx, r,
		containers.NewContainerInstanceSpec(containers.NamespaceName, instanceID),
		func(res *containers.ContainerInstanceSpec) error {
			instanceSpec := res.TypedSpec()
			instanceSpec.ContainerID = containerID
			instanceSpec.Generation = generation
			instanceSpec.Image = digest
			instanceSpec.Entrypoint = spec.TypedSpec().Entrypoint
			instanceSpec.Args = spec.TypedSpec().Args
			instanceSpec.WorkingDir = spec.TypedSpec().WorkingDir
			instanceSpec.RunAs = spec.TypedSpec().RunAs
			instanceSpec.Environment = spec.TypedSpec().Environment
			instanceSpec.Mounts = mounts
			instanceSpec.Security = spec.TypedSpec().Security
			instanceSpec.Network = spec.TypedSpec().Network
			instanceSpec.Resources = spec.TypedSpec().Resources

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
