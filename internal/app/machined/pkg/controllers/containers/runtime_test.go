// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	containersctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/containers"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// fakeTaskRunner stands in for containerd. Run blocks until either ctx is canceled (a stop was
// requested) or the test calls exitNow (the container exiting on its own).
type fakeTaskRunner struct {
	mu       sync.Mutex
	list     []string
	started  map[string]uint32
	starts   map[string]int
	finished map[string]struct{}
	removed  []string
	selfExit map[string]chan int32
	closed   bool

	// stopsWanted, when non-zero, holds every stop until that many of them are in flight at once.
	stopsWanted int
	stopsSeen   int
	stopsReady  chan struct{}
}

func newFakeTaskRunner() *fakeTaskRunner {
	return &fakeTaskRunner{
		started:  map[string]uint32{},
		starts:   map[string]int{},
		finished: map[string]struct{}{},
		selfExit: map[string]chan int32{},
	}
}

// presetList seeds the containers List reports before any instance is started, standing in for
// state left behind by a previous machined run.
func (f *fakeTaskRunner) presetList(ids ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.list = append(f.list, ids...)
}

func (f *fakeTaskRunner) List(context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.list...), nil
}

func (f *fakeTaskRunner) Remove(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.removed = append(f.removed, id)
	f.list = slices.DeleteFunc(f.list, func(existing string) bool { return existing == id })

	return nil
}

func (f *fakeTaskRunner) Run(ctx context.Context, id string, _ containers.ContainerInstanceSpecSpec, started func(pid uint32)) (int32, error) {
	const fakePID = 4242

	f.mu.Lock()
	f.started[id] = fakePID
	f.starts[id]++
	exitCh := make(chan int32, 1)
	f.selfExit[id] = exitCh
	f.mu.Unlock()

	started(fakePID)

	defer func() {
		f.mu.Lock()
		defer f.mu.Unlock()

		f.finished[id] = struct{}{}
	}()

	select {
	case <-ctx.Done():
		// Stands in for a graceful stop: the real runner would signal and wait, but for the fake the
		// process is simply gone once asked.
		f.awaitConcurrentStops()

		return 0, nil
	case exitCode := <-exitCh:
		return exitCode, nil
	}
}

func (f *fakeTaskRunner) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.closed = true

	return nil
}

// requireConcurrentStops makes every stop block until n of them are in flight, standing in for n
// containers each sitting out their graceful shutdown timeout.
func (f *fakeTaskRunner) requireConcurrentStops(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.stopsWanted = n
	f.stopsReady = make(chan struct{})
}

// awaitConcurrentStops blocks until as many stops as requireConcurrentStops asked for have arrived.
//
// A caller that stops one container and waits for it before asking the next one never gets here more
// than once, so it blocks forever, which is the point.
func (f *fakeTaskRunner) awaitConcurrentStops() {
	f.mu.Lock()

	if f.stopsWanted == 0 {
		f.mu.Unlock()

		return
	}

	f.stopsSeen++

	ready := f.stopsReady

	if f.stopsSeen == f.stopsWanted {
		close(ready)
	}

	f.mu.Unlock()

	<-ready
}

// exitNow makes a running instance's Run return exitCode on its own, simulating the container
// process exiting without anyone asking it to stop.
func (f *fakeTaskRunner) exitNow(id string, exitCode int32) {
	f.mu.Lock()
	ch := f.selfExit[id]
	f.mu.Unlock()

	if ch != nil {
		ch <- exitCode
	}
}

func (f *fakeTaskRunner) wasStarted(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	_, ok := f.started[id]

	return ok
}

func (f *fakeTaskRunner) wasRemoved(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Contains(f.removed, id)
}

// startCount reports how many times Run has been entered for id, which is how a container being
// started a second time is told apart from one that was only ever started once.
func (f *fakeTaskRunner) startCount(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.starts[id]
}

// hasFinished reports whether Run has returned for id, i.e. the task is no longer running.
func (f *fakeTaskRunner) hasFinished(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	_, ok := f.finished[id]

	return ok
}

type RuntimeSuite struct {
	ctest.DefaultSuite

	runner *fakeTaskRunner
}

func TestRuntimeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &RuntimeSuite{})
}

