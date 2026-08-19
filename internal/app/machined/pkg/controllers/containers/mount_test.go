// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	containersctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/containers"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

const (
	// mountContainer is the container name the mount tests use.
	mountContainer = "nginx"
	// testVolumeID is the block volume ID a userVolume mount resolves to.
	testVolumeID = "u-web-content"
	// testVolumeTarget is the host path the block subsystem mounts the volume at.
	testVolumeTarget = "/var/mnt/web-content"
)

// mountControllerName is the controller's own name, used as both requester and finalizer.
const mountControllerName = "containers.MountController"

type MountSuite struct {
	ctest.DefaultSuite
}

func TestMountSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &MountSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&containersctrl.MountController{}))
			},
		},
	})
}

// SetupTest creates the shutdown barrier, which the controller requires to do anything: its absence
// means the node is going down.
func (suite *MountSuite) SetupTest() {
	suite.DefaultSuite.SetupTest()

	suite.Require().NoError(suite.State().Create(suite.Ctx(),
		containers.NewContainerLifecycle(containers.NamespaceName, containers.ContainerLifecycleID)))
}

// testRequestID mirrors the controller's naming so tests can find what it creates.
var testRequestID = mountControllerName + "/" + mountContainer + "/" + testVolumeID

func (suite *MountSuite) createSpec(mounts ...containers.ContainerMountSpec) {
	spec := containers.NewContainerSpec(containers.NamespaceName, mountContainer)
	spec.TypedSpec().Image = containers.ContainerImageSpec{Ref: "docker.io/library/nginx:latest"}
	spec.TypedSpec().Mounts = mounts

	suite.Require().NoError(suite.State().Create(suite.Ctx(), spec))
}

func (suite *MountSuite) userVolumeMount(options ...string) containers.ContainerMountSpec {
	return containers.ContainerMountSpec{
		Kind:        containers.MountKindUserVolume,
		VolumeID:    testVolumeID,
		Destination: "/usr/share/nginx/html",
		Options:     options,
	}
}

// satisfyMount creates the VolumeMountStatus the block subsystem would produce for the request.
func (suite *MountSuite) satisfyMount(readOnly bool) {
	status := block.NewVolumeMountStatus(block.NamespaceName, testRequestID)
	status.TypedSpec().VolumeID = testVolumeID
	status.TypedSpec().Requester = mountControllerName
	status.TypedSpec().Target = testVolumeTarget
	status.TypedSpec().ReadOnly = readOnly

	suite.Require().NoError(suite.State().Create(suite.Ctx(), status))
}

// createInstance stands in for the instance controller having decided the container should run.
func (suite *MountSuite) createInstance(generation uint64) {
	instance := containers.NewContainerInstanceSpec(containers.NamespaceName, containers.InstanceID(mountContainer, generation))
	instance.TypedSpec().ContainerID = mountContainer
	instance.TypedSpec().Generation = generation

	suite.Require().NoError(suite.State().Create(suite.Ctx(), instance))
}

func (suite *MountSuite) assertMountsReady(ready bool) {
	ctest.AssertResource(suite, mountContainer, func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
		asrt.Equal(ready, status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)
	})
}

// TestPassesThroughTmpfsAndHostPath covers the mount kinds that need nothing from the block
// subsystem, and so are ready without anything else happening.
func (suite *MountSuite) TestPassesThroughTmpfsAndHostPath() {
	suite.createSpec(
		containers.ContainerMountSpec{
			Kind:        containers.MountKindTmpfs,
			Destination: "/tmp",
			Size:        64 << 20,
		},
		containers.ContainerMountSpec{
			Kind:        containers.MountKindHostPath,
			Source:      "/dev",
			Destination: "/dev",
			Options:     []string{"ro"},
		},
	)

	ctest.AssertResource(suite, mountContainer, func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Ready)

		// Guarded: this closure is retried until it holds, so it must not panic on an intermediate
		// state.
		if !asrt.Len(status.TypedSpec().Mounts, 2) {
			return
		}

		asrt.Equal(uint64(64<<20), status.TypedSpec().Mounts[0].Size)
		asrt.Equal("/dev", status.TypedSpec().Mounts[1].Source)
	})

	// Nothing was asked of the block subsystem.
	ctest.AssertNoResource[*block.VolumeMountRequest](suite, testRequestID)
}

