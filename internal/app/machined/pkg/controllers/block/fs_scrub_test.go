// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const (
	fsScrubVolumeID  = "vol-var"
	fsScrubMountpath = "/var"
	fsScrubTaskOwner = "block.FSScrubController"
)

type FSScrubSuite struct {
	ctest.DefaultSuite
}

func TestFSScrubSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &FSScrubSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.FSScrubController{}))
			},
		},
	})
}

// createMountStatus creates a MountStatus for the given mountpoint. fs_scrub
// looks at MountStatus.Spec.Target when matching its tasks to mounts.
func (suite *FSScrubSuite) createMountStatus(id, mountpoint string) *block.MountStatus {
	suite.T().Helper()

	ms := block.NewMountStatus(block.NamespaceName, id)
	ms.TypedSpec().Target = mountpoint
	suite.Create(ms)

	return ms
}

// createSchedule creates an FSScrubSchedule that fires shortly. Using a
// near-future StartTime kicks the controller's timer so we exercise the full
// create-task path within the test timeout.
func (suite *FSScrubSuite) createSchedule(id, mountpoint string, period time.Duration, startsIn time.Duration) *block.FSScrubSchedule {
	suite.T().Helper()

	sched := block.NewFSScrubSchedule(id)
	sched.TypedSpec().Mountpoint = mountpoint
	sched.TypedSpec().Period = period
	sched.TypedSpec().StartTime = time.Now().Add(startsIn)
	suite.Create(sched)

	return sched
}

// fakeTaskStatus posts a TaskStatus into state. fs_scrub does not produce
// TaskStatuses itself; in production they come from runtime.TasksController.
// In these unit tests we play that role by hand.
func (suite *FSScrubSuite) fakeTaskStatus(id, owner, result string, taskState runtimeres.TaskState) {
	suite.T().Helper()

	ts := runtimeres.NewTaskStatus(id)
	ts.TypedSpec().ID = id
	ts.TypedSpec().Owner = owner
	ts.TypedSpec().TaskState = taskState
	ts.TypedSpec().Result = result
	ts.TypedSpec().Start = time.Now().Add(-2 * time.Second)
	ts.TypedSpec().Duration = 2 * time.Second
	suite.Create(ts)
}

// TestNoScheduleNoTask verifies that with no schedule the controller emits
// nothing — start-of-day baseline.
func (suite *FSScrubSuite) TestNoScheduleNoTask() {
	ctest.AssertNoResource[*runtimeres.Task](suite, fsScrubMountpath)
	ctest.AssertNoResource[*block.FSScrubStatus](suite, fsScrubVolumeID)
}

// TestScheduledStatusReported verifies the controller publishes an FSScrubStatus
// in the "scheduled" state once a schedule is observed, even before the timer
// fires. This is the user-visible "this scrub is queued" indicator.
func (suite *FSScrubSuite) TestScheduledStatusReported() {
	suite.createMountStatus(fsScrubVolumeID, fsScrubMountpath)
	// A long delay so the timer doesn't fire and turn this into a different test.
	suite.createSchedule(fsScrubVolumeID, fsScrubMountpath, 1*time.Hour, 1*time.Hour)

	ctest.AssertResource(suite, fsScrubVolumeID, func(s *block.FSScrubStatus, asrt *assert.Assertions) {
		asrt.Equal(fsScrubMountpath, s.TypedSpec().Mountpoint)
		asrt.Equal(1*time.Hour, s.TypedSpec().Period)
		asrt.Equal("scheduled", s.TypedSpec().Status)
	})
}

