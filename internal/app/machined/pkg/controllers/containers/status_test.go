// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	containersctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/containers"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

const (
	// statusTestContainer is the container name most tests in this suite use.
	statusTestContainer = "nginx"
	// statusTestImageRef is the image the test container spec declares.
	statusTestImageRef = "docker.io/library/nginx:latest"
	// statusTestDigest is the digest a ready image status resolves testImageRef to.
	statusTestDigest = "sha256:abc"
)

type StatusSuite struct {
	ctest.DefaultSuite
}

func TestStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &StatusSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&containersctrl.StatusController{}))
			},
		},
	})
}

// createSpec creates a ContainerSpec for statusTestContainer, applying any mutators.
func (suite *StatusSuite) createSpec(mutate ...func(*containers.ContainerSpecSpec)) {
	suite.createNamedSpec(statusTestContainer, mutate...)
}

func (suite *StatusSuite) createNamedSpec(name string, mutate ...func(*containers.ContainerSpecSpec)) {
	spec := containers.NewContainerSpec(containers.NamespaceName, name)
	spec.TypedSpec().Image = containers.ContainerImageSpec{Ref: statusTestImageRef}

	for _, m := range mutate {
		m(spec.TypedSpec())
	}

	suite.Create(spec)
}

// markImageReady fakes the image controller's output for statusTestContainer.
func (suite *StatusSuite) markImageReady() {
	status := containers.NewContainerImageStatus(containers.NamespaceName, statusTestContainer)
	status.TypedSpec().Phase = containers.ContainerImagePhaseReady
	status.TypedSpec().Image = statusTestImageRef
	status.TypedSpec().Digest = statusTestDigest

	suite.Create(status)
}

// setInstanceStatus fakes the runtime controller's output for a generation of statusTestContainer's
// instance. The resource ID follows the generation the mutator sets, as RuntimeController's own IDs
// do.
func (suite *StatusSuite) setInstanceStatus(mutate func(*containers.ContainerInstanceStatusSpec)) *containers.ContainerInstanceStatus {
	spec := containers.ContainerInstanceStatusSpec{ContainerID: statusTestContainer}

	mutate(&spec)

	status := containers.NewContainerInstanceStatus(containers.NamespaceName, containers.InstanceID(statusTestContainer, spec.Generation))
	*status.TypedSpec() = spec

	suite.Create(status)

	return status
}

func (suite *StatusSuite) assertStatus(check func(*containers.ContainerStatus, *assert.Assertions)) {
	ctest.AssertResource(suite, statusTestContainer, check)
}

// TestPendingWithoutImage covers a fresh container with no image status yet: it has nothing to run,
// so it is waiting, not merely uninitialized.
func (suite *StatusSuite) TestPendingWithoutImage() {
	suite.createSpec()

	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerStatePending, status.TypedSpec().State)
		asrt.Equal(containers.ContainerHealthPending, status.TypedSpec().Health)
		asrt.Contains(status.TypedSpec().WaitingFor, "image")
	})
}

// TestPullingWhileImagePulls covers the image controller reporting an in-progress pull.
func (suite *StatusSuite) TestPullingWhileImagePulls() {
	suite.createSpec()

	status := containers.NewContainerImageStatus(containers.NamespaceName, statusTestContainer)
	status.TypedSpec().Phase = containers.ContainerImagePhasePulling
	status.TypedSpec().Image = statusTestImageRef
	suite.Create(status)

	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerStatePulling, status.TypedSpec().State)
		asrt.Equal(containers.ContainerHealthPulling, status.TypedSpec().Health)
	})
}

// TestStartingOnceImageReady covers every gate satisfied but no instance yet: the instance
// controller is about to create one.
func (suite *StatusSuite) TestStartingOnceImageReady() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerStateStarting, status.TypedSpec().State)
		asrt.Equal(containers.ContainerHealthPulling, status.TypedSpec().Health)
		asrt.Equal(statusTestDigest, status.TypedSpec().Image)
	})
}

// TestRunningReflectsInstance covers a running instance's PID and generation being projected onto
// the aggregate.
func (suite *StatusSuite) TestRunningReflectsInstance() {
	suite.createSpec()
	suite.markImageReady()

	suite.setInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
		spec.Phase = containers.ContainerInstancePhaseRunning
		spec.PID = 1234
	})

	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerStateRunning, status.TypedSpec().State)
		asrt.Equal(containers.ContainerHealthHealthy, status.TypedSpec().Health)
		asrt.Equal(uint32(1234), status.TypedSpec().PID)
	})
}

