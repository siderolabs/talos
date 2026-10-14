// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	containersctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/containers"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

const (
	// mountContainer is the container name the mount tests use.
	mountContainer = "nginx"
	// testVolumeName is the user volume a volume mount names.
	testVolumeName = "web-content"
	// testVolumeID is the block volume ID the user volume is known by.
	testVolumeID = "u-" + testVolumeName
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
		Timeout: 15 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&containersctrl.MountController{}))
		},
	})
}

// SetupTest creates the shutdown barrier, which the controller requires to do anything: its absence
// means the node is going down. It also declares the test volume as a user volume, which is what the
// controller resolves volume names against.
func (suite *MountSuite) SetupTest() {
	suite.DefaultSuite.SetupTest()

	suite.Require().NoError(suite.State().Create(suite.Ctx(),
		containers.NewContainerLifecycle(containers.NamespaceName, containers.ContainerLifecycleID)))

	declareTestVolume(&suite.DefaultSuite)
}

// declareTestVolume creates a machine config declaring the test volume as a user volume.
func declareTestVolume(suite *ctest.DefaultSuite) {
	userVolume := blockcfg.NewUserVolumeConfigV1Alpha1()
	userVolume.MetaName = testVolumeName

	cfg, err := container.New(userVolume)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), config.NewMachineConfig(cfg)))
}

// replaceVolumes replaces the volume documents in the active machine config.
func (suite *MountSuite) replaceVolumes(documents ...configcfg.Document) {
	current, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.ActiveID)
	suite.Require().NoError(err)

	cfg, err := container.New(documents...)
	suite.Require().NoError(err)

	updated := config.NewMachineConfig(cfg)
	updated.Metadata().SetVersion(current.Metadata().Version())

	suite.Require().NoError(suite.State().Update(suite.Ctx(), updated))
}

// testRequestID mirrors the controller's naming so tests can find what it creates.
var testRequestID = mountControllerName + "/" + mountContainer + "/" + testVolumeID

func (suite *MountSuite) createSpec(mounts ...containers.ContainerMountSpec) {
	spec := containers.NewContainerSpec(containers.NamespaceName, mountContainer)
	spec.TypedSpec().Image = containers.ContainerImageSpec{Ref: "docker.io/library/nginx:latest"}
	spec.TypedSpec().Mounts = mounts

	suite.Require().NoError(suite.State().Create(suite.Ctx(), spec))
}

func (suite *MountSuite) volumeMount(options ...string) containers.ContainerMountSpec {
	return containers.ContainerMountSpec{
		Kind:        containers.MountKindVolume,
		VolumeName:  testVolumeName,
		Destination: "/usr/share/nginx/html",
		Options:     options,
	}
}

// satisfyMount creates the VolumeMountStatus the block subsystem would produce for the request.
func (suite *MountSuite) satisfyMount(readOnly bool) {
	suite.satisfyMountFor(testRequestID, testVolumeID, readOnly)
}

// satisfyMountFor is satisfyMount for a request on any volume.
func (suite *MountSuite) satisfyMountFor(requestID, volumeID string, readOnly bool) {
	status := block.NewVolumeMountStatus(block.NamespaceName, requestID)
	status.TypedSpec().VolumeID = volumeID
	status.TypedSpec().Requester = mountControllerName
	status.TypedSpec().Target = testVolumeTarget
	status.TypedSpec().ReadOnly = readOnly

	suite.Require().NoError(suite.State().Create(suite.Ctx(), status))
}

// testInstanceID is the instance the mount tests stand in for: the first generation of mountContainer.
var testInstanceID = containers.InstanceID(mountContainer, 0)

// createInstance stands in for the instance controller having decided the container should run.
func (suite *MountSuite) createInstance() {
	instance := containers.NewContainerInstanceSpec(containers.NamespaceName, testInstanceID)
	instance.TypedSpec().ContainerID = mountContainer

	suite.Require().NoError(suite.State().Create(suite.Ctx(), instance))
}