// SetupTest gives each test its own fake runner: fakeTaskRunner accumulates state (started,
// removed) that must not leak between tests.
func (suite *RuntimeSuite) SetupTest() {
	suite.runner = newFakeTaskRunner()

	suite.AfterSetup = func(s *ctest.DefaultSuite) {
		s.Require().NoError(s.Runtime().RegisterController(&containersctrl.RuntimeController{
			RunnerProvider: func() (containersctrl.TaskRunner, error) { return suite.runner, nil },
		}))
	}

	suite.DefaultSuite.SetupTest()
}

// markCRIUp fakes the cri service being up, which is what gates the controller creating its
// TaskRunner at all.
func markCRIUp(suite ctest.Suite) {
	service := v1alpha1.NewService("cri")
	service.TypedSpec().Running = true
	service.TypedSpec().Healthy = true

	suite.Require().NoError(suite.State().Create(suite.Ctx(), service))
}

func (suite *RuntimeSuite) createLifecycle() {
	suite.Require().NoError(suite.State().Create(suite.Ctx(),
		containers.NewContainerLifecycle(containers.NamespaceName, containers.ContainerLifecycleID)))
}

func (suite *RuntimeSuite) createInstanceSpec(id string) {
	spec := containers.NewContainerInstanceSpec(containers.NamespaceName, id)
	spec.TypedSpec().ContainerID = id
	spec.TypedSpec().Image = "sha256:abc123"

	suite.Require().NoError(suite.State().Create(suite.Ctx(), spec))
}

func (suite *RuntimeSuite) TestStartsInstanceAndReportsRunning() {
	markCRIUp(suite)
	suite.createLifecycle()

	const id = "nginx-0"

	suite.createInstanceSpec(id)

	ctest.AssertResource(suite, id, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
		asrt.NotZero(status.TypedSpec().PID)
	})

	suite.Assert().True(suite.runner.wasStarted(id))

	instance, err := ctest.Get[*containers.ContainerInstanceSpec](suite,
		containers.NewContainerInstanceSpec(containers.NamespaceName, id).Metadata())
	suite.Require().NoError(err)
	suite.Assert().True(instance.Metadata().Finalizers().Has((&containersctrl.RuntimeController{}).Name()))
}

func (suite *RuntimeSuite) TestReportsTerminatedOnSelfExit() {
	markCRIUp(suite)
	suite.createLifecycle()

	const id = "nginx-1"

	suite.createInstanceSpec(id)

	ctest.AssertResource(suite, id, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
	})

	suite.runner.exitNow(id, 137)

	ctest.AssertResource(suite, id, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseTerminated, status.TypedSpec().Phase)
		asrt.Equal(int32(137), status.TypedSpec().ExitCode)
		asrt.Zero(status.TypedSpec().PID)
	})
}

func (suite *RuntimeSuite) TestTeardownStopsRemovesAndReleasesFinalizer() {
	markCRIUp(suite)
	suite.createLifecycle()

	const id = "nginx-2"

	suite.createInstanceSpec(id)

	ctest.AssertResource(suite, id, func(*containers.ContainerInstanceStatus, *assert.Assertions) {})

	md := containers.NewContainerInstanceSpec(containers.NamespaceName, id).Metadata()

	_, err := suite.State().Teardown(suite.Ctx(), md)
	suite.Require().NoError(err)

	_, err = suite.State().WatchFor(suite.Ctx(), md, state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	suite.Assert().True(suite.runner.wasRemoved(id))

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), md))
}

func (suite *RuntimeSuite) TestSweepsOrphanedContainerOnStartup() {
	const orphanID = "leftover-0"

	suite.runner.presetList(orphanID)

	// The barrier is created by a startup task that completes before the controller runtime starts, so
	// it is always there by the time the first pass runs; its absence means the node is on the way
	// down, when sweeping would be pointless.
	suite.createLifecycle()

	markCRIUp(suite)

	suite.AssertWithin(3*time.Second, 50*time.Millisecond, func() error {
		if !suite.runner.wasRemoved(orphanID) {
			return retry.ExpectedErrorf("orphan not yet removed")
		}

		return nil
	})
}