// TestImageReflectsRunningInstance covers an edited image reference while the previous instance is
// still running: the aggregate names the digest the live PID is actually executing, not the
// reference that has yet to come down.
func (suite *StatusSuite) TestImageReflectsRunningInstance() {
	const (
		newRef    = "docker.io/library/nginx:2"
		oldDigest = "sha256:old"
	)

	suite.createSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Image = containers.ContainerImageSpec{Ref: newRef}
	})

	// The new reference is on its way down, so there is no digest for it yet.
	imageStatus := containers.NewContainerImageStatus(containers.NamespaceName, statusTestContainer)
	imageStatus.TypedSpec().Phase = containers.ContainerImagePhasePulling
	imageStatus.TypedSpec().Image = newRef
	suite.Create(imageStatus)

	instanceSpec := containers.NewContainerInstanceSpec(containers.NamespaceName, containers.InstanceID(statusTestContainer, 0))
	instanceSpec.TypedSpec().ContainerID = statusTestContainer
	instanceSpec.TypedSpec().Image = oldDigest
	suite.Create(instanceSpec)

	suite.setInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
		spec.Phase = containers.ContainerInstancePhaseRunning
		spec.PID = 1234
	})

	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerStateRunning, status.TypedSpec().State)
		asrt.Equal(oldDigest, status.TypedSpec().Image)
	})
}

// TestImageIgnoresStatusForAnotherRef covers an image status left over from the previous reference:
// with no instance to report, the aggregate falls back to the reference itself rather than passing
// off a digest of something else as the current image.
func (suite *StatusSuite) TestImageIgnoresStatusForAnotherRef() {
	const newRef = "docker.io/library/nginx:2"

	suite.createSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Image = containers.ContainerImageSpec{Ref: newRef}
	})

	// Ready, but for the reference the spec no longer declares.
	suite.markImageReady()

	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(newRef, status.TypedSpec().Image)
	})
}

// TestBackoffReflectsTerminatedInstance covers a terminated instance: the container is not running,
// but nothing about it is terminal, so it reports degraded rather than stopped.
func (suite *StatusSuite) TestBackoffReflectsTerminatedInstance() {
	suite.createSpec()
	suite.markImageReady()

	suite.setInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
		spec.Generation = 3
		spec.Phase = containers.ContainerInstancePhaseTerminated
		spec.ExitCode = 137
		spec.Error = "signal: killed"
	})

	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerStateBackoff, status.TypedSpec().State)
		asrt.Equal(containers.ContainerHealthDegraded, status.TypedSpec().Health)
		asrt.Equal(uint32(0), status.TypedSpec().PID)
		asrt.Equal(int32(137), status.TypedSpec().ExitCode)
		asrt.Equal(uint64(3), status.TypedSpec().RestartCount)
		asrt.Equal("signal: killed", status.TypedSpec().Error)
	})
}

// TestExitedRightAfterTermination covers the transient window right after a task exits, before the
// restart interval has elapsed: the container is between attempts, not yet cycling.
func (suite *StatusSuite) TestExitedRightAfterTermination() {
	suite.createSpec()
	suite.markImageReady()

	suite.setInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
		spec.Phase = containers.ContainerInstancePhaseTerminated
		spec.FinishedAt = time.Now()
	})

	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerStateExited, status.TypedSpec().State)
	})
}

// TestBackoffOnceRestartIntervalElapses covers the Exited-to-Backoff flip happening on its own: it
// is driven by the clock rather than by another resource, so nothing else will trigger the pass that
// notices it.
//
// The Exited state this starts in is asserted by TestExitedRightAfterTermination instead, and
// deliberately not here: it only holds for RestartInterval after FinishedAt, so an assertion on it
// would share this test's single 5s window and become unsatisfiable on the first slow scheduling
// stall. Backoff on its own carries the claim regardless — no other write can produce it.
func (suite *StatusSuite) TestBackoffOnceRestartIntervalElapses() {
	suite.createSpec()
	suite.markImageReady()

	suite.setInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
		spec.Phase = containers.ContainerInstancePhaseTerminated
		spec.FinishedAt = time.Now()
	})

	// No further writes: only the controller's own wake-up can move this.
	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerStateBackoff, status.TypedSpec().State)
	})
}