// finishInstance stands in for the runtime controller reporting that the instance's task has exited.
func (suite *MountSuite) finishInstance() {
	status := containers.NewContainerInstanceStatus(containers.NamespaceName, testInstanceID)
	status.TypedSpec().ContainerID = mountContainer
	status.TypedSpec().Phase = containers.ContainerInstancePhaseTerminated
	status.TypedSpec().FinishedAt = time.Now()

	suite.Require().NoError(suite.State().Create(suite.Ctx(), status))
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

// TestRequestsAndResolvesUserVolume covers the whole happy path for a user volume: the request, the
// wait, the finalizer, and the resolved host path.
func (suite *MountSuite) TestRequestsAndResolvesUserVolume() {
	suite.createSpec(suite.volumeMount())

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
		// The reason names the volume, not the internal block volume ID.
		asrt.Equal(`waiting for volume "web-content" to be mounted`, status.TypedSpec().Error)
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
		asrt.Equal(testVolumeName, status.TypedSpec().Mounts[0].VolumeName)
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
	suite.createSpec(suite.volumeMount("ro"))

	ctest.AssertResource(suite, testRequestID,
		func(request *block.VolumeMountRequest, asrt *assert.Assertions) {
			asrt.True(request.TypedSpec().ReadOnly)
		})
}

// TestResolvesExistingVolume covers a name declared by an ExistingVolumeConfig.
func (suite *MountSuite) TestResolvesExistingVolume() {
	doc := blockcfg.NewExistingVolumeConfigV1Alpha1()
	doc.MetaName = testVolumeName

	suite.assertResolvesTo(doc, "e-"+testVolumeName)
}

// TestResolvesExternalVolume covers a name declared by an ExternalVolumeConfig.
func (suite *MountSuite) TestResolvesExternalVolume() {
	doc := blockcfg.NewExternalVolumeConfigV1Alpha1()
	doc.MetaName = testVolumeName

	suite.assertResolvesTo(doc, "x-"+testVolumeName)
}

// assertResolvesTo declares the test volume with document, and checks the volume mount resolves to
// volumeID all the way to a ready mount status.
func (suite *MountSuite) assertResolvesTo(document configcfg.Document, volumeID string) {
	suite.replaceVolumes(document)
	suite.createSpec(suite.volumeMount())

	requestID := mountControllerName + "/" + mountContainer + "/" + volumeID

	ctest.AssertResource(suite, requestID, func(request *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(volumeID, request.TypedSpec().VolumeID)
	})

	status := block.NewVolumeMountStatus(block.NamespaceName, requestID)
	status.TypedSpec().VolumeID = volumeID
	status.TypedSpec().Requester = mountControllerName
	status.TypedSpec().Target = testVolumeTarget

	suite.Require().NoError(suite.State().Create(suite.Ctx(), status))

	ctest.AssertResource(suite, mountContainer, func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)

		if !asrt.Len(status.TypedSpec().Mounts, 1) {
			return
		}

		asrt.Equal(testVolumeTarget, status.TypedSpec().Mounts[0].Source)
		asrt.Equal(testVolumeName, status.TypedSpec().Mounts[0].VolumeName)
		asrt.Equal(volumeID, status.TypedSpec().Mounts[0].VolumeID)
	})
}

// TestFollowsVolumeKindChange covers the volume document behind a name changing kind while the
// container spec stays the same: the controller re-resolves the name and requests the new volume.
func (suite *MountSuite) TestFollowsVolumeKindChange() {
	suite.createSpec(suite.volumeMount())

	ctest.AssertResource(suite, testRequestID, func(*block.VolumeMountRequest, *assert.Assertions) {})

	existingVolume := blockcfg.NewExistingVolumeConfigV1Alpha1()
	existingVolume.MetaName = testVolumeName

	suite.replaceVolumes(existingVolume)

	ctest.AssertResource(suite, mountControllerName+"/"+mountContainer+"/e-"+testVolumeName,
		func(*block.VolumeMountRequest, *assert.Assertions) {})
	ctest.AssertNoResource[*block.VolumeMountRequest](suite, testRequestID)
}

// TestUnresolvedVolumeIsNotRequested covers a mount whose name resolves to no volume: nothing is
// requested from the block subsystem, and the container is held back.
func (suite *MountSuite) TestUnresolvedVolumeIsNotRequested() {
	suite.replaceVolumes()
	suite.createSpec(suite.volumeMount())

	ctest.AssertResource(suite, mountContainer, func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Ready)
		asrt.Equal(`no UserVolumeConfig, ExistingVolumeConfig or ExternalVolumeConfig named "web-content" is configured`, status.TypedSpec().Error)
	})

	requests, err := safe.StateListAll[*block.VolumeMountRequest](suite.Ctx(), suite.State())
	suite.Require().NoError(err)
	suite.Assert().Zero(requests.Len())
}

