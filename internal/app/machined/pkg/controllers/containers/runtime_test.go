// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"context"
	"fmt"
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
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// fakeTaskRunner stands in for containerd. Run blocks until either ctx is canceled (a stop was
// requested) or the test calls exitNow (the container exiting on its own).
type fakeTaskRunner struct {
	mu           sync.Mutex
	list         []string
	started      map[string]uint32
	starts       map[string]int
	finished     map[string]struct{}
	removed      []string
	removeFails  map[string]struct{}
	removeFailed map[string]int
	selfExit     map[string]chan int32
	closed       bool

	// stopsWanted, when non-zero, holds every stop until that many of them are in flight at once.
	stopsWanted int
	stopsSeen   int
	stopsReady  chan struct{}
}

func newFakeTaskRunner() *fakeTaskRunner {
	return &fakeTaskRunner{
		started:      map[string]uint32{},
		starts:       map[string]int{},
		finished:     map[string]struct{}{},
		removeFails:  map[string]struct{}{},
		removeFailed: map[string]int{},
		selfExit:     map[string]chan int32{},
	}
}

// failRemove makes Remove fail for id until clearRemoveFailure is called, standing in for a
// container the runtime will not let go of. Every reconcile that reaches it then returns an error, so
// this is also how a controller restart is provoked.
func (f *fakeTaskRunner) failRemove(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.removeFails[id] = struct{}{}
}

func (f *fakeTaskRunner) clearRemoveFailure(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.removeFails, id)
}

// removeFailureCount reports how many removals of id have been failed, which is how a test tells that
// a reconcile has actually returned an error rather than merely being expected to.
func (f *fakeTaskRunner) removeFailureCount(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.removeFailed[id]
}

// isClosed reports whether the runner's client has been released, which must not happen on the
// controller restart path: the instance goroutines are still using it.
func (f *fakeTaskRunner) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.closed
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

	if _, fails := f.removeFails[id]; fails {
		f.removeFailed[id]++

		return fmt.Errorf("remove of %q is failing on purpose", id)
	}

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

// fakePIDRecorder stands in for the ServicePID resource writer, recording what the controller asked
// for rather than touching COSI. Only containers with security.machinedAccess are recorded at all, so
// the absence of an entry is as meaningful as its contents.
type fakePIDRecorder struct {
	mu       sync.Mutex
	recorded map[string]int32
	cleared  []string
}

func newFakePIDRecorder() *fakePIDRecorder {
	return &fakePIDRecorder{recorded: map[string]int32{}}
}

// record implements pid.Recorder.
func (f *fakePIDRecorder) record(serviceName string, servicePID int32, clearEntry bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if clearEntry {
		f.cleared = append(f.cleared, serviceName)

		delete(f.recorded, serviceName)

		return nil
	}

	f.recorded[serviceName] = servicePID

	return nil
}

// pidOf reports the PID recorded for serviceName, and whether one is recorded at all.
func (f *fakePIDRecorder) pidOf(serviceName string) (int32, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	servicePID, ok := f.recorded[serviceName]

	return servicePID, ok
}

// wasCleared reports whether serviceName was ever cleared, which outlives the entry itself.
func (f *fakePIDRecorder) wasCleared(serviceName string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Contains(f.cleared, serviceName)
}

type RuntimeSuite struct {
	ctest.DefaultSuite

	runner      *fakeTaskRunner
	pidRecorder *fakePIDRecorder
}

func TestRuntimeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &RuntimeSuite{})
}

