// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system_test

import (
	"context"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
)

// stuckRunner models a service that does not stop: its Run does not return when
// the context is cancelled, so the service never reaches events.StateFinished
// and never fires system.StateEventDown.
//
// In the field this was a KubeVirt VirtualMachineInstance: the containerd shim
// backing the VMI would not go away, so the 'cri' service could not be stopped.
// The only property of that situation which matters to stopServices is modelled
// here -- a service which is asked to stop and does not.
type stuckRunner struct {
	// release lets the test unblock the runner during cleanup, so the test
	// leaves no goroutine behind whether it passes or fails.
	release chan struct{}
}

func newStuckRunner() *stuckRunner {
	return &stuckRunner{release: make(chan struct{})}
}

func (r *stuckRunner) Open() error { return nil }

func (r *stuckRunner) Close() error { return nil }

func (r *stuckRunner) Run(_ context.Context, eventSink events.Recorder, _ runner.OnStart) (runner.Status, error) {
	eventSink(events.StateRunning, "Running")

	// deliberately not selecting on ctx.Done(): this service ignores the request to stop
	<-r.release

	return runner.Status{Started: true}, nil
}

func (r *stuckRunner) String() string { return "stuckRunner()" }

// TestShutdownBoundedWithStuckService asserts that shutting all services down
// terminates even when one of them never stops.
//
// singleton.Shutdown -> stopServices ends with
//
//	return conditions.WaitForAll(stoppedConds...).Wait(ctx)
//
// on the caller's context. The 30 s deadline declared earlier in the same
// function covers only the wait for reverse dependencies, and the callers of
// this code path (v1alpha1 Controller.runTask, reached from the shutdown and
// reboot sequences) attach no deadline of their own. A single service which
// does not stop therefore blocks the sequence forever: on a physical node the
// remaining phases -- unmounting the filesystems and powering the machine off
// -- are never reached.
func (suite *SystemServicesSuite) TestShutdownBoundedWithStuckService() {
	if testing.Short() {
		suite.T().Skip("takes ~30s: it waits out the shutdown bound")
	}

	// the bound stopServices is expected to honour, plus margin for the test
	const shutdownDeadline = 40 * time.Second

	rt := newRuntime(suite.T())

	// a private instance: this test terminates the instance it uses, and must not
	// disturb the package-level one shared by the other tests in this suite
	system.Services(rt)

	svcs := system.NewServices(rt)

	stuck := newStuckRunner()
	suite.T().Cleanup(func() { close(stuck.release) })

	svcs.LoadAndStart(
		&MockService{name: "stuck-shim", runner: stuck},
	)

	// wait until the service is actually up, so that the shutdown below has
	// something to stop and the test does not race the service runner
	suite.Require().NoError(retry.Constant(30*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
		for _, svcRunner := range svcs.List() {
			if proto := svcRunner.AsProto(); proto.Id == "stuck-shim" {
				if proto.State != events.StateRunning.String() {
					return retry.ExpectedErrorf("service should be running, got %q", proto.State)
				}

				return nil
			}
		}

		return retry.ExpectedErrorf("service not registered yet")
	}))

	returned := make(chan struct{})

	// context.Background(), deliberately: this is what the shutdown and reboot
	// sequences pass in, and the point of the test is that stopServices must
	// bound itself rather than rely on the caller for a deadline
	go func() {
		defer close(returned)

		svcs.Shutdown(context.Background())
	}()

	select {
	case <-returned:
	case <-time.After(shutdownDeadline):
		suite.FailNow("Shutdown did not return", "still stopping services after %s; a service which does not stop blocks the shutdown sequence forever", shutdownDeadline)
	}
}