// TestLastExitSurvivesInstanceGap covers the window with no instance status at all — between
// generations, or while a restart waits on a gate: the outcome of the execution that just ended is
// exactly what an operator is looking at there, so it must not read back as a clean slate.
func (suite *StatusSuite) TestLastExitSurvivesInstanceGap() {
	suite.createSpec()
	suite.markImageReady()

	instanceStatus := suite.setInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
		spec.Generation = 3
		spec.Phase = containers.ContainerInstancePhaseTerminated
		spec.ExitCode = 137
		spec.Error = "signal: killed"
	})

	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(uint64(3), status.TypedSpec().RestartCount)
		asrt.Equal(int32(137), status.TypedSpec().ExitCode)
		asrt.Equal("signal: killed", status.TypedSpec().Error)
	})

	suite.Destroy(instanceStatus)

	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerStateStarting, status.TypedSpec().State)
		asrt.Equal(uint64(3), status.TypedSpec().RestartCount)
		asrt.Equal(int32(137), status.TypedSpec().ExitCode)
		asrt.Equal("signal: killed", status.TypedSpec().Error)
	})
}

// TestStoppingWhileInstanceTearsDown covers an instance being torn down: RuntimeController is still
// stopping the task, so the spec's own resource phase is the only signal that says so — the status
// phase does not move to Terminated until the task has actually exited.
func (suite *StatusSuite) TestStoppingWhileInstanceTearsDown() {
	suite.createSpec()
	suite.markImageReady()

	suite.setInstanceStatus(func(spec *containers.ContainerInstanceStatusSpec) {
		spec.Phase = containers.ContainerInstancePhaseRunning
		spec.PID = 1234
	})

	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerStateRunning, status.TypedSpec().State)
		asrt.Equal(containers.ContainerHealthHealthy, status.TypedSpec().Health)
	})

	instanceSpec := containers.NewContainerInstanceSpec(containers.NamespaceName, containers.InstanceID(statusTestContainer, 0))
	suite.Create(instanceSpec)
	suite.AddFinalizer(instanceSpec.Metadata(), "test")

	_, err := suite.State().Teardown(suite.Ctx(), instanceSpec.Metadata())
	suite.Require().NoError(err)

	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerStateStopping, status.TypedSpec().State)
		// Unchanged from before the teardown started, not recomputed from the still-Running phase.
		asrt.Equal(containers.ContainerHealthHealthy, status.TypedSpec().Health)
	})
}

// TestImageErrorSurfaces covers a failed pull being reported through the aggregate when there is no
// instance error to take precedence over it.
func (suite *StatusSuite) TestImageErrorSurfaces() {
	suite.createSpec()

	status := containers.NewContainerImageStatus(containers.NamespaceName, statusTestContainer)
	status.TypedSpec().Phase = containers.ContainerImagePhaseFailed
	status.TypedSpec().Image = statusTestImageRef
	status.TypedSpec().Error = "signature verification denied"
	suite.Create(status)

	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerStateBackoff, status.TypedSpec().State)
		asrt.Equal("signature verification denied", status.TypedSpec().Error)
	})
}

// TestWaitingOnContainerDependency covers dependsOn.containers: a container naming another one is
// pending until that other container reports healthy.
func (suite *StatusSuite) TestWaitingOnContainerDependency() {
	suite.createSpec(func(spec *containers.ContainerSpecSpec) {
		spec.DependsOn.Containers = []string{"other"}
	})
	suite.markImageReady()

	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerStatePending, status.TypedSpec().State)
		asrt.Contains(status.TypedSpec().WaitingFor, "container: other")
	})

	other := containers.NewContainerStatus(containers.NamespaceName, "other")
	other.TypedSpec().Health = containers.ContainerHealthHealthy
	suite.Create(other)

	suite.assertStatus(func(status *containers.ContainerStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerStateStarting, status.TypedSpec().State)
	})
}

// TestRemovedOnSpecRemoval covers a removed ContainerConfig: the aggregate is destroyed rather than
// parked in a final value, matching ContainerStatus's "in-memory only" contract.
func (suite *StatusSuite) TestRemovedOnSpecRemoval() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertStatus(func(*containers.ContainerStatus, *assert.Assertions) {})

	suite.Destroy(containers.NewContainerSpec(containers.NamespaceName, statusTestContainer))

	ctest.AssertNoResource[*containers.ContainerStatus](suite, statusTestContainer)
}