// TestReadOnlyMountIsNotAcceptedForWritable covers the volume coming back with less access than was
// asked for.
//
// Mount requests are merged per volume and end up read-only if every requester asked for read-only,
// so another holder can decide this. Reporting ready would give the container a read-only mount it
// did not ask for.
func (suite *MountSuite) TestReadOnlyMountIsNotAcceptedForWritable() {
	suite.createSpec(suite.volumeMount())

	ctest.AssertResource(suite, testRequestID, func(*block.VolumeMountRequest, *assert.Assertions) {})

	suite.satisfyMount(true)

	ctest.AssertResource(suite, mountContainer, func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Ready)
		asrt.Equal(`volume "web-content" is mounted read-only`, status.TypedSpec().Error)
	})
}

// TestTearingDownMountMarksNotReady covers the volume behind a live mount going away.
//
// The mount is reported not-ready while the finalizer stays put: it keeps the volume mounted until the
// container has actually gone.
func (suite *MountSuite) TestTearingDownMountMarksNotReady() {
	suite.assertTearingDownMountMarksNotReady(nil, testVolumeID)
}

// TestTearingDownExistingVolumeMountMarksNotReady is TestTearingDownMountMarksNotReady for an
// existing volume.
func (suite *MountSuite) TestTearingDownExistingVolumeMountMarksNotReady() {
	doc := blockcfg.NewExistingVolumeConfigV1Alpha1()
	doc.MetaName = testVolumeName

	suite.assertTearingDownMountMarksNotReady(doc, "e-"+testVolumeName)
}

// TestTearingDownExternalVolumeMountMarksNotReady is TestTearingDownMountMarksNotReady for an
// external volume.
func (suite *MountSuite) TestTearingDownExternalVolumeMountMarksNotReady() {
	doc := blockcfg.NewExternalVolumeConfigV1Alpha1()
	doc.MetaName = testVolumeName

	suite.assertTearingDownMountMarksNotReady(doc, "x-"+testVolumeName)
}

// assertTearingDownMountMarksNotReady declares the test volume with document, unless it is nil and
// the default user volume is used, and checks a live mount of volumeID being torn down is reported
// not-ready while the hold stays.
func (suite *MountSuite) assertTearingDownMountMarksNotReady(document configcfg.Document, volumeID string) {
	requestID := suite.prepareVolume(document, volumeID)

	suite.createSpec(suite.volumeMount())

	ctest.AssertResource(suite, requestID, func(*block.VolumeMountRequest, *assert.Assertions) {})
	suite.satisfyMountFor(requestID, volumeID, false)
	suite.assertMountsReady(true)

	// A live instance is what stops the hold being released while the container still runs.
	suite.createInstance()

	statusMD := block.NewVolumeMountStatus(block.NamespaceName, requestID).Metadata()

	_, err := suite.State().Teardown(suite.Ctx(), statusMD)
	suite.Require().NoError(err)

	suite.assertMountsReady(false)

	ctest.AssertResource(suite, mountContainer, func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
		asrt.Equal(`volume "web-content" is pending unmount, waiting for the container to stop`, status.TypedSpec().Error)
	})

	ctest.AssertResource(suite, requestID, func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Has(mountControllerName),
			"the hold was released while the container was still running")
	})
}

// TestReleasesTearingDownMountOnceInstanceStops is the other half: once the task exits, the hold is
// released so the volume can be unmounted, and taken again once the volume is mounted anew.
//
// This is what lets an external volume whose mount parameters changed be remounted: the request is
// still wanted, so it is never released the way an unwanted one is, and the exited instance is kept
// around until its replacement's mounts are ready.
func (suite *MountSuite) TestReleasesTearingDownMountOnceInstanceStops() {
	suite.assertReleasesTearingDownMountOnceInstanceStops(nil, testVolumeID)
}

// TestReleasesTearingDownExistingVolumeMountOnceInstanceStops is
// TestReleasesTearingDownMountOnceInstanceStops for an existing volume.
func (suite *MountSuite) TestReleasesTearingDownExistingVolumeMountOnceInstanceStops() {
	doc := blockcfg.NewExistingVolumeConfigV1Alpha1()
	doc.MetaName = testVolumeName

	suite.assertReleasesTearingDownMountOnceInstanceStops(doc, "e-"+testVolumeName)
}