// TestTaskCreatedOnTimer is the headline create-task path. We schedule a scrub
// to fire almost immediately; the controller's internal timer should push the
// mountpoint onto its trigger channel, and a Task resource should pop out the
// other side with the expected args, name and SELinux label.
func (suite *FSScrubSuite) TestTaskCreatedOnTimer() {
	suite.createMountStatus(fsScrubVolumeID, fsScrubMountpath)
	suite.createSchedule(fsScrubVolumeID, fsScrubMountpath, 1*time.Hour, 100*time.Millisecond)

	ctest.AssertResource(suite, fsScrubMountpath, func(t *runtimeres.Task, asrt *assert.Assertions) {
		asrt.Equal(fsScrubMountpath, t.TypedSpec().ID)
		asrt.Equal(fsScrubTaskOwner, t.TypedSpec().Owner)
		// Verify args contain xfs_scrub binary path and the mountpoint —
		// the exact shape of the command line.
		asrt.Contains(t.TypedSpec().Args, "/usr/sbin/xfs_scrub")
		asrt.Contains(t.TypedSpec().Args, "-T")
		asrt.Contains(t.TypedSpec().Args, "-v")
		asrt.Equal(fsScrubMountpath, t.TypedSpec().Args[len(t.TypedSpec().Args)-1])
		asrt.NotEmpty(t.TypedSpec().SelinuxLabel)
	})
}

// TestFinalizerAddedOnTaskCreation verifies that when the controller starts a
// scrub it adds a finalizer to the MountStatus, blocking the volume manager
// from tearing the mount down while the scrub runs.
func (suite *FSScrubSuite) TestFinalizerAddedOnTaskCreation() {
	suite.createMountStatus(fsScrubVolumeID, fsScrubMountpath)
	suite.createSchedule(fsScrubVolumeID, fsScrubMountpath, 1*time.Hour, 100*time.Millisecond)

	ctest.AssertResource(suite, fsScrubVolumeID, func(ms *block.MountStatus, asrt *assert.Assertions) {
		asrt.True(ms.Metadata().Finalizers().Has("block.FSScrubController"),
			"controller should hold a finalizer on MountStatus while scrub runs")
	})
}

// TestForeignTaskStatusIgnored is the regression test for the FIXME the previous
// revision left behind: a TaskStatus produced by some *other* controller must
// not be interpreted as a finished scrub. We mark a mount and schedule for our
// task, then drop in a "foreign" TaskStatus whose ID happens to collide with
// our mountpoint. The controller must:
//  1. Not destroy our (still-running) Task,
//  2. Not remove its finalizer from the MountStatus,
//  3. Not roll the FSScrubStatus into a "completed" state.
func (suite *FSScrubSuite) TestForeignTaskStatusIgnored() {
	suite.createMountStatus(fsScrubVolumeID, fsScrubMountpath)
	suite.createSchedule(fsScrubVolumeID, fsScrubMountpath, 1*time.Hour, 100*time.Millisecond)

	// Wait for our Task and finalizer to be in place — that's the state in
	// which the bug used to bite.
	ctest.AssertResource(suite, fsScrubMountpath, func(*runtimeres.Task, *assert.Assertions) {})
	ctest.AssertResource(suite, fsScrubVolumeID, func(ms *block.MountStatus, asrt *assert.Assertions) {
		asrt.True(ms.Metadata().Finalizers().Has("block.FSScrubController"))
	})

	// Foreign TaskStatus, same ID as our mountpoint, but a different TaskName.
	suite.fakeTaskStatus(fsScrubMountpath, "some_other_controller_task", "Boom", runtimeres.TaskStateCompleted)

	// Give the controller several reconcile passes to (incorrectly) react.
	// AssertResource has a built-in retry; we use a positive assertion that
	// our state is still intact, which would fail if the controller had
	// torn things down.
	ctest.AssertResource(suite, fsScrubMountpath, func(t *runtimeres.Task, asrt *assert.Assertions) {
		asrt.Equal(fsScrubTaskOwner, t.TypedSpec().Owner,
			"our Task must still be present and untouched")
	})
	ctest.AssertResource(suite, fsScrubVolumeID, func(ms *block.MountStatus, asrt *assert.Assertions) {
		asrt.True(ms.Metadata().Finalizers().Has("block.FSScrubController"),
			"finalizer must NOT have been removed because of a foreign TaskStatus")
	})
	ctest.AssertResource(suite, fsScrubVolumeID, func(s *block.FSScrubStatus, asrt *assert.Assertions) {
		asrt.NotEqual("Boom", s.TypedSpec().Status,
			"FSScrubStatus must not have absorbed the foreign task's result")
	})
}

