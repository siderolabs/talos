// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/pid"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type minimalRuntime struct {
	logging v1alpha1runtime.LoggingManager
}

func (m *minimalRuntime) Logging() v1alpha1runtime.LoggingManager { return m.logging }
func (m *minimalRuntime) Config() config.Config                   { return nil }

func (m *minimalRuntime) ConfigContainer() config.Container { panic("not implemented") }
func (m *minimalRuntime) ConfigCompleteForBoot() bool       { panic("not implemented") }
func (m *minimalRuntime) RollbackToConfigAfter(time.Duration) error {
	panic("not implemented")
}
func (m *minimalRuntime) CancelConfigRollbackTimeout()             { panic("not implemented") }
func (m *minimalRuntime) SetConfig(config.Provider) error          { panic("not implemented") }
func (m *minimalRuntime) SetPersistedConfig(config.Provider) error { panic("not implemented") }
func (m *minimalRuntime) CanApplyImmediate(config.Provider) error  { panic("not implemented") }
func (m *minimalRuntime) State() v1alpha1runtime.State             { panic("not implemented") }
func (m *minimalRuntime) Events() v1alpha1runtime.EventStream      { panic("not implemented") }
func (m *minimalRuntime) NodeName() (string, error)                { panic("not implemented") }
func (m *minimalRuntime) IsBootstrapAllowed() bool                 { panic("not implemented") }
func (m *minimalRuntime) GetSystemInformation(_ context.Context) (*hardware.SystemInformation, error) {
	panic("not implemented")
}

// Fake runner.
type fakeRunner struct {
	id   string
	args []string

	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once

	// runErr is returned from Run on natural completion (after finish()).
	runErr error

	// runCalled / stopCalled let tests assert on the runner lifecycle.
	runCalled  atomic.Bool
	stopCalled atomic.Bool
}

func (f *fakeRunner) String() string { return f.id }
func (f *fakeRunner) Open() error    { return nil }
func (f *fakeRunner) Close() error   { return nil }
func (f *fakeRunner) Run(_ events.Recorder, _ pid.Recorder) error {
	f.runCalled.Store(true)

	select {
	case <-f.stopCh:
		return errors.New("stopped")
	case <-f.doneCh:
		return f.runErr
	}
}

func (f *fakeRunner) Stop() error {
	f.stopCalled.Store(true)
	f.stopOnce.Do(func() { close(f.stopCh) })

	return nil
}

// finish lets the test simulate the underlying process completion.
func (f *fakeRunner) finish() {
	close(f.doneCh)
}

// runnerRegistry tracks every runner the controller asks for, indexed by
// the Task ID. This lets tests reach in and drive the underlying "process"
// without races on a private map.
type runnerRegistry struct {
	mu      sync.Mutex
	runners map[string]*fakeRunner
	errs    map[string]error
}

func newRunnerRegistry() *runnerRegistry {
	return &runnerRegistry{
		runners: make(map[string]*fakeRunner),
		errs:    make(map[string]error),
	}
}

// setError pre-stages the error a future runner will return on natural
// completion.
func (rr *runnerRegistry) setError(id string, err error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	rr.errs[id] = err
}

func (rr *runnerRegistry) factory() func(v1alpha1runtime.Runtime, *runner.Args, string) runner.Runner {
	return func(_ v1alpha1runtime.Runtime, args *runner.Args, _ string) runner.Runner {
		rr.mu.Lock()
		defer rr.mu.Unlock()

		// Use ProcessArgs[0] as the unique ID — the factory doesn't see
		// the task ID directly. Falls back to the runner.Args.ID if
		// ProcessArgs is empty.
		id := args.ID
		if len(args.ProcessArgs) > 0 {
			id = args.ProcessArgs[0]
		}

		fr := &fakeRunner{
			id:     id,
			args:   args.ProcessArgs,
			stopCh: make(chan struct{}),
			doneCh: make(chan struct{}),
			runErr: rr.errs[id],
		}
		rr.runners[id] = fr

		return fr
	}
}

// get fetches a runner by the marker we used as ProcessArgs[0]. It retries
// briefly because the controller creates the runner asynchronously after a
// Task resource is observed.
//
//nolint:unparam
func (rr *runnerRegistry) get(t *testing.T, id string, timeout time.Duration) *fakeRunner {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rr.mu.Lock()
		fr, ok := rr.runners[id]
		rr.mu.Unlock()

		if ok {
			return fr
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("runner %q never created", id)

	return nil
}

// =============================================================================
// In-memory tests using the fake runner.
// =============================================================================

type TasksFakeRunnerSuite struct {
	ctest.DefaultSuite

	registry *runnerRegistry
	logDir   string
}

func TestTasksFakeRunnerSuite(t *testing.T) {
	registry := newRunnerRegistry()
	logDir := t.TempDir()

	suite.Run(t, &TasksFakeRunnerSuite{
		registry: registry,
		logDir:   logDir,
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrl.TasksController{
					Runtime: &minimalRuntime{
						logging: logging.NewFileLoggingManager(logDir),
					},
					NewRunner: registry.factory(),
				}))
			},
		},
	})
}