// TestLifecycleTeardownStopsRunningContainers covers the shutdown path as the sequencer actually
// drives it: the barrier is torn down while the instance spec is still PhaseRunning, and nothing else
// asks the container to stop.
//
// Waiting for the instance spec to be torn down first, as the other lifecycle test does, tests the
// easy half. Here the controller has to notice the barrier itself and wind down on its own, otherwise
// the finalizer never clears and the stopContainers phase sits there until its five-minute timeout on
// every single reboot.
func (suite *RuntimeSuite) TestLifecycleTeardownStopsRunningContainers() {
	markCRIUp(suite)
	suite.createLifecycle()

	const id = "nginx-4"

	suite.createInstanceSpec(id)

	ctest.AssertResource(suite, id, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
	})

	lifecycleMD := containers.NewContainerLifecycle(containers.NamespaceName, containers.ContainerLifecycleID).Metadata()

	_, err := suite.State().Teardown(suite.Ctx(), lifecycleMD)
	suite.Require().NoError(err)

	// This is what the stopContainers sequencer phase waits on.
	_, err = suite.State().WatchFor(suite.Ctx(), lifecycleMD, state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	// The barrier exists so that the task is actually stopped before containerd goes away, so
	// releasing it without having stopped anything would defeat the point.
	suite.Assert().True(suite.runner.hasFinished(id))
}

// TestDestroyedLifecycleDoesNotRestartContainers covers the whole shutdown path as the sequencer
// drives it, including the destroy that follows the barrier being released.
//
// The instance spec outlives the barrier and stays PhaseRunning throughout, so treating an absent
// barrier as normal operation restarts the container immediately after stopping it. That container
// then runs on into the phase that kills containerd without ever being stopped gracefully, and its
// record is left behind to be swept as an orphan on the next boot, which is exactly what a graceful
// reboot on a real node produced.
func (suite *RuntimeSuite) TestDestroyedLifecycleDoesNotRestartContainers() {
	markCRIUp(suite)
	suite.createLifecycle()

	const id = "nginx-6"

	suite.createInstanceSpec(id)

	ctest.AssertResource(suite, id, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
	})

	lifecycleMD := containers.NewContainerLifecycle(containers.NamespaceName, containers.ContainerLifecycleID).Metadata()

	_, err := suite.State().Teardown(suite.Ctx(), lifecycleMD)
	suite.Require().NoError(err)

	_, err = suite.State().WatchFor(suite.Ctx(), lifecycleMD, state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	// What the sequencer task does once the finalizers clear.
	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), lifecycleMD))

	suite.Assert().True(suite.runner.hasFinished(id))

	startsBefore := suite.runner.startCount(id)

	// Nothing observable happens on the cease path, by design, so a window is the only way to assert
	// the absence of a restart. Destroying the barrier is an event on a strong input, so the
	// controller does reconcile within it; before the fix that pass restarted the container
	// immediately, well inside this window.
	time.Sleep(500 * time.Millisecond)

	suite.Assert().Equal(startsBefore, suite.runner.startCount(id), "container was restarted after the barrier was destroyed")
}

// TestDoesNotRerunFinishedInstance covers a controller restart finding a generation that already ran.
//
// Terminated generations are deliberately kept in PhaseRunning so their outcome stays inspectable,
// while the in-memory record of what has been started does not survive a restart. The status, being
// an output, does, and is seeded here with the controller as its owner to stand in for that. Without
// it, every retained generation would be run a second time, concurrently, into the same cgroup and
// log.
func (suite *RuntimeSuite) TestDoesNotRerunFinishedInstance() {
	markCRIUp(suite)
	suite.createLifecycle()

	const (
		finishedID = "nginx-5"
		// Sorts after finishedID: the pass walks specs in ID order, so this only means "the pass is
		// done with finishedID" if it comes last.
		clockID = "zz-clock-0"
	)

	status := containers.NewContainerInstanceStatus(containers.NamespaceName, finishedID)
	status.TypedSpec().ContainerID = "nginx"
	status.TypedSpec().Phase = containers.ContainerInstancePhaseTerminated
	status.TypedSpec().ExitCode = 0
	status.TypedSpec().FinishedAt = time.Now()

	suite.Require().NoError(suite.State().Create(suite.Ctx(), status,
		state.WithCreateOwner((&containersctrl.RuntimeController{}).Name())))

	suite.createInstanceSpec(finishedID)
	suite.createInstanceSpec(clockID)

	// A fresh spec does get started, which is what proves a full pass ran over both.
	ctest.AssertResource(suite, clockID, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
	})

	suite.Assert().False(suite.runner.wasStarted(finishedID))

	// The status is what records that it already ran, so it has to survive output cleanup.
	ctest.AssertResource(suite, finishedID, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseTerminated, status.TypedSpec().Phase)
	})
}