// TestOwnTaskStatusCompletes is the happy path for status reporting: a
// TaskStatus with our TaskName and TaskStateCompleted should drive the
// FSScrubStatus.Status field to the task's Result and trigger Task teardown
// + finalizer release on the MountStatus.
func (suite *FSScrubSuite) TestOwnTaskStatusCompletes() {
	suite.createMountStatus(fsScrubVolumeID, fsScrubMountpath)
	suite.createSchedule(fsScrubVolumeID, fsScrubMountpath, 1*time.Hour, 100*time.Millisecond)

	// Wait until our Task is published — we need to be tracking it before
	// the matching TaskStatus arrives, otherwise processStatuses skips it.
	ctest.AssertResource(suite, fsScrubMountpath, func(*runtimeres.Task, *assert.Assertions) {})

	// Now play the role of the tasks controller and post a completed status.
	suite.fakeTaskStatus(fsScrubMountpath, fsScrubTaskOwner, "Success", runtimeres.TaskStateCompleted)

	// FSScrubStatus should reflect "Success".
	ctest.AssertResource(suite, fsScrubVolumeID, func(s *block.FSScrubStatus, asrt *assert.Assertions) {
		asrt.Equal(fsScrubMountpath, s.TypedSpec().Mountpoint)
		asrt.Equal("Success", s.TypedSpec().Status)
	})

	// Finalizer must be released — the scrub is done, we no longer need to
	// block unmount.
	ctest.AssertResource(suite, fsScrubVolumeID, func(ms *block.MountStatus, asrt *assert.Assertions) {
		asrt.False(ms.Metadata().Finalizers().Has("block.FSScrubController"),
			"finalizer should be released after own-task completion")
	})

	// And the Task itself should be torn down — fs_scrub teardowns/destroys
	// it once it sees a Completed status. NB: there's no peer to release
	// the destroy-ready signal, but fs_scrub is using OutputShared and is
	// the resource owner, so Teardown returns ok-to-destroy immediately.
	ctest.AssertNoResource[*runtimeres.Task](suite, fsScrubMountpath)
}

// TestRunningStatusIgnored verifies that intermediate (non-Completed)
// TaskStatuses from our own task don't trigger teardown. Only Completed
// statuses should advance the state machine.
func (suite *FSScrubSuite) TestRunningStatusIgnored() {
	suite.createMountStatus(fsScrubVolumeID, fsScrubMountpath)
	suite.createSchedule(fsScrubVolumeID, fsScrubMountpath, 1*time.Hour, 100*time.Millisecond)

	ctest.AssertResource(suite, fsScrubMountpath, func(*runtimeres.Task, *assert.Assertions) {})

	suite.fakeTaskStatus(fsScrubMountpath, fsScrubTaskOwner, "", runtimeres.TaskStateRunning)

	// Task must still exist and the finalizer must still be held.
	ctest.AssertResource(suite, fsScrubMountpath, func(t *runtimeres.Task, asrt *assert.Assertions) {
		asrt.Equal(fsScrubTaskOwner, t.TypedSpec().Owner)
	})
	ctest.AssertResource(suite, fsScrubVolumeID, func(ms *block.MountStatus, asrt *assert.Assertions) {
		asrt.True(ms.Metadata().Finalizers().Has("block.FSScrubController"))
	})
}

// TestScheduleRemovedTearsDownTask is the schedule-withdrawal path: when the
// FSScrubSchedule is removed (e.g. user disabled scrubbing for this mount),
// any in-flight Task should be marked for destruction and eventually destroyed.
func (suite *FSScrubSuite) TestScheduleRemovedTearsDownTask() {
	suite.createMountStatus(fsScrubVolumeID, fsScrubMountpath)
	sched := suite.createSchedule(fsScrubVolumeID, fsScrubMountpath, 1*time.Hour, 100*time.Millisecond)

	ctest.AssertResource(suite, fsScrubMountpath, func(*runtimeres.Task, *assert.Assertions) {})

	suite.Destroy(sched)

	ctest.AssertNoResource[*runtimeres.Task](suite, fsScrubMountpath)
	ctest.AssertNoResource[*block.FSScrubStatus](suite, fsScrubVolumeID)
}