// TestRequestsAndResolvesUserVolume covers the whole happy path for a userVolume: the request, the
// wait, the finalizer, and the resolved host path.
func (suite *MountSuite) TestRequestsAndResolvesUserVolume() {
	suite.createSpec(suite.userVolumeMount())

	ctest.AssertResource(suite, testRequestID, func(request *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(testVolumeID, request.TypedSpec().VolumeID)
		asrt.Equal(mountControllerName, request.TypedSpec().Requester)
		// Writable by default.
		asrt.False(request.TypedSpec().ReadOnly)
		// Detached would give a file descriptor with no path to bind into the container.
		asrt.False(request.TypedSpec().Detached)
	})

	// Until the volume is mounted there is no host path, so the container cannot start.
	ctest.AssertResource(suite, mountContainer, func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Ready)
		asrt.Contains(status.TypedSpec().Error, testVolumeID)
	})

	suite.satisfyMount(false)

	ctest.AssertResource(suite, mountContainer, func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Ready)

		if !asrt.Len(status.TypedSpec().Mounts, 1) {
			return
		}

		// The host path is only knowable from the mount status.
		asrt.Equal(testVolumeTarget, status.TypedSpec().Mounts[0].Source)
		asrt.Equal("/usr/share/nginx/html", status.TypedSpec().Mounts[0].Destination)
		asrt.Equal(testVolumeID, status.TypedSpec().Mounts[0].VolumeID)
	})

	// Without the finalizer the volume could be unmounted from under the container.
	ctest.AssertResource(suite, testRequestID, func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Has(mountControllerName))
	})
}

// TestReadOnlyOptionIsRequested covers ro reaching the block subsystem, which is what actually
// enforces it.
func (suite *MountSuite) TestReadOnlyOptionIsRequested() {
	suite.createSpec(suite.userVolumeMount("ro"))

	ctest.AssertResource(suite, testRequestID,
		func(request *block.VolumeMountRequest, asrt *assert.Assertions) {
			asrt.True(request.TypedSpec().ReadOnly)
		})
}

// TestReadOnlyMountIsNotAcceptedForWritable covers the volume coming back with less access than was
// asked for.
//
// Mount requests are merged per volume and end up read-only if every requester asked for read-only,
// so another holder can decide this. Reporting ready would give the container a read-only mount it
// did not ask for.
func (suite *MountSuite) TestReadOnlyMountIsNotAcceptedForWritable() {
	suite.createSpec(suite.userVolumeMount())

	ctest.AssertResource(suite, testRequestID, func(*block.VolumeMountRequest, *assert.Assertions) {})

	suite.satisfyMount(true)

	ctest.AssertResource(suite, mountContainer, func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Ready)
		asrt.Contains(status.TypedSpec().Error, "read-only")
	})
}

// TestTearingDownMountMarksNotReady covers the volume behind a live mount going away.
//
// The mount is reported not-ready while the finalizer stays put: it keeps the volume mounted until the
// container has actually gone.
func (suite *MountSuite) TestTearingDownMountMarksNotReady() {
	suite.createSpec(suite.userVolumeMount())

	ctest.AssertResource(suite, testRequestID, func(*block.VolumeMountRequest, *assert.Assertions) {})
	suite.satisfyMount(false)
	suite.assertMountsReady(true)

	// A live instance is what stops the hold being released while the container still runs.
	suite.createInstance(0)

	statusMD := block.NewVolumeMountStatus(block.NamespaceName, testRequestID).Metadata()

	_, err := suite.State().Teardown(suite.Ctx(), statusMD)
	suite.Require().NoError(err)

	suite.assertMountsReady(false)

	ctest.AssertResource(suite, testRequestID, func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Has(mountControllerName),
			"the hold was released while the container was still running")
	})
}

