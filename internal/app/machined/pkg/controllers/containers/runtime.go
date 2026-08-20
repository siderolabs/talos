// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// TaskRunner runs one container execution against a container runtime.
//
// Everything platform-specific lives behind this interface: the containerd client, the cgroup, the
// OCI spec, the signal sequence on teardown. The controller above it only orchestrates. That split
// is also what makes the controller testable without containerd.
type TaskRunner interface {
	// List returns the IDs of containers currently present in the namespace.
	//
	// Used for the orphan sweep: containerd's state is persistent, so containers can outlive the
	// process that created them.
	List(ctx context.Context) ([]string, error)

	// Remove deletes a container along with its snapshot, tolerating absence.
	Remove(ctx context.Context, id string) error

	// Run creates the container and blocks until its task exits.
	//
	// started is called once with the task PID. Canceling ctx must stop the task gracefully
	// (SIGTERM, grace period, SIGKILL) and clean up everything it created, using a context that
	// outlives the cancellation.
	Run(ctx context.Context, id string, spec containers.ContainerInstanceSpecSpec, started func(pid uint32)) (exitCode int32, err error)

	// Close releases the underlying client.
	Close() error
}

// RuntimeController runs container instances by interacting with the container runtime.
type RuntimeController struct {
	// Runtime provides the logging manager for container logs.
	Runtime runtime.Runtime

	// RunnerProvider is overridable for testing.
	RunnerProvider func() (TaskRunner, error)

	instances map[string]*instanceRunState
	swept     bool
}