// TestMountTeardownReleasesFinalizer is the explicit "withdraw" test the brief
// asks for: when the *input* MountStatus enters the tearing-down phase, the
// controller must remove its finalizer so the mount can finish unmounting,
// and tear down its own outputs (the Task) along with it.
//
// The flow being verified:
//
//	user/volumemgr -> Teardown(MountStatus)        (input, InputStrong)
//	fs_scrub       -> RemoveFinalizer(MountStatus) (so MountStatus can be destroyed)
//	fs_scrub       -> Teardown+Destroy(Task)       (output, withdrawn together)
//	fs_scrub       -> CleanupOutputs(FSScrubStatus)
func (suite *FSScrubSuite) TestMountTeardownReleasesFinalizer() {
	ms := suite.createMountStatus(fsScrubVolumeID, fsScrubMountpath)
	suite.createSchedule(fsScrubVolumeID, fsScrubMountpath, 1*time.Hour, 100*time.Millisecond)

	// Wait until the controller has acquired the finalizer — this is the
	// state in which a withdraw must release it.
	ctest.AssertResource(suite, fsScrubVolumeID, func(ms *block.MountStatus, asrt *assert.Assertions) {
		asrt.True(ms.Metadata().Finalizers().Has("block.FSScrubController"))
	})
	ctest.AssertResource(suite, fsScrubMountpath, func(*runtimeres.Task, *assert.Assertions) {})

	// add finalizer to task as if TasksController started executing it
	suite.AddFinalizer(runtimeres.NewTask(fsScrubMountpath).Metadata(), "runtime.TasksController")

	// Begin the withdraw: mark the MountStatus for teardown. We use the
	// state-level Teardown directly (rather than the controller's Teardown),
	// matching what an upstream owner would do.
	_, err := suite.State().Teardown(suite.Ctx(), ms.Metadata())
	suite.Require().NoError(err)

	// The FSScrubController has requested the task to terminate
	ctest.AssertResource(suite, fsScrubMountpath, func(t *runtimeres.Task, asrt *assert.Assertions) {
		asrt.Equal(t.Metadata().Phase(), resource.PhaseTearingDown)
	})

	// Task finished, remove finalizer from it and report success
	suite.RemoveFinalizer(runtimeres.NewTask(fsScrubMountpath).Metadata(), "runtime.TasksController")
	suite.fakeTaskStatus(fsScrubMountpath, fsScrubTaskOwner, "Success", runtimeres.TaskStateCompleted)

	suite.AssertWithin(5*time.Second, 50*time.Millisecond, func() error {
		got, err := suite.State().Get(suite.Ctx(), ms.Metadata())
		if err != nil {
			return nil //nolint:nilerr
		}

		if got.Metadata().Finalizers().Has("block.FSScrubController") {
			return retry.ExpectedErrorf("finalizer still held on tearing-down mount")
		}

		return nil
	})

	// And the controller's downstream Task must be torn down too — keeping
	// it alive after the input has gone would be a leak.
	ctest.AssertNoResource[*runtimeres.Task](suite, fsScrubMountpath)
}