// createTask is a convenience helper that creates a Task resource. The first
// element of args is used as the unique runner key in the registry, so each
// test should pass a distinct value (e.g. the task ID).
func (suite *TasksFakeRunnerSuite) createTask(id, owner string, args ...string) {
	suite.T().Helper()

	t := runtimeres.NewTask(id)
	t.TypedSpec().ID = id
	t.TypedSpec().Owner = owner
	t.TypedSpec().Args = args
	t.TypedSpec().SelinuxLabel = "system_u:system_r:init_t:s0"
	suite.Create(t)
}

// TestRunsTaskAndPropagatesOwner verifies the basic happy path: a created
// Task is observed, the runner factory is called, the controller publishes
// a Running TaskStatus that includes the propagated Owner, and on natural
// completion the status flips to Completed with Result="Success".
func (suite *TasksFakeRunnerSuite) TestRunsTaskAndPropagatesOwner() {
	const id = "task-ok"
	suite.createTask(id, "fs_scrub", id, "arg1", "arg2")

	// While running we should see TaskState==Running and Owner carried.
	ctest.AssertResource(suite, id, func(s *runtimeres.TaskStatus, asrt *assert.Assertions) {
		asrt.Equal(id, s.TypedSpec().ID)
		asrt.Equal("fs_scrub", s.TypedSpec().Owner)
		asrt.Equal(runtimeres.TaskStateRunning, s.TypedSpec().TaskState)
	})

	// Drive the runner to natural completion.
	fr := suite.registry.get(suite.T(), id, 5*time.Second)
	fr.finish()

	// After completion the status must show Completed + Success.
	ctest.AssertResource(suite, id, func(s *runtimeres.TaskStatus, asrt *assert.Assertions) {
		asrt.Equal(runtimeres.TaskStateCompleted, s.TypedSpec().TaskState)
		asrt.Equal("Success", s.TypedSpec().Result)
		asrt.Equal("fs_scrub", s.TypedSpec().Owner,
			"Owner must remain populated through completion")
	})
}

// TestRunnerErrorRecordedAsResult verifies that when the runner returns a
// non-nil error from Run(), the controller writes that error's message into
// the TaskStatus.Result field rather than "Success".
func (suite *TasksFakeRunnerSuite) TestRunnerErrorRecordedAsResult() {
	const id = "task-err"
	suite.registry.setError(id, errors.New("scrub found corruption"))
	suite.createTask(id, "fs_scrub", id)

	fr := suite.registry.get(suite.T(), id, 5*time.Second)
	fr.finish()

	ctest.AssertResource(suite, id, func(s *runtimeres.TaskStatus, asrt *assert.Assertions) {
		asrt.Equal(runtimeres.TaskStateCompleted, s.TypedSpec().TaskState)
		asrt.Equal("scrub found corruption", s.TypedSpec().Result)
	})
}

// TestFinalizerLifecycle verifies the controller adds its finalizer when a
// task starts running, and removes it once the task completes — without
// the finalizer dance, an Owner that destroys the Task during a run would
// race with the runner.
func (suite *TasksFakeRunnerSuite) TestFinalizerLifecycle() {
	const id = "task-finalizer"
	suite.createTask(id, "fs_scrub", id)

	// While running the controller's finalizer must be present.
	ctest.AssertResource(suite, id, func(t *runtimeres.Task, asrt *assert.Assertions) {
		asrt.True(t.Metadata().Finalizers().Has("runtime.TasksController"))
	})

	// Complete the task.
	fr := suite.registry.get(suite.T(), id, 5*time.Second)
	fr.finish()

	// Finalizer must be released after completion.
	ctest.AssertResource(suite, id, func(t *runtimeres.Task, asrt *assert.Assertions) {
		asrt.False(t.Metadata().Finalizers().Has("runtime.TasksController"),
			"finalizer should be released after task completion")
	})
}