// Name implements controller.Controller interface.
func (ctrl *RuntimeController) Name() string {
	return "containers.RuntimeController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RuntimeController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceSpecType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerLifecycleType,
			ID:        optional.Some(containers.ContainerLifecycleID),
			Kind:      controller.InputStrong,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some(criServiceID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *RuntimeController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: containers.ContainerInstanceStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// instanceRunState tracks one running execution.
type instanceRunState struct {
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu         sync.Mutex
	phase      containers.ContainerInstancePhase
	pid        uint32
	exitCode   int32
	err        error
	startedAt  time.Time
	finishedAt time.Time
}

func (s *instanceRunState) snapshot() containers.ContainerInstanceStatusSpec {
	s.mu.Lock()
	defer s.mu.Unlock()

	spec := containers.ContainerInstanceStatusSpec{
		Phase:      s.phase,
		PID:        s.pid,
		ExitCode:   s.exitCode,
		StartedAt:  s.startedAt,
		FinishedAt: s.finishedAt,
	}

	if s.err != nil {
		spec.Error = s.err.Error()
	}

	return spec
}

func (s *instanceRunState) setStarted(pid uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.phase = containers.ContainerInstancePhaseRunning
	s.pid = pid
	s.startedAt = time.Now()
}

func (s *instanceRunState) setFinished(exitCode int32, err error, everStarted bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// A task that never started is a setup failure, which is a different thing from a task that ran
	// and exited: the exit code is meaningless in the first case.
	if everStarted {
		s.phase = containers.ContainerInstancePhaseTerminated
		s.exitCode = exitCode
	} else {
		s.phase = containers.ContainerInstancePhaseFailed
	}

	s.pid = 0
	s.err = err
	s.finishedAt = time.Now()
}

func (s *instanceRunState) stop() {
	s.cancel()
	s.wg.Wait()
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *RuntimeController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.RunnerProvider == nil {
		ctrl.RunnerProvider = func() (TaskRunner, error) {
			return newContainerdRunner(ctrl.Runtime.Logging(), logger)
		}
	}

	ctrl.instances = map[string]*instanceRunState{}
	// Per controller run, not per process: the sweep is what clears a container record left behind by
	// a stop that failed, and a controller restart is one of the ways that happens.
	ctrl.swept = false

	notifyCh := make(chan struct{}, 1)

	var taskRunner TaskRunner

	// Registered before the instance-stopping defer so that it runs after it: the instances use the
	// runner's client, so they have to be joined before it is closed.
	defer func() {
		if taskRunner != nil {
			taskRunner.Close() //nolint:errcheck
		}
	}()

	defer func() {
		for _, instance := range ctrl.instances {
			instance.stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-notifyCh:
		}

		criUp, err := ctrl.criIsUp(ctx, r)
		if err != nil {
			return err
		}

		if criUp && taskRunner == nil {
			if taskRunner, err = ctrl.RunnerProvider(); err != nil {
				logger.Error("failed to create the container task runner", zap.Error(err))

				return fmt.Errorf("failed to create task runner: %w", err)
			}

			logger.Info("connected to the container runtime, containers can now be started")
		}

		if err := ctrl.reconcile(ctx, r, logger, taskRunner, notifyCh); err != nil {
			logger.Error("failed to reconcile container instances", zap.Error(err))

			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *RuntimeController) criIsUp(ctx context.Context, r controller.Runtime) (bool, error) {
	service, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, criServiceID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get %q service: %w", criServiceID, err)
	}

	return service.TypedSpec().Running && service.TypedSpec().Healthy, nil
}

// reconcile brings the running instances in line with the instance specs.
//
// taskRunner is nil while the container runtime is not up yet. The shutdown barrier is still handled
// in that case: the finalizer may have been taken before a controller restart, and this controller is
// the only one that releases it, so the shutdown sequence would otherwise wait out its whole timeout
// with the runtime down.
//
//nolint:gocyclo,cyclop
func (ctrl *RuntimeController) reconcile(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	taskRunner TaskRunner,
	notifyCh chan struct{},
) error {
	lifecycle, err := readContainerLifecycle(ctx, r)
	if err != nil {
		return err
	}

	// The barrier tearing down is the node on its way down, and it is torn down before the phase that
	// stops containerd, which is the only window in which a task can still be stopped gracefully.
	// Everything is wound down here and nothing is started again: returning before the pass below is
	// what makes that stick, since the instance specs outlive the barrier and are still PhaseRunning,
	// so the pass would start them straight back up.
	//
	// An absent barrier means the same thing. It is created by a startup task that completes before
	// the controller runtime starts, so it is not missing at boot; it goes missing only when the
	// shutdown phase destroys it, one pass after releasing it. Treating that as normal operation is
	// what previously restarted every container immediately after stopping it, leaving containers
	// running into the phase that kills containerd, and their records behind to be swept as orphans on
	// the next boot.
	if lifecycle == nil || lifecycle.Metadata().Phase() == resource.PhaseTearingDown {
		ctrl.stopAll(logger)

		return reconcileLifecycle(ctx, r, logger, lifecycle, ctrl.Name(), len(ctrl.instances) == 0)
	}

	if taskRunner == nil {
		logger.Debug("waiting for the container runtime")

		return reconcileLifecycle(ctx, r, logger, lifecycle, ctrl.Name(), len(ctrl.instances) == 0)
	}

	containerInstanceSpecs, err := safe.ReaderListAll[*containers.ContainerInstanceSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list instance specs: %w", err)
	}

	// Sweep before creating anything. Instance resources are in-memory and gone after a machined
	// restart, but the container runtime's are not, so generations restart from zero and a leftover
	// container would collide with a new one of the same ID. Deleting first makes that a non-event,
	// and it only works because one controller owns both halves.
	if !ctrl.swept {
		if err := ctrl.sweepOrphans(ctx, logger, taskRunner, containerInstanceSpecs); err != nil {
			return err
		}

		ctrl.swept = true
	}

	// Ask every instance whose spec is tearing down to stop before waiting for any of them: one stop is
	// bounded by the graceful shutdown timeout plus the wait for the kill, and stopping them one after
	// another would multiply that bound by the number of containers going away at once.
	for containerInstanceSpec := range containerInstanceSpecs.All() {
		if containerInstanceSpec.Metadata().Phase() != resource.PhaseTearingDown {
			continue
		}

		if instance, exists := ctrl.instances[containerInstanceSpec.Metadata().ID()]; exists {
			instance.cancel()
		}
	}

	r.StartTrackingOutputs()

	live := map[string]struct{}{}

	for containerInstanceSpec := range containerInstanceSpecs.All() {
		id := containerInstanceSpec.Metadata().ID()

		switch containerInstanceSpec.Metadata().Phase() {
		case resource.PhaseRunning:
			live[id] = struct{}{}

			if !containerInstanceSpec.Metadata().Finalizers().Has(ctrl.Name()) {
				// The finalizer is the handshake with the instance controller: it will not destroy
				// this instance until the task is stopped and cleaned up.
				if err := r.AddFinalizer(ctx, containerInstanceSpec.Metadata(), ctrl.Name()); err != nil {
					return fmt.Errorf("failed to add finalizer on %q: %w", id, err)
				}
			}

			instance, exists := ctrl.instances[id]
			if !exists {
				// A terminated instance stays PhaseRunning until the controller above decides to
				// replace it, which for a restart is only once the restart interval has elapsed, and
				// ctrl.instances is memory that a controller restart loses. Without this check, an
				// instance whose task has already run and exited would be run a second time by a
				// controller restart landing in that window. The status is this controller's own
				// output, so it outlives the restart and is the record of what already ran.
				finished, err := ctrl.retainFinished(ctx, r, id)
				if err != nil {
					return err
				}

				if finished {
					continue
				}

				instance = ctrl.start(ctx, logger, taskRunner, containerInstanceSpec, notifyCh)
				ctrl.instances[id] = instance
			}

			if err := ctrl.writeStatus(ctx, r, containerInstanceSpec, instance); err != nil {
				return err
			}
		case resource.PhaseTearingDown:
			instance, exists := ctrl.instances[id]
			if exists {
				logger.Info("stopping container instance", zap.String("instance", id))

				// Stopping is synchronous: the task must be gone, and its runtime state cleaned up,
				// before the instance controller is allowed to destroy the resource.
				instance.stop()
				delete(ctrl.instances, id)

				logger.Info("container instance stopped", zap.String("instance", id))
			}

			if err := taskRunner.Remove(ctx, id); err != nil {
				return fmt.Errorf("failed to remove container %q: %w", id, err)
			}

			if containerInstanceSpec.Metadata().Finalizers().Has(ctrl.Name()) {
				if err := r.RemoveFinalizer(ctx, containerInstanceSpec.Metadata(), ctrl.Name()); err != nil {
					return fmt.Errorf("failed to remove finalizer on %q: %w", id, err)
				}

				logger.Debug("released the container instance for destruction", zap.String("instance", id))
			}
		}
	}

	// Any goroutine whose spec vanished outright. Asked to stop first, joined second, for the same
	// reason as the tearing-down pass above.
	for id, instance := range ctrl.instances {
		if _, exists := live[id]; exists {
			continue
		}

		logger.Info("instance spec is gone, stopping the container", zap.String("instance", id))

		instance.cancel()
	}

	for id, instance := range ctrl.instances {
		if _, exists := live[id]; exists {
			continue
		}

		instance.stop()
		delete(ctrl.instances, id)
	}

	if err := safe.CleanupOutputs[*containers.ContainerInstanceStatus](ctx, r); err != nil {
		return fmt.Errorf("failed to clean up outputs: %w", err)
	}

	return reconcileLifecycle(ctx, r, logger, lifecycle, ctrl.Name(), len(ctrl.instances) == 0)
}

// stopAll stops every running instance, synchronously.
//
// Used on the way down: the caller releases the shutdown barrier once this returns, so this has to
// leave nothing running behind it.
//
// Everything is asked to stop before anything is waited for, so the graceful shutdown timeouts run
// concurrently: serially, a handful of containers ignoring SIGTERM would take longer than the
// shutdown sequence gives the barrier as a whole.
func (ctrl *RuntimeController) stopAll(logger *zap.Logger) {
	for containerID, instanceRunState := range ctrl.instances {
		logger.Info("stopping container instance", zap.String("instance", containerID))

		instanceRunState.cancel()
	}

	for containerID, instanceRunState := range ctrl.instances {
		instanceRunState.stop()
		delete(ctrl.instances, containerID)

		logger.Info("container instance stopped", zap.String("instance", containerID))
	}
}

// retainFinished reports whether the instance already ran to completion, keeping its status if so.
//
// The status is read back rather than trusted from memory because ctrl.instances does not survive a
// controller restart, while the status, being an output, does. A caller that gets true must not start
// the instance.
func (ctrl *RuntimeController) retainFinished(ctx context.Context, r controller.Runtime, id string) (bool, error) {
	status, err := safe.ReaderGetByID[*containers.ContainerInstanceStatus](ctx, r, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get instance status %q: %w", id, err)
	}

	if !status.TypedSpec().Phase.Done() {
		// Reported running, but nothing is running it: this is the interrupted case rather than the
		// finished one, so it is started again and the status corrected.
		return false, nil
	}

	previous := *status.TypedSpec()

	// Rewritten unchanged purely to mark it as still wanted: an output left untouched during a pass
	// is reclaimed by CleanupOutputs, and this status is the only record that this generation ran.
	if err := safe.WriterModify(ctx, r,
		containers.NewContainerInstanceStatus(containers.NamespaceName, id),
		func(res *containers.ContainerInstanceStatus) error {
			*res.TypedSpec() = previous

			return nil
		},
	); err != nil {
		return false, fmt.Errorf("failed to retain instance status %q: %w", id, err)
	}

	return true, nil
}

// sweepOrphans removes containers with no corresponding instance spec.
func (ctrl *RuntimeController) sweepOrphans(
	ctx context.Context,
	logger *zap.Logger,
	taskRunner TaskRunner,
	specs safe.List[*containers.ContainerInstanceSpec],
) error {
	existing, err := taskRunner.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	wanted := map[string]struct{}{}

	for spec := range specs.All() {
		wanted[spec.Metadata().ID()] = struct{}{}
	}

	for _, id := range existing {
		if _, exists := wanted[id]; exists {
			continue
		}

		logger.Info("removing orphaned container", zap.String("container", id))

		if err := taskRunner.Remove(ctx, id); err != nil {
			return fmt.Errorf("failed to remove orphaned container %q: %w", id, err)
		}
	}

	return nil
}

// start launches the goroutine that runs one instance to completion.
func (ctrl *RuntimeController) start(
	ctx context.Context,
	logger *zap.Logger,
	taskRunner TaskRunner,
	spec *containers.ContainerInstanceSpec,
	notifyCh chan struct{},
) *instanceRunState {
	id := spec.Metadata().ID()

	instance := &instanceRunState{
		phase: containers.ContainerInstancePhaseCreated,
	}

	// Deliberately derived from the controller context: canceling it is how the task is stopped.
	// The runner is responsible for using a context that outlives the cancellation for its own
	// teardown, or the stop sequence would be canceled before it could run.
	runCtx, cancel := context.WithCancel(ctx)
	instance.cancel = cancel

	instanceSpec := *spec.TypedSpec()

	// notify wakes the controller's main loop without blocking. notifyCh is single-slot and purely
	// a coalescing "something changed, look again" signal: a full buffer already means a reconcile
	// is pending, so a second send is redundant, and it must never block. Blocking here on a
	// canceled or uncancelable context can deadlock the controller's shutdown: if the main loop has
	// already returned via ctx.Done(), nothing will ever drain this channel again.
	notify := func() {
		select {
		case notifyCh <- struct{}{}:
		default:
		}
	}

	instance.wg.Go(func() {
		var everStarted bool

		defer func() {
			if p := recover(); p != nil {
				// One bad container must not take down machined.
				instance.setFinished(0, fmt.Errorf("panic: %v", p), everStarted)

				logger.Error("container run panicked", zap.Stack("stack"), zap.String("instance", id))
			}

			// Wake the controller so the terminal status is published even if nothing else changes.
			notify()
		}()

		logger.Info("starting container",
			zap.String("instance", id),
			zap.String("image", instanceSpec.Image),
		)

		exitCode, err := taskRunner.Run(runCtx, id, instanceSpec, func(pid uint32) {
			everStarted = true

			instance.setStarted(pid)

			logger.Info("container started", zap.String("instance", id), zap.Uint32("pid", pid))

			notify()
		})

		instance.setFinished(exitCode, err, everStarted)

		switch {
		case err != nil:
			logger.Error("container run failed", zap.String("instance", id), zap.Error(err))
		case exitCode != 0:
			logger.Warn("container exited non-zero", zap.String("instance", id), zap.Int32("exitCode", exitCode))
		default:
			logger.Info("container exited", zap.String("instance", id))
		}
	})

	return instance
}

func (ctrl *RuntimeController) writeStatus(
	ctx context.Context,
	r controller.Runtime,
	spec *containers.ContainerInstanceSpec,
	instance *instanceRunState,
) error {
	snapshot := instance.snapshot()

	if err := safe.WriterModify(ctx, r,
		containers.NewContainerInstanceStatus(containers.NamespaceName, spec.Metadata().ID()),
		func(res *containers.ContainerInstanceStatus) error {
			status := res.TypedSpec()

			status.ContainerID = spec.TypedSpec().ContainerID
			status.Generation = spec.TypedSpec().Generation
			status.Phase = snapshot.Phase
			status.PID = snapshot.PID
			status.ExitCode = snapshot.ExitCode
			status.Error = snapshot.Error
			status.StartedAt = snapshot.StartedAt
			status.FinishedAt = snapshot.FinishedAt

			return nil
		},
	); err != nil {
		return fmt.Errorf("failed to write instance status %q: %w", spec.Metadata().ID(), err)
	}

	return nil
}