// TestMultipleMountsIndependent verifies that scrubs for different mounts
// don't interfere — a TaskStatus for one mountpoint must update only that
// mount's FSScrubStatus and finalizer, not its neighbor's. This double-checks
// that the mountpoint-as-ID convention holds.
func (suite *FSScrubSuite) TestMultipleMountsIndependent() {
	suite.createMountStatus(fsScrubVolumeID, fsScrubMountpath)
	suite.createMountStatus("vol-data", "/var/lib/data")

	suite.createSchedule(fsScrubVolumeID, fsScrubMountpath, 1*time.Hour, 100*time.Millisecond)
	suite.createSchedule("vol-data", "/var/lib/data", 24*time.Hour, 100*time.Millisecond)

	ctest.AssertResource(suite, fsScrubMountpath, func(*runtimeres.Task, *assert.Assertions) {})
	ctest.AssertResource(suite, "/var/lib/data", func(*runtimeres.Task, *assert.Assertions) {})

	// Complete only the /var task.
	suite.fakeTaskStatus(fsScrubMountpath, fsScrubTaskOwner, "Success", runtimeres.TaskStateCompleted)

	ctest.AssertNoResource[*runtimeres.Task](suite, fsScrubMountpath)
	// /var/lib/data must remain untouched.
	ctest.AssertResource(suite, "/var/lib/data", func(t *runtimeres.Task, asrt *assert.Assertions) {
		asrt.Equal(fsScrubTaskOwner, t.TypedSpec().Owner)
	})
	ctest.AssertResource(suite, "vol-data", func(ms *block.MountStatus, asrt *assert.Assertions) {
		asrt.True(ms.Metadata().Finalizers().Has("block.FSScrubController"),
			"unrelated mount's finalizer must remain intact")
	})
}

// TestStaleTaskStatusForUntrackedMountIgnored verifies that a TaskStatus
// arriving for a mountpoint we are NOT currently tracking (e.g. left over
// from a previous run, or simply a name collision with our TaskName) does
// not crash the controller and does not produce phantom FSScrubStatuses.
func (suite *FSScrubSuite) TestStaleTaskStatusForUntrackedMountIgnored() {
	// Note: no schedule, no mount status — just a "stale" completed task
	// status with our TaskName for an unknown mountpoint.
	suite.fakeTaskStatus("/nonexistent", fsScrubTaskOwner, "Success", runtimeres.TaskStateCompleted)

	// Controller must not produce an FSScrubStatus for an unknown mountpoint.
	ctest.AssertNoResource[*block.FSScrubStatus](suite, "/nonexistent")
	ctest.AssertNoResource[*runtimeres.Task](suite, "/nonexistent")

	// Sanity: the controller is still alive and reactive — adding a real
	// schedule afterward must still produce a Task.
	suite.createMountStatus(fsScrubVolumeID, fsScrubMountpath)
	suite.createSchedule(fsScrubVolumeID, fsScrubMountpath, 1*time.Hour, 100*time.Millisecond)

	ctest.AssertResource(suite, fsScrubMountpath, func(t *runtimeres.Task, asrt *assert.Assertions) {
		asrt.Equal(fsScrubTaskOwner, t.TypedSpec().Owner)
	})
}

// TestStatusReportedAfterCompletion is the end-to-end status reporting test:
// after a successful task completion, FSScrubStatus must carry the timing
// data from the TaskStatus.
func (suite *FSScrubSuite) TestStatusReportedAfterCompletion() {
	suite.createMountStatus(fsScrubVolumeID, fsScrubMountpath)
	suite.createSchedule(fsScrubVolumeID, fsScrubMountpath, 1*time.Hour, 100*time.Millisecond)

	ctest.AssertResource(suite, fsScrubMountpath, func(*runtimeres.Task, *assert.Assertions) {})

	startedAt := time.Now().Add(-3 * time.Second)
	dur := 1500 * time.Millisecond

	ts := runtimeres.NewTaskStatus(fsScrubMountpath)
	ts.TypedSpec().ID = fsScrubMountpath
	ts.TypedSpec().Owner = fsScrubTaskOwner
	ts.TypedSpec().TaskState = runtimeres.TaskStateCompleted
	ts.TypedSpec().Result = "Success"
	ts.TypedSpec().Start = startedAt
	ts.TypedSpec().Duration = dur
	suite.Create(ts)

	ctest.AssertResource(suite, fsScrubVolumeID, func(s *block.FSScrubStatus, asrt *assert.Assertions) {
		asrt.Equal(fsScrubMountpath, s.TypedSpec().Mountpoint)
		asrt.Equal("Success", s.TypedSpec().Status)
		asrt.Equal(dur, s.TypedSpec().Duration)
		// Time should have been carried through (allow some slop on
		// nanosecond truncation).
		asrt.WithinDuration(startedAt, s.TypedSpec().Time, time.Second)
	})
}