// TestTaskWithdrawCancelsRunner is the explicit "withdraw" test the brief asks
// for from the tasks-controller side. The flow:
//
//	owner       -> Teardown(Task)
//	tasks ctrl  -> stop the runner (cancellation)
//	tasks ctrl  -> RemoveFinalizer(Task)
//	owner       -> Destroy(Task)  (now possible because finalizer is gone)
//	tasks ctrl  -> CleanupOutputs(TaskStatus)
//
// We drive this with the fake runner and assert that Stop() was called on
// the runner and that the TaskStatus is eventually cleaned up.
func (suite *TasksFakeRunnerSuite) TestTaskWithdrawCancelsRunner() {
	const id = "task-withdraw"
	suite.createTask(id, "fs_scrub", id)

	// Wait for the runner to actually be started before tearing down — we
	// want to test the cancellation path, not the unstarted-task path.
	fr := suite.registry.get(suite.T(), id, 5*time.Second)
	suite.Require().Eventually(func() bool { return fr.runCalled.Load() }, 5*time.Second, 10*time.Millisecond,
		"runner.Run() should have been called")

	ctest.AssertResource(suite, id, func(t *runtimeres.Task, asrt *assert.Assertions) {
		asrt.True(t.Metadata().Finalizers().Has("runtime.TasksController"))
	})

	// Begin the withdraw via state.Teardown.
	taskMeta := runtimeres.NewTask(id).Metadata()
	_, err := suite.State().Teardown(suite.Ctx(), taskMeta)
	suite.Require().NoError(err)

	// Stop() must be called on the runner — this is the controller asking
	// the underlying process to terminate.
	suite.Require().Eventually(func() bool { return fr.stopCalled.Load() }, 5*time.Second, 10*time.Millisecond,
		"runner.Stop() should have been called on Task teardown")

	// And the finalizer must be removed so the destroy can proceed. We
	// poll the state directly because the finalizer-release-then-destroy
	// dance can be over by the time we look.
	suite.AssertWithin(5*time.Second, 50*time.Millisecond, func() error {
		got, err := suite.State().Get(suite.Ctx(), taskMeta)
		if err != nil {
			// Already destroyed — that's the success case.
			return nil //nolint:nilerr
		}

		if got.Metadata().Finalizers().Has("runtime.TasksController") {
			return retry.ExpectedErrorf("finalizer still held during withdraw")
		}

		return nil
	})

	// And the TaskStatus output should be cleaned up to match.
	ctest.AssertNoResource[*runtimeres.TaskStatus](suite, id)
}

// TestMultipleTasksConcurrent verifies that two Tasks created together both
// reach the Running state and both are independently completable. The
// controller starts at most one task per reconcile pass (note the `break`
// in Run), but a new pass picks up the next Created task immediately, so
// from the test's perspective both tasks observably run.
// ^ - should it be the case, or we should execute in sequence only?
func (suite *TasksFakeRunnerSuite) TestMultipleTasksConcurrent() {
	const idA, ownerA, idB, ownerB = "task-a", "fs_scrub", "task-b", "fstrim"

	suite.createTask(idA, ownerA, idA)

	suite.createTask(idB, ownerB, idB)

	// Both runners must be created and running.
	frA := suite.registry.get(suite.T(), idA, 5*time.Second)
	frB := suite.registry.get(suite.T(), idB, 5*time.Second)

	suite.Require().Eventually(func() bool { return frA.runCalled.Load() && frB.runCalled.Load() },
		5*time.Second, 10*time.Millisecond,
		"both runners should be started")

	// Both TaskStatuses should report Running.
	ctest.AssertResource(suite, idA, func(s *runtimeres.TaskStatus, asrt *assert.Assertions) {
		asrt.Equal(runtimeres.TaskStateRunning, s.TypedSpec().TaskState)
		asrt.Equal(ownerA, s.TypedSpec().Owner)
	})
	ctest.AssertResource(suite, idB, func(s *runtimeres.TaskStatus, asrt *assert.Assertions) {
		asrt.Equal(runtimeres.TaskStateRunning, s.TypedSpec().TaskState)
		asrt.Equal(ownerB, s.TypedSpec().Owner)
	})

	// Complete A only — B should remain running.
	frA.finish()

	ctest.AssertResource(suite, idA, func(s *runtimeres.TaskStatus, asrt *assert.Assertions) {
		asrt.Equal(runtimeres.TaskStateCompleted, s.TypedSpec().TaskState)
	})
	ctest.AssertResource(suite, idB, func(s *runtimeres.TaskStatus, asrt *assert.Assertions) {
		asrt.Equal(runtimeres.TaskStateRunning, s.TypedSpec().TaskState,
			"B must remain running while A completes independently")
	})

	// Now finish B and verify it lands.
	frB.finish()
	ctest.AssertResource(suite, idB, func(s *runtimeres.TaskStatus, asrt *assert.Assertions) {
		asrt.Equal(runtimeres.TaskStateCompleted, s.TypedSpec().TaskState)
	})
}