// TestHandlesBarrierWithoutContainerRuntime covers a reboot with the CRI down.
//
// The barrier is answered whether or not the container runtime is up. Skipping the whole reconcile
// while it is down strands a finalizer taken before a controller restart, and the stopContainers
// phase then waits out its five-minute timeout and aborts the reboot.
func (suite *RuntimeSuite) TestHandlesBarrierWithoutContainerRuntime() {
	// Deliberately no markCRIUp: there is no task runner for the whole test.
	suite.createLifecycle()

	lifecycleMD := containers.NewContainerLifecycle(containers.NamespaceName, containers.ContainerLifecycleID).Metadata()

	suite.AssertWithin(3*time.Second, 50*time.Millisecond, func() error {
		lifecycle, err := ctest.Get[*containers.ContainerLifecycle](suite, lifecycleMD)
		if err != nil {
			return err
		}

		if !lifecycle.Metadata().Finalizers().Has((&containersctrl.RuntimeController{}).Name()) {
			return retry.ExpectedErrorf("lifecycle finalizer not yet held")
		}

		return nil
	})

	_, err := suite.State().Teardown(suite.Ctx(), lifecycleMD)
	suite.Require().NoError(err)

	// What the stopContainers sequencer phase waits on.
	_, err = suite.State().WatchFor(suite.Ctx(), lifecycleMD, state.WithFinalizerEmpty())
	suite.Require().NoError(err)
}

// TestStopsContainersConcurrentlyOnBarrierTeardown asserts every container is asked to stop before
// any of them is waited for.
//
// The fake runner holds each stop until both have arrived, so a shutdown that stops one container and
// joins it before asking the next never releases the barrier. On a real node each of those waits is
// the graceful shutdown timeout plus the wait for the kill, which serially exceeds the timeout the
// shutdown sequence gives the barrier.
func (suite *RuntimeSuite) TestStopsContainersConcurrentlyOnBarrierTeardown() {
	suite.runner.requireConcurrentStops(2)

	markCRIUp(suite)
	suite.createLifecycle()

	const (
		idA = "nginx-8"
		idB = "nginx-9"
	)

	suite.createInstanceSpec(idA)
	suite.createInstanceSpec(idB)

	for _, id := range []string{idA, idB} {
		ctest.AssertResource(suite, id, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
			asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
		})
	}

	lifecycleMD := containers.NewContainerLifecycle(containers.NamespaceName, containers.ContainerLifecycleID).Metadata()

	_, err := suite.State().Teardown(suite.Ctx(), lifecycleMD)
	suite.Require().NoError(err)

	_, err = suite.State().WatchFor(suite.Ctx(), lifecycleMD, state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	suite.Assert().True(suite.runner.hasFinished(idA))
	suite.Assert().True(suite.runner.hasFinished(idB))
}

func (suite *RuntimeSuite) TestHoldsLifecycleFinalizerWhileRunningReleasesWhenIdle() {
	markCRIUp(suite)
	suite.createLifecycle()

	const id = "nginx-3"

	suite.createInstanceSpec(id)

	ctest.AssertResource(suite, id, func(*containers.ContainerInstanceStatus, *assert.Assertions) {})

	lifecycleMD := containers.NewContainerLifecycle(containers.NamespaceName, containers.ContainerLifecycleID).Metadata()

	suite.AssertWithin(3*time.Second, 50*time.Millisecond, func() error {
		lifecycle, err := ctest.Get[*containers.ContainerLifecycle](suite, lifecycleMD)
		if err != nil {
			return err
		}

		if !lifecycle.Metadata().Finalizers().Has((&containersctrl.RuntimeController{}).Name()) {
			return retry.ExpectedErrorf("lifecycle finalizer not yet held")
		}

		return nil
	})

	instanceMD := containers.NewContainerInstanceSpec(containers.NamespaceName, id).Metadata()

	_, err := suite.State().Teardown(suite.Ctx(), instanceMD)
	suite.Require().NoError(err)

	_, err = suite.State().WatchFor(suite.Ctx(), instanceMD, state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	// Nothing left running: the barrier is releasable now, whatever prompts the next reconcile.
	_, err = suite.State().Teardown(suite.Ctx(), lifecycleMD)
	suite.Require().NoError(err)

	_, err = suite.State().WatchFor(suite.Ctx(), lifecycleMD, state.WithFinalizerEmpty())
	suite.Require().NoError(err)
}