// TestReleasesTearingDownExternalVolumeMountOnceInstanceStops is
// TestReleasesTearingDownMountOnceInstanceStops for an external volume.
func (suite *MountSuite) TestReleasesTearingDownExternalVolumeMountOnceInstanceStops() {
	doc := blockcfg.NewExternalVolumeConfigV1Alpha1()
	doc.MetaName = testVolumeName

	suite.assertReleasesTearingDownMountOnceInstanceStops(doc, "x-"+testVolumeName)
}

// assertReleasesTearingDownMountOnceInstanceStops declares the test volume as for
// assertTearingDownMountMarksNotReady, and checks the hold on a torn down mount of volumeID is
// released once the task exits and taken again once the volume is mounted anew.
func (suite *MountSuite) assertReleasesTearingDownMountOnceInstanceStops(document configcfg.Document, volumeID string) {
	requestID := suite.prepareVolume(document, volumeID)

	suite.createSpec(suite.volumeMount())

	ctest.AssertResource(suite, requestID, func(*block.VolumeMountRequest, *assert.Assertions) {})
	suite.satisfyMountFor(requestID, volumeID, false)
	suite.assertMountsReady(true)

	suite.createInstance()

	statusMD := block.NewVolumeMountStatus(block.NamespaceName, requestID).Metadata()

	_, err := suite.State().Teardown(suite.Ctx(), statusMD)
	suite.Require().NoError(err)

	ctest.AssertResource(suite, mountContainer, func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
		asrt.Equal(`volume "web-content" is pending unmount, waiting for the container to stop`, status.TypedSpec().Error)
	})

	// The task exits; the instance itself is still there.
	suite.finishInstance()

	ctest.AssertResource(suite, requestID, func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.False(status.Metadata().Finalizers().Has(mountControllerName),
			"the hold was kept after the container stopped")
	})

	ctest.AssertResource(suite, mountContainer, func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Ready)
		asrt.Equal(`volume "web-content" is being unmounted`, status.TypedSpec().Error)
	})

	// The request is still wanted, so it survives the release.
	ctest.AssertResource(suite, requestID, func(request *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseRunning, request.Metadata().Phase())
	})

	// The block subsystem finishes the unmount and mounts the volume again.
	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), statusMD))
	suite.satisfyMountFor(requestID, volumeID, false)

	suite.assertMountsReady(true)

	ctest.AssertResource(suite, requestID, func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Has(mountControllerName))
	})
}

// prepareVolume declares the test volume with document, keeping the default user volume when it is
// nil, and returns the ID of the request the controller makes for volumeID.
func (suite *MountSuite) prepareVolume(document configcfg.Document, volumeID string) string {
	if document != nil {
		suite.replaceVolumes(document)
	}

	return mountControllerName + "/" + mountContainer + "/" + volumeID
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

	suite.createSpec(suite.volumeMount())

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
	suite.createSpec(suite.volumeMount())

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
	suite.createSpec(suite.volumeMount())

	ctest.AssertResource(suite, testRequestID, func(*block.VolumeMountRequest, *assert.Assertions) {})
	suite.satisfyMount(false)
	suite.assertMountsReady(true)

	suite.createInstance()

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
		containers.NewContainerInstanceSpec(containers.NamespaceName, testInstanceID).Metadata()))

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, testRequestID)
}

// TestReleasesUnwantedMountOnceInstanceExits covers a mount removed from the spec of a container whose
// task has already exited, while its instance is still around.
func (suite *MountSuite) TestReleasesUnwantedMountOnceInstanceExits() {
	suite.createSpec(suite.volumeMount())

	ctest.AssertResource(suite, testRequestID, func(*block.VolumeMountRequest, *assert.Assertions) {})
	suite.satisfyMount(false)
	suite.assertMountsReady(true)

	suite.createInstance()
	suite.finishInstance()

	ctest.UpdateWithConflicts(suite, containers.NewContainerSpec(containers.NamespaceName, mountContainer),
		func(spec *containers.ContainerSpec) error {
			spec.TypedSpec().Mounts = nil

			return nil
		})

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, testRequestID)
}

// TestReleasesEverythingOnLifecycleTeardown covers the shutdown path.
//
// The barrier is what the stopContainers phase waits on, so it has to be released; the container spec
// still exists at that point, and treating it as a reason to keep the mount would hang the shutdown.
func (suite *MountSuite) TestReleasesEverythingOnLifecycleTeardown() {
	suite.createSpec(suite.volumeMount())

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