// TestDoesNotFinalizeTearingDownMount covers a mount status left over from a previous generation.
//
// Adding a finalizer to something already tearing down would block that teardown forever, and the
// replacement mount status can only be created once the old one is gone.
func (suite *MountSuite) TestDoesNotFinalizeTearingDownMount() {
	// Put the leftover in place, tearing down and pinned there by a foreign finalizer, before the
	// container exists. Creating the spec first would let the controller legitimately finalize the
	// status during the window before the teardown, which is what made an earlier version of this
	// test race.
	suite.satisfyMount(false)

	statusMD := block.NewVolumeMountStatus(block.NamespaceName, testRequestID).Metadata()
	suite.AddFinalizer(statusMD, "test")

	_, err := suite.State().Teardown(suite.Ctx(), statusMD)
	suite.Require().NoError(err)

	suite.createSpec(suite.userVolumeMount())

	// The request is made, but the leftover status cannot be used or finalized.
	ctest.AssertResource(suite, testRequestID, func(*block.VolumeMountRequest, *assert.Assertions) {})

	suite.assertMountsReady(false)

	ctest.AssertResource(suite, testRequestID, func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.False(status.Metadata().Finalizers().Has(mountControllerName),
			"a finalizer was added to a mount status which was already tearing down")
	})

	suite.RemoveFinalizer(statusMD, "test")
}

// TestReleasesMountWhenContainerGoesAway covers the mount being given back once the container is gone
// and nothing is running.
func (suite *MountSuite) TestReleasesMountWhenContainerGoesAway() {
	suite.createSpec(suite.userVolumeMount())

	ctest.AssertResource(suite, testRequestID, func(*block.VolumeMountRequest, *assert.Assertions) {})
	suite.satisfyMount(false)
	suite.assertMountsReady(true)

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(),
		containers.NewContainerSpec(containers.NamespaceName, mountContainer).Metadata()))

	// The request is destroyed and the finalizer dropped, so the volume can be unmounted again.
	ctest.AssertNoResource[*block.VolumeMountRequest](suite, testRequestID)

	ctest.AssertResource(suite, testRequestID, func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.False(status.Metadata().Finalizers().Has(mountControllerName))
	})

	ctest.AssertNoResource[*containers.ContainerMountStatus](suite, mountContainer)
}

// TestKeepsMountWhileInstanceIsLive covers a mount removed from the spec while the container is still
// running with it.
//
// The task may still have the path open, so the hold has to outlive the spec change.
func (suite *MountSuite) TestKeepsMountWhileInstanceIsLive() {
	suite.createSpec(suite.userVolumeMount())

	ctest.AssertResource(suite, testRequestID, func(*block.VolumeMountRequest, *assert.Assertions) {})
	suite.satisfyMount(false)
	suite.assertMountsReady(true)

	suite.createInstance(0)

	// Drop the mount from the spec while the instance is still live.
	ctest.UpdateWithConflicts(suite, containers.NewContainerSpec(containers.NamespaceName, mountContainer),
		func(spec *containers.ContainerSpec) error {
			spec.TypedSpec().Mounts = nil

			return nil
		})

	// The spec no longer wants it, but the running task might, so it stays.
	ctest.AssertResource(suite, testRequestID, func(request *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseRunning, request.Metadata().Phase())
	})

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(),
		containers.NewContainerInstanceSpec(containers.NamespaceName, containers.InstanceID(mountContainer, 0)).Metadata()))

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, testRequestID)
}

// TestReleasesEverythingOnLifecycleTeardown covers the shutdown path.
//
// The barrier is what the stopContainers phase waits on, so it has to be released; the container spec
// still exists at that point, and treating it as a reason to keep the mount would hang the shutdown.
func (suite *MountSuite) TestReleasesEverythingOnLifecycleTeardown() {
	suite.createSpec(suite.userVolumeMount())

	ctest.AssertResource(suite, testRequestID, func(*block.VolumeMountRequest, *assert.Assertions) {})
	suite.satisfyMount(false)
	suite.assertMountsReady(true)

	lifecycleMD := containers.NewContainerLifecycle(containers.NamespaceName, containers.ContainerLifecycleID).Metadata()

	_, err := suite.State().Teardown(suite.Ctx(), lifecycleMD)
	suite.Require().NoError(err)

	// This is what the stopContainers sequencer phase blocks on.
	_, err = suite.State().WatchFor(suite.Ctx(), lifecycleMD, state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, testRequestID)
}
