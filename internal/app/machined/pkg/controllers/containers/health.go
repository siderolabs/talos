// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// HealthController runs the configured health checks against running container instances.
//
// It only observes: the outcome is published as ContainerHealthStatus, which StatusController folds
// into the container's health. A failing check never stops or restarts anything.
type HealthController struct {
	// ProbeProvider is overridable for testing.
	//
	// The returned closer is called once the probe is stopped.
	ProbeProvider func(ProbeTarget) (Probe, io.Closer, error)

	probes   map[string]*probeRun
	notifyCh chan struct{}
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
		{
			Namespace: runtimeres.NamespaceName,
			Type:      runtimeres.ServicePIDType,
			ID:        optional.Some(criServiceID),
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
	ctrl.notifyCh = make(chan struct{}, 1)

	// Probes are cheap to start again, so unlike container tasks they do not outlive a Run.
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
		case <-ctrl.notifyCh:
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

	for id, run := range ctrl.probes {
		if target, ok := desired[id]; ok && target.target == run.target {
			continue
		}

		run.stop()

		delete(ctrl.probes, id)
	}

	for id, target := range desired {
		if _, running := ctrl.probes[id]; running {
			continue
		}

		previous, err := previousOutcome(ctx, runtime, id)
		if err != nil {
			return err
		}

		ctrl.probes[id] = ctrl.start(logger, id, target.target, previous)
	}

	return ctrl.publish(ctx, runtime, desired)
}

// publish writes the outcome of every decided probe, and removes the outcomes of stopped ones.
func (ctrl *HealthController) publish(ctx context.Context, runtime controller.Runtime, desired map[string]desiredProbe) error {
	runtime.StartTrackingOutputs()

	for id, run := range ctrl.probes {
		result, ok := run.result()
		if !ok {
			// No check has decided yet: no resource, which reads as pending.
			continue
		}

		containerID := desired[id].containerID

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

type desiredProbe struct {
	containerID string
	target      ProbeTarget
}

// desiredTargets returns the probes which should be running, keyed by instance ID.
//
// An instance is probed while its task is running and its container has a health check configured.
// The health check comes from the container spec rather than the instance: editing it re-targets
// the probe without replacing the execution.
func (ctrl *HealthController) desiredTargets(ctx context.Context, runtime controller.Runtime) (map[string]desiredProbe, error) {
	instanceStatuses, err := safe.ReaderListAll[*containers.ContainerInstanceStatus](ctx, runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to list instance statuses: %w", err)
	}

	desired := map[string]desiredProbe{}

	runtimePID, err := safe.ReaderGetByID[*runtimeres.ServicePID](ctx, runtime, criServiceID)
	if err != nil {
		if state.IsNotFoundError(err) {
			// The task PIDs cannot be resolved without it; it is back before any task runs again.
			return desired, nil
		}

		return nil, fmt.Errorf("failed to get %s service PID: %w", criServiceID, err)
	}

	for instanceStatus := range instanceStatuses.All() {
		probe, ok, err := ctrl.desiredTarget(ctx, runtime, instanceStatus, uint32(runtimePID.TypedSpec().PID)) //nolint:gosec // PIDs are positive.
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
	ctx context.Context, runtime controller.Runtime, instanceStatus *containers.ContainerInstanceStatus, runtimePID uint32,
) (desiredProbe, bool, error) {
	id := instanceStatus.Metadata().ID()
	status := instanceStatus.TypedSpec()

	if status.Phase != containers.ContainerInstancePhaseRunning || status.PID == 0 {
		return desiredProbe{}, false, nil
	}

	instanceSpec, err := safe.ReaderGetByID[*containers.ContainerInstanceSpec](ctx, runtime, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			return desiredProbe{}, false, nil
		}

		return desiredProbe{}, false, fmt.Errorf("failed to get instance spec %q: %w", id, err)
	}

	if instanceSpec.Metadata().Phase() == resource.PhaseTearingDown {
		return desiredProbe{}, false, nil
	}

	containerSpec, err := safe.ReaderGetByID[*containers.ContainerSpec](ctx, runtime, status.ContainerID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return desiredProbe{}, false, nil
		}

		return desiredProbe{}, false, fmt.Errorf("failed to get container spec %q: %w", status.ContainerID, err)
	}

	healthCheck := containerSpec.TypedSpec().HealthCheck
	if !healthCheck.Enabled() {
		return desiredProbe{}, false, nil
	}

	return desiredProbe{
		containerID: status.ContainerID,
		target: ProbeTarget{
			RuntimePID:  runtimePID,
			PID:         status.PID,
			HostNetwork: instanceSpec.TypedSpec().Network.HostNetwork,
			HealthCheck: healthCheck,
		},
	}, true, nil
}

// previousOutcome returns the outcome already published for an instance, if any.
//
// Instance IDs are unique per execution, so an outcome found here belongs to the same execution: a
// probe replaced because its settings changed, or restarted with the controller, carries on from it
// instead of reporting the container pending again.
func previousOutcome(ctx context.Context, runtime controller.Runtime, id string) (optional.Optional[containers.ContainerHealthStatusSpec], error) {
	res, err := safe.ReaderGetByID[*containers.ContainerHealthStatus](ctx, runtime, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			return optional.None[containers.ContainerHealthStatusSpec](), nil
		}

		return optional.None[containers.ContainerHealthStatusSpec](), fmt.Errorf("failed to get health status %q: %w", id, err)
	}

	return optional.Some(*res.TypedSpec()), nil
}

func (ctrl *HealthController) start(
	logger *zap.Logger, id string, target ProbeTarget, previous optional.Optional[containers.ContainerHealthStatusSpec],
) *probeRun {
	ctx, cancel := context.WithCancel(context.Background())

	run := &probeRun{
		target: target,
		cancel: cancel,
	}

	run.status, run.decided = previous.Get()

	// An execution that already has an outcome is past its start-up: no reason to wait again.
	waitInitialDelay := !run.decided

	// Same coalescing wake-up as RuntimeController's: never blocks, a full slot already means a pass is due.
	notify := func() {
		select {
		case ctrl.notifyCh <- struct{}{}:
		default:
		}
	}

	report := func(healthy bool, message string) {
		if run.report(healthy, message) {
			notify()
		}
	}

	run.wg.Go(func() {
		defer func() {
			if p := recover(); p != nil {
				logger.Error("container health check panicked", zap.Any("panic", p), zap.Stack("stack"), zap.String("instance", id))

				// The probe is not restarted while its target is unchanged: report the failure rather
				// than leaving the container pending for good.
				report(false, fmt.Sprintf("health check panicked: %v", p))
			}
		}()

		probe, closer, err := ctrl.ProbeProvider(target)

		defer func() {
			if closer != nil {
				closer.Close() //nolint:errcheck
			}
		}()

		check := probe

		if err != nil {
			// Building the probe is retried on every check, each failed attempt counting as a failed
			// check: a passing container must not stay degraded over a transient failure to open its
			// network namespace, which would otherwise never be retried while the target is unchanged.
			check = func(ctx context.Context) error {
				if probe == nil {
					if probe, closer, err = ctrl.ProbeProvider(target); err != nil {
						probe, closer = nil, nil

						return err
					}
				}

				return probe(ctx)
			}
		}

		runProbeLoop(ctx, target.HealthCheck, waitInitialDelay, check, report)
	})

	return run
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

// report records an outcome, reporting whether it changed anything.
func (run *probeRun) report(healthy bool, message string) bool {
	run.mu.Lock()
	defer run.mu.Unlock()

	if run.decided && run.status.Healthy == healthy && run.status.Message == message {
		return false
	}

	if !run.decided || run.status.Healthy != healthy {
		run.status.LastChange = time.Now()
	}

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
// report is called with healthy=true after every passing check, and with healthy=false once
// FailureThreshold consecutive checks have failed. A failure below the threshold reports nothing: a
// container that has not passed yet stays undecided, and one that has stays healthy.
//
// Interval is measured from the end of one check to the start of the next, so a check running up to
// its timeout never pushes the next one right behind it.
//
// This is not system/health.Run: services have no failure threshold and no undecided outcome.
func runProbeLoop(
	ctx context.Context, healthCheck containers.ContainerHealthCheckSpec, waitInitialDelay bool, probe Probe, report func(healthy bool, message string),
) {
	// The config layer always fills these in; a zero here would otherwise spin.
	if healthCheck.Interval <= 0 || healthCheck.Timeout <= 0 {
		report(false, fmt.Sprintf("invalid health check schedule: interval %s, timeout %s", healthCheck.Interval, healthCheck.Timeout))

		return
	}

	if waitInitialDelay {
		select {
		case <-ctx.Done():
			return
		case <-time.After(healthCheck.InitialDelay):
		}
	}

	// Rearmed after every check; Reset discards a stale expiry, so the first arming is irrelevant.
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

// applyThreshold reports the outcome of one check, given the consecutive failures before it, and
// returns the consecutive failures after it.
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