// SetupTest gives each test its own fake runner: fakeTaskRunner accumulates state (started,
// removed) that must not leak between tests.
func (suite *RuntimeSuite) SetupTest() {
	suite.runner = newFakeTaskRunner()
	suite.pidRecorder = newFakePIDRecorder()

	suite.AfterSetup = func(s *ctest.DefaultSuite) {
		s.Require().NoError(s.Runtime().RegisterController(&containersctrl.RuntimeController{
			RunnerProvider: func() (containersctrl.TaskRunner, error) { return suite.runner, nil },
			PIDRecorder:    suite.pidRecorder.record,
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

func (suite *RuntimeSuite) createInstanceSpecAllowingMachined(id string) {
	spec := containers.NewContainerInstanceSpec(containers.NamespaceName, id)
	spec.TypedSpec().ContainerID = id
	spec.TypedSpec().Image = "sha256:abc123"
	spec.TypedSpec().Security.MachinedAccess = true

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

// TestAllowMachinedAccessRecordsAndClearsServicePID covers the ServicePID half of
// security.machinedAccess: the PID is published under the ctr- prefix while the container runs, and
// taken back once it is gone.
func (suite *RuntimeSuite) TestAllowMachinedAccessRecordsAndClearsServicePID() {
	markCRIUp(suite)
	suite.createLifecycle()

	const id = "nginx-8"

	servicePIDName := constants.ContainerServicePIDPrefix + id

	suite.createInstanceSpecAllowingMachined(id)

	ctest.AssertResource(suite, id, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
	})

	// Recorded from the started callback, so it may land a moment after the status does.
	suite.AssertWithin(3*time.Second, 50*time.Millisecond, func() error {
		servicePID, ok := suite.pidRecorder.pidOf(servicePIDName)
		if !ok {
			return retry.ExpectedErrorf("no ServicePID recorded for %q", servicePIDName)
		}

		suite.Assert().Equal(int32(4242), servicePID)

		return nil
	})

	md := containers.NewContainerInstanceSpec(containers.NamespaceName, id).Metadata()

	_, err := suite.State().Teardown(suite.Ctx(), md)
	suite.Require().NoError(err)

	_, err = suite.State().WatchFor(suite.Ctx(), md, state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	suite.Assert().True(suite.pidRecorder.wasCleared(servicePIDName),
		"ServicePID %q was never cleared", servicePIDName)

	_, stillRecorded := suite.pidRecorder.pidOf(servicePIDName)
	suite.Assert().False(stillRecorded, "ServicePID %q is still recorded after the container stopped", servicePIDName)

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), md))
}

// TestNoServicePIDWithoutAllowMachinedAccess is the other half of the contract: a container that did
// not ask for machined access must not be given an identity machined would recognize.
func (suite *RuntimeSuite) TestNoServicePIDWithoutAllowMachinedAccess() {
	markCRIUp(suite)
	suite.createLifecycle()

	const id = "nginx-9"

	suite.createInstanceSpec(id)

	ctest.AssertResource(suite, id, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
		asrt.NotZero(status.TypedSpec().PID)
	})

	// The container is running and its PID is known, so the recorder has had every chance to be
	// called; nothing being there is the assertion.
	_, recorded := suite.pidRecorder.pidOf(constants.ContainerServicePIDPrefix + id)
	suite.Assert().False(recorded, "a ServicePID was recorded without security.machinedAccess")
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

// provokeControllerRestart takes the controller down and returns once it is back up and idle.
func (suite *RuntimeSuite) provokeControllerRestart(throwawayID string) {
	suite.runner.failRemove(throwawayID)

	suite.createInstanceSpec(throwawayID)

	ctest.AssertResource(suite, throwawayID, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
	})

	md := containers.NewContainerInstanceSpec(containers.NamespaceName, throwawayID).Metadata()

	_, err := suite.State().Teardown(suite.Ctx(), md)
	suite.Require().NoError(err)

	// The failed removal is the error that takes Run down, so having seen one is what makes the restart
	// a fact rather than an assumption.
	suite.AssertWithin(5*time.Second, 50*time.Millisecond, func() error {
		if suite.runner.removeFailureCount(throwawayID) == 0 {
			return retry.ExpectedErrorf("removal has not failed yet")
		}

		return nil
	})

	suite.runner.clearRemoveFailure(throwawayID)

	_, err = suite.State().WatchFor(suite.Ctx(), md, state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), md))

	// The status outliving its spec is reclaimed by CleanupOutputs, which only runs at the end of a
	// pass that failed at nothing. Waiting for it to go is therefore how to know the destroy above has
	// been fully digested and the controller is idle again.
	ctest.AssertNoResource[*containers.ContainerInstanceStatus](suite, throwawayID)
}

