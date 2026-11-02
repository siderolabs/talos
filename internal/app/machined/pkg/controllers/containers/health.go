// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/panicsafe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// HealthController runs the configured health checks against running container instances.
//
// It only observes: the outcome is published as ContainerHealthStatus, which StatusController folds
// into the container's health. A failing check never stops or restarts anything.
type HealthController struct {
	// ProbeProvider is overridable for testing.
	ProbeProvider func(ProbeTarget) Probe

	probes map[string]*probeRun
}

// Name implements controller.Controller interface.
func (ctrl *HealthController) Name() string {
	return "containers.HealthController"
}

// Inputs implements controller.Controller interface.
func (ctrl *HealthController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerInstanceStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *HealthController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: containers.ContainerHealthStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *HealthController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	if ctrl.ProbeProvider == nil {
		ctrl.ProbeProvider = NewProbe
	}

	ctrl.probes = map[string]*probeRun{}

	// Probes and their outcomes do not outlive a Run: after a restart, a container reads pending
	// until its first check decides.
	defer func() {
		for id, run := range ctrl.probes {
			run.stop()

			delete(ctrl.probes, id)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		}

		if err := ctrl.reconcile(ctx, runtime, logger); err != nil {
			return err
		}

		runtime.ResetRestartBackoff()
	}
}

// reconcile starts and stops probes to match the running instances, and publishes their outcomes.
func (ctrl *HealthController) reconcile(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	desired, err := ctrl.desiredTargets(ctx, runtime)
	if err != nil {
		return err
	}

	// Replaced probes whose outcome the probe replacing them carries on from.
	replaced := map[string]*probeRun{}

	for id, run := range ctrl.probes {
		target, ok := desired[id]
		if ok && target == run.target {
			continue
		}

		run.stop()

		delete(ctrl.probes, id)

		// The outcome stands if only the schedule changed.
		if ok && run.target.HealthCheck.SameCheck(target.HealthCheck) {
			replaced[id] = run
		}
	}

	for id, target := range desired {
		if _, running := ctrl.probes[id]; running {
			continue
		}

		ctrl.probes[id] = ctrl.start(logger, runtime, id, target, replaced[id])
	}

	return ctrl.publish(ctx, runtime)
}

// publish writes the outcome of every decided probe, and removes the outcomes of stopped ones.
func (ctrl *HealthController) publish(ctx context.Context, runtime controller.Runtime) error {
	runtime.StartTrackingOutputs()

	for id, run := range ctrl.probes {
		result, ok := run.result()
		if !ok {
			// No check has decided yet: no resource, which reads as pending.
			continue
		}

		containerID := run.target.ContainerID

		if err := safe.WriterModify(ctx, runtime,
			containers.NewContainerHealthStatus(containers.NamespaceName, id),
			func(res *containers.ContainerHealthStatus) error {
				*res.TypedSpec() = result

				res.Metadata().Labels().Set(containers.ContainerSpecIdLabel, containerID)

				return nil
			},
		); err != nil {
			return fmt.Errorf("failed to write health status %q: %w", id, err)
		}
	}

	if err := safe.CleanupOutputs[*containers.ContainerHealthStatus](ctx, runtime); err != nil {
		return fmt.Errorf("failed to clean up outputs: %w", err)
	}

	return nil
}

// desiredTargets returns the probes which should be running, keyed by instance ID: one per running
// task whose container has a health check, even if its PID is not known, so that the failure is
// reported rather than the container left pending.
func (ctrl *HealthController) desiredTargets(ctx context.Context, reader controller.Reader) (map[string]ProbeTarget, error) {
	instanceStatuses, err := safe.ReaderListAll[*containers.ContainerInstanceStatus](ctx, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to list instance statuses: %w", err)
	}

	desired := map[string]ProbeTarget{}

	for instanceStatus := range instanceStatuses.All() {
		probe, ok, err := ctrl.desiredTarget(ctx, reader, instanceStatus)
		if err != nil {
			return nil, err
		}

		if ok {
			desired[instanceStatus.Metadata().ID()] = probe
		}
	}

	return desired, nil
}