// TestControllerRestartLeavesContainersRunning covers a reconcile error taking the controller down.
//
// The controller runtime restarts a controller whose Run returns an error, and a restart is not a
// reason to touch a container: the goroutines waiting on the tasks, the containerd client they wait
// through, and the record of what has been started all have to outlive one Run. Tearing them down
// instead meant any single failed removal restarted every container on the node.
func (suite *RuntimeSuite) TestControllerRestartLeavesContainersRunning() {
	markCRIUp(suite)
	suite.createLifecycle()

	const (
		survivorID = "nginx-10"
		// Sorts after survivorID, so a pass that aborts here has already been past the survivor.
		stuckID = "nginx-11"
	)

	suite.createInstanceSpec(survivorID)
	suite.createInstanceSpec(stuckID)

	for _, id := range []string{survivorID, stuckID} {
		ctest.AssertResource(suite, id, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
			asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
		})
	}

	startsBefore := suite.runner.startCount(survivorID)

	before, err := ctest.Get[*containers.ContainerInstanceStatus](suite,
		containers.NewContainerInstanceStatus(containers.NamespaceName, survivorID).Metadata())
	suite.Require().NoError(err)

	// Removal is the last thing standing between a torn-down instance and its finalizer being
	// released, so failing it is what makes every subsequent pass return an error.
	suite.runner.failRemove(stuckID)

	stuckMD := containers.NewContainerInstanceSpec(containers.NamespaceName, stuckID).Metadata()

	_, err = suite.State().Teardown(suite.Ctx(), stuckMD)
	suite.Require().NoError(err)

	// The stuck instance is stopped: it is the removal that fails, not the stop.
	suite.AssertWithin(3*time.Second, 50*time.Millisecond, func() error {
		if !suite.runner.hasFinished(stuckID) {
			return retry.ExpectedErrorf("stuck instance not yet stopped")
		}

		return nil
	})

	// Several restarts' worth of window. Nothing observable marks a restart, so its consequences are
	// what can be asserted: the survivor was neither stopped nor started again, and the client the
	// goroutine waiting on it is using was not closed underneath it.
	time.Sleep(time.Second)

	suite.Assert().False(suite.runner.hasFinished(survivorID), "surviving container was stopped by a controller restart")
	suite.Assert().Equal(startsBefore, suite.runner.startCount(survivorID), "surviving container was restarted")
	suite.Assert().False(suite.runner.isClosed(), "container runtime client was closed on a controller restart")

	// Still the same execution rather than a replacement that happens to look alike: a restart that
	// stopped and restarted the container would report a fresh PID and start time.
	ctest.AssertResource(suite, survivorID, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
		asrt.Equal(before.TypedSpec().PID, status.TypedSpec().PID)
		asrt.Equal(before.TypedSpec().StartedAt, status.TypedSpec().StartedAt)
	})

	// Once removal works again the instance is released, which is what proves the retry never needed
	// the sweep: the pass itself comes back to it.
	suite.runner.clearRemoveFailure(stuckID)

	_, err = suite.State().WatchFor(suite.Ctx(), stuckMD, state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	suite.Assert().True(suite.runner.wasRemoved(stuckID))
}

// TestSurvivingContainerStillReportsItsExitAfterRestart covers the wake-up path across a restart.
//
// Surviving the restart is only half of outliving the controller: the container also has to still be
// watched afterwards. The goroutine waiting on the task is the one from before the restart, and the
// channel it wakes the loop through is the only thing that can prompt a pass here, the controller
// having been left idle on purpose. A channel belonging to a single Run would leave that goroutine
// sending into one nothing drains any more, and the exit would go unreported until some unrelated
// event happened along.
func (suite *RuntimeSuite) TestSurvivingContainerStillReportsItsExitAfterRestart() {
	markCRIUp(suite)
	suite.createLifecycle()

	const survivorID = "nginx-14"

	suite.createInstanceSpec(survivorID)

	ctest.AssertResource(suite, survivorID, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
	})

	// Sorts after the survivor, so the pass has already been past it by the time it fails.
	suite.provokeControllerRestart("zz-throwaway-0")

	// Untouched by the restart, and still the same execution.
	suite.Require().Equal(1, suite.runner.startCount(survivorID))
	suite.Require().False(suite.runner.hasFinished(survivorID))

	suite.runner.exitNow(survivorID, 42)

	ctest.AssertResource(suite, survivorID, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseTerminated, status.TypedSpec().Phase)
		asrt.Equal(int32(42), status.TypedSpec().ExitCode)
	})

	suite.Assert().Equal(1, suite.runner.startCount(survivorID), "surviving container was started a second time")
}

// TestSurvivingContainerCanStillBeStoppedAfterRestart covers the stop path across a restart.
//
// The stop goes through the cancel function the instance was created with, which hangs off a context
// belonging to the controller rather than to any one Run, and through the map entry the new Run
// inherited. If either had been per-Run, the new Run would find nothing to stop, release the finalizer
// with the task still running, and hand the instance over to be destroyed underneath it.
func (suite *RuntimeSuite) TestSurvivingContainerCanStillBeStoppedAfterRestart() {
	markCRIUp(suite)
	suite.createLifecycle()

	const survivorID = "nginx-15"

	suite.createInstanceSpec(survivorID)

	ctest.AssertResource(suite, survivorID, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
	})

	suite.provokeControllerRestart("zz-throwaway-1")

	suite.Require().Equal(1, suite.runner.startCount(survivorID))
	suite.Require().False(suite.runner.hasFinished(survivorID))

	survivorMD := containers.NewContainerInstanceSpec(containers.NamespaceName, survivorID).Metadata()

	_, err := suite.State().Teardown(suite.Ctx(), survivorMD)
	suite.Require().NoError(err)

	_, err = suite.State().WatchFor(suite.Ctx(), survivorMD, state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	// The finalizer is released only once the task is gone, so a container still running here would
	// mean the handshake had been broken by the restart.
	suite.Assert().True(suite.runner.hasFinished(survivorID))
	suite.Assert().True(suite.runner.wasRemoved(survivorID))
}