// desiredTarget returns the probe which should be running for an instance, if any.
func (ctrl *HealthController) desiredTarget(
	ctx context.Context, reader controller.Reader, instanceStatus *containers.ContainerInstanceStatus,
) (ProbeTarget, bool, error) {
	id := instanceStatus.Metadata().ID()
	status := instanceStatus.TypedSpec()

	if status.Phase != containers.ContainerInstancePhaseRunning {
		return ProbeTarget{}, false, nil
	}

	instanceSpec, err := safe.ReaderGetByID[*containers.ContainerInstanceSpec](ctx, reader, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			return ProbeTarget{}, false, nil
		}

		return ProbeTarget{}, false, fmt.Errorf("failed to get instance spec %q: %w", id, err)
	}

	if instanceSpec.Metadata().Phase() == resource.PhaseTearingDown {
		return ProbeTarget{}, false, nil
	}

	containerSpec, err := safe.ReaderGetByID[*containers.ContainerSpec](ctx, reader, status.ContainerID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return ProbeTarget{}, false, nil
		}

		return ProbeTarget{}, false, fmt.Errorf("failed to get container spec %q: %w", status.ContainerID, err)
	}

	healthCheck := containerSpec.TypedSpec().HealthCheck
	if !healthCheck.Enabled() {
		return ProbeTarget{}, false, nil
	}

	return ProbeTarget{
		ContainerID: status.ContainerID,
		PID:         status.PID,
		HostNetwork: instanceSpec.TypedSpec().Network.HostNetwork,
		HealthCheck: healthCheck,
	}, true, nil
}

// start starts the probe of an instance, carrying over the outcome of the stopped probe it
// replaces, if any.
func (ctrl *HealthController) start(logger *zap.Logger, runtime controller.Runtime, id string, target ProbeTarget, replaced *probeRun) *probeRun {
	ctx, cancel := context.WithCancel(context.Background())

	run := &probeRun{
		target: target,
		cancel: cancel,
	}

	if replaced != nil {
		run.status, run.decided = replaced.result()
	}

	// A decided execution is past its start-up.
	waitInitialDelay := !run.decided

	report := func(healthy bool, message string) {
		if run.report(healthy, message) {
			runtime.QueueReconcile()
		}
	}

	run.wg.Go(func() {
		if err := panicsafe.Run(func() { ctrl.runProbe(ctx, target, waitInitialDelay, report) }); err != nil {
			logger.Error("container health check panicked", zap.String("instance", id), zap.Error(err))

			// Not restarted while the target is unchanged: report rather than stay pending. The
			// stack after the first line is for the log only.
			message, _, _ := strings.Cut(err.Error(), "\n")

			report(false, message)
		}
	})

	return run
}

// runProbe runs the probe for target on its health check's schedule until ctx is canceled.
func (ctrl *HealthController) runProbe(ctx context.Context, target ProbeTarget, waitInitialDelay bool, report func(healthy bool, message string)) {
	runProbeLoop(ctx, target.HealthCheck, waitInitialDelay, ctrl.ProbeProvider(target), report)
}

// probeRun tracks one running probe.
type probeRun struct {
	target ProbeTarget
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	decided bool
	status  containers.ContainerHealthStatusSpec
}

func (run *probeRun) stop() {
	run.cancel()
	run.wg.Wait()
}

// report records an outcome, returning true on a change between healthy and unhealthy. A new
// failure message alone is not a change: it often varies on every check, e.g. by source port.
func (run *probeRun) report(healthy bool, message string) bool {
	run.mu.Lock()
	defer run.mu.Unlock()

	if run.decided && run.status.Healthy == healthy {
		return false
	}

	run.status.LastChange = time.Now()
	run.decided = true
	run.status.Healthy = healthy
	run.status.Message = message

	return true
}

func (run *probeRun) result() (containers.ContainerHealthStatusSpec, bool) {
	run.mu.Lock()
	defer run.mu.Unlock()

	return run.status, run.decided
}

// runProbeLoop runs probe on the health check's schedule until ctx is canceled.
//
// report gets healthy=true after every passing check, and healthy=false once FailureThreshold
// consecutive checks have failed. Interval runs from the end of one check to the start of the next.
func runProbeLoop(
	ctx context.Context, healthCheck containers.ContainerHealthCheckSpec, waitInitialDelay bool, probe Probe, report func(healthy bool, message string),
) {
	if waitInitialDelay {
		select {
		case <-ctx.Done():
			return
		case <-time.After(healthCheck.InitialDelay):
		}
	}

	// Rearmed after every check.
	timer := time.NewTimer(healthCheck.Interval)
	defer timer.Stop()

	var failures uint32

	for {
		checkCtx, checkCancel := context.WithTimeout(ctx, healthCheck.Timeout)
		err := probe(checkCtx)

		checkCancel()

		if ctx.Err() != nil {
			return
		}

		failures = applyThreshold(err, failures, healthCheck.FailureThreshold, report)

		timer.Reset(healthCheck.Interval)

		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}

// applyThreshold reports the outcome of one check and returns the consecutive failures after it.
func applyThreshold(err error, failures, threshold uint32, report func(healthy bool, message string)) uint32 {
	if err == nil {
		report(true, "")

		return 0
	}

	failures++

	if failures >= threshold {
		report(false, err.Error())
	}

	return failures
}