// destroyInstanceSpecOutright removes an instance spec without the controller having released it,
// standing in for a spec destroyed out of band.
//
// COSI refuses to destroy a resource that still carries finalizers, and the controller re-adds its own
// on any pass that sees the spec still running, so clearing them and destroying has to be retried as
// one step until it lands.
func (suite *RuntimeSuite) destroyInstanceSpecOutright(id string) {
	md := containers.NewContainerInstanceSpec(containers.NamespaceName, id).Metadata()

	suite.AssertWithin(10*time.Second, 10*time.Millisecond, func() error {
		spec, err := ctest.Get[*containers.ContainerInstanceSpec](suite, md)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		for _, finalizer := range *spec.Metadata().Finalizers() {
			if err := suite.State().RemoveFinalizer(suite.Ctx(), md, finalizer); err != nil {
				return retry.ExpectedError(err)
			}
		}

		if err := suite.State().Destroy(suite.Ctx(), md); err != nil {
			return retry.ExpectedError(err)
		}

		return nil
	})
}

// TestVanishedInstanceRetriesRemoval covers a spec destroyed out of band whose removal fails.
//
// The tearing-down path can afford a failed removal: the finalizer stays on the spec and brings the
// next pass back to it. A vanished spec leaves nothing to hold a finalizer on, so the entry in the
// instance map is the only record that the container is still there. Dropping it on a failure strands
// the container in the container runtime with nothing left that knows about it, and no second orphan
// sweep coming either, since the sweep runs once per process.
func (suite *RuntimeSuite) TestVanishedInstanceRetriesRemoval() {
	markCRIUp(suite)
	suite.createLifecycle()

	const id = "nginx-16"

	suite.createInstanceSpec(id)

	ctest.AssertResource(suite, id, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
	})

	suite.runner.failRemove(id)

	suite.destroyInstanceSpecOutright(id)

	suite.AssertWithin(5*time.Second, 50*time.Millisecond, func() error {
		if suite.runner.removeFailureCount(id) == 0 {
			return retry.ExpectedErrorf("removal has not been attempted yet")
		}

		return nil
	})

	// The task is stopped either way: it is the cleanup after it that fails.
	suite.Assert().True(suite.runner.hasFinished(id))
	suite.Assert().False(suite.runner.wasRemoved(id))

	suite.runner.clearRemoveFailure(id)

	// Nothing refers to this instance any more, so a retry can only come from the controller having
	// held on to it.
	suite.AssertWithin(15*time.Second, 50*time.Millisecond, func() error {
		if !suite.runner.wasRemoved(id) {
			return retry.ExpectedErrorf("removal was not retried")
		}

		return nil
	})
}

// TestFailingInstanceDoesNotBlockOthers covers one instance's failure against the rest of the pass.
//
// Returning on the first per-instance error abandons every instance sorting after it, and with a
// failure that persists those instances are never reached at all: each restart runs a fresh pass that
// dies in the same place. Collecting the errors instead means the pass completes and only the failing
// instance is held back.
func (suite *RuntimeSuite) TestFailingInstanceDoesNotBlockOthers() {
	markCRIUp(suite)
	suite.createLifecycle()

	const (
		stuckID = "nginx-12"
		// Sorts after stuckID: a pass that gives up on the first error never gets here.
		laterID = "nginx-13"
	)

	suite.createInstanceSpec(stuckID)

	ctest.AssertResource(suite, stuckID, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
	})

	suite.runner.failRemove(stuckID)

	stuckMD := containers.NewContainerInstanceSpec(containers.NamespaceName, stuckID).Metadata()

	_, err := suite.State().Teardown(suite.Ctx(), stuckMD)
	suite.Require().NoError(err)

	// Created while the removal is still failing, so reaching it at all requires the pass to have
	// carried on past the failure.
	suite.createInstanceSpec(laterID)

	ctest.AssertResource(suite, laterID, func(status *containers.ContainerInstanceStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstancePhaseRunning, status.TypedSpec().Phase)
	})

	suite.runner.clearRemoveFailure(stuckID)

	_, err = suite.State().WatchFor(suite.Ctx(), stuckMD, state.WithFinalizerEmpty())
	suite.Require().NoError(err)
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
