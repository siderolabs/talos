// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

const (
	// testRawVolumeName is the raw volume a device names.
	testRawVolumeName = "data"
	// testRawVolumeID is the block volume ID the raw volume is known by.
	testRawVolumeID = "r-" + testRawVolumeName
	// testRawVolumeLocation is the partition backing the raw volume.
	testRawVolumeLocation = "/dev/sdb1"
	// testRawVolumeMountLocation is the unlocked device of the encrypted raw volume.
	testRawVolumeMountLocation = "/dev/dm-0"
	// otherContainer is a second container competing for the raw volume.
	otherContainer = "osd"
)

// deviceHoldFinalizer mirrors the controller's naming of the hold on a raw volume.
func deviceHoldFinalizer(containerID string) string {
	return mountControllerName + "/" + containerID
}

func rawVolumeDevice() containers.ContainerDeviceSpec {
	return containers.ContainerDeviceSpec{
		Kind:        containers.DeviceKindRawVolume,
		VolumeName:  testRawVolumeName,
		Destination: "/dev/data",
	}
}

func (suite *MountSuite) createDeviceSpec(containerID string, devices ...containers.ContainerDeviceSpec) {
	spec := containers.NewContainerSpec(containers.NamespaceName, containerID)
	spec.TypedSpec().Image = containers.ContainerImageSpec{Ref: "docker.io/library/nginx:latest"}
	spec.TypedSpec().Devices = devices

	suite.Require().NoError(suite.State().Create(suite.Ctx(), spec))
}

func (suite *MountSuite) removeDevices() {
	ctest.UpdateWithConflicts(suite, containers.NewContainerSpec(containers.NamespaceName, mountContainer),
		func(spec *containers.ContainerSpec) error {
			spec.TypedSpec().Devices = nil

			return nil
		})
}

// createRawVolume creates the VolumeStatus the block subsystem would produce for the raw volume.
func (suite *MountSuite) createRawVolume(phase block.VolumePhase) {
	status := block.NewVolumeStatus(block.NamespaceName, testRawVolumeID)
	status.Metadata().Labels().Set(block.RawVolumeLabel, "")
	status.TypedSpec().Phase = phase
	status.TypedSpec().Location = testRawVolumeLocation

	if phase == block.VolumePhaseReady {
		status.TypedSpec().MountLocation = testRawVolumeMountLocation
	}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), status))
}

func (suite *MountSuite) rawVolumeMD() *resource.Metadata {
	return block.NewVolumeStatus(block.NamespaceName, testRawVolumeID).Metadata()
}

func (suite *MountSuite) assertRawVolumeHeldBy(containerIDs ...string) {
	ctest.AssertResource(suite, testRawVolumeID, func(status *block.VolumeStatus, asrt *assert.Assertions) {
		var holders []string

		for _, finalizer := range *status.Metadata().Finalizers() {
			if strings.HasPrefix(finalizer, deviceHoldFinalizer("")) {
				holders = append(holders, finalizer)
			}
		}

		expected := make([]string, 0, len(containerIDs))

		for _, containerID := range containerIDs {
			expected = append(expected, deviceHoldFinalizer(containerID))
		}

		asrt.ElementsMatch(expected, holders)
	})
}

func (suite *MountSuite) assertMountStatusError(containerID, expected string) {
	ctest.AssertResource(suite, containerID, func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Ready)
		asrt.Equal(expected, status.TypedSpec().Error)
	})
}

// createDeviceInstance stands in for the instance controller having created an instance of mountContainer
// using the raw volume, or not using it if withDevice is false.
func (suite *MountSuite) createDeviceInstance(generation uint64, withDevice bool) {
	instance := containers.NewContainerInstanceSpec(containers.NamespaceName, containers.InstanceID(mountContainer, generation))
	instance.TypedSpec().ContainerID = mountContainer
	instance.TypedSpec().Generation = generation

	if withDevice {
		instance.TypedSpec().Devices = []containers.ResolvedDeviceSpec{
			{
				Kind:        containers.DeviceKindRawVolume,
				Source:      testRawVolumeMountLocation,
				Destination: "/dev/data",
				VolumeID:    testRawVolumeID,
				VolumeName:  testRawVolumeName,
			},
		}
	}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), instance))
}

func (suite *MountSuite) finishDeviceInstance(generation uint64) {
	status := containers.NewContainerInstanceStatus(containers.NamespaceName, containers.InstanceID(mountContainer, generation))
	status.TypedSpec().ContainerID = mountContainer
	status.TypedSpec().Generation = generation
	status.TypedSpec().Phase = containers.ContainerInstancePhaseTerminated
	status.TypedSpec().FinishedAt = time.Now()

	suite.Require().NoError(suite.State().Create(suite.Ctx(), status))
}

// readyRawVolume brings mountContainer to a ready raw volume device.
func (suite *MountSuite) readyRawVolume() {
	suite.createDeviceSpec(mountContainer, rawVolumeDevice())
	suite.createRawVolume(block.VolumePhaseReady)
	suite.assertMountsReady(true)
	suite.assertRawVolumeHeldBy(mountContainer)
}

func (suite *MountSuite) TestHoldsAndResolvesRawVolume() {
	suite.createDeviceSpec(mountContainer, rawVolumeDevice())

	suite.assertMountStatusError(mountContainer, `waiting for raw volume "data" to be ready`)

	// Held as soon as it exists, so it is not closed while e.g. still being unlocked.
	suite.createRawVolume(block.VolumePhaseWaiting)
	suite.assertRawVolumeHeldBy(mountContainer)
	suite.assertMountStatusError(mountContainer, `waiting for raw volume "data" to be ready`)

	ctest.UpdateWithConflicts(suite, block.NewVolumeStatus(block.NamespaceName, testRawVolumeID),
		func(status *block.VolumeStatus) error {
			status.TypedSpec().Phase = block.VolumePhaseReady
			status.TypedSpec().MountLocation = testRawVolumeMountLocation

			return nil
		})

	ctest.AssertResource(suite, mountContainer, func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)

		// The unlocked device, not the partition behind it.
		asrt.Equal([]containers.ResolvedDeviceSpec{
			{
				Kind:        containers.DeviceKindRawVolume,
				Source:      testRawVolumeMountLocation,
				Destination: "/dev/data",
				VolumeID:    testRawVolumeID,
				VolumeName:  testRawVolumeName,
			},
		}, status.TypedSpec().Devices)
	})
}

func (suite *MountSuite) TestKeepsClosingRawVolumeUntilInstanceStops() {
	suite.readyRawVolume()
	suite.createDeviceInstance(0, true)

	_, err := suite.State().Teardown(suite.Ctx(), suite.rawVolumeMD())
	suite.Require().NoError(err)

	suite.assertMountStatusError(mountContainer, `raw volume "data" is being closed, waiting for the container to stop`)
	suite.assertRawVolumeHeldBy(mountContainer)

	suite.finishDeviceInstance(0)

	suite.assertRawVolumeHeldBy()
	suite.assertMountStatusError(mountContainer, `raw volume "data" is being closed`)
}

func (suite *MountSuite) TestDoesNotHoldClosingRawVolume() {
	// Closing before the container exists, and pinned there by a foreign finalizer.
	suite.createRawVolume(block.VolumePhaseReady)
	suite.AddFinalizer(suite.rawVolumeMD(), "test")

	_, err := suite.State().Teardown(suite.Ctx(), suite.rawVolumeMD())
	suite.Require().NoError(err)

	suite.createDeviceSpec(mountContainer, rawVolumeDevice())

	suite.assertMountStatusError(mountContainer, `raw volume "data" is being closed`)
	suite.assertRawVolumeHeldBy()

	suite.RemoveFinalizer(suite.rawVolumeMD(), "test")
}

func (suite *MountSuite) TestReleasesRawVolumeWhenDeviceIsRemoved() {
	suite.readyRawVolume()

	suite.removeDevices()

	suite.assertRawVolumeHeldBy()
	suite.assertMountsReady(true)
}

func (suite *MountSuite) TestKeepsRawVolumeWhileInstanceUsesIt() {
	suite.readyRawVolume()
	suite.createDeviceInstance(0, true)

	suite.removeDevices()

	ctest.AssertResource(suite, mountContainer, func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
		asrt.Empty(status.TypedSpec().Devices)
	})

	suite.assertRawVolumeHeldBy(mountContainer)

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(),
		containers.NewContainerInstanceSpec(containers.NamespaceName, containers.InstanceID(mountContainer, 0)).Metadata()))

	suite.assertRawVolumeHeldBy()
}

// TestReplacementWithoutDeviceReleasesRawVolume covers liveness being decided per instance: the
// container keeps running, but its live instance no longer uses the raw volume.
func (suite *MountSuite) TestReplacementWithoutDeviceReleasesRawVolume() {
	suite.readyRawVolume()
	suite.createDeviceInstance(0, true)

	suite.removeDevices()
	suite.assertRawVolumeHeldBy(mountContainer)

	suite.finishDeviceInstance(0)
	suite.createDeviceInstance(1, false)

	suite.assertRawVolumeHeldBy()
}

func (suite *MountSuite) TestRawVolumeMovesToAnotherContainerOnceStopped() {
	suite.readyRawVolume()
	suite.createDeviceInstance(0, true)

	suite.removeDevices()
	suite.createDeviceSpec(otherContainer, rawVolumeDevice())

	suite.assertMountStatusError(otherContainer, `raw volume "data" is held by container "nginx"`)
	suite.assertRawVolumeHeldBy(mountContainer)

	suite.finishDeviceInstance(0)

	suite.assertRawVolumeHeldBy(otherContainer)

	ctest.AssertResource(suite, otherContainer, func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)
	})
}

// TestReleasesLeftoverRawVolumeHold covers a hold with no container left behind it, as found after a
// controller restart.
func (suite *MountSuite) TestReleasesLeftoverRawVolumeHold() {
	suite.createRawVolume(block.VolumePhaseReady)
	suite.AddFinalizer(suite.rawVolumeMD(), deviceHoldFinalizer("ghost"))

	suite.createDeviceSpec(mountContainer)

	suite.assertRawVolumeHeldBy()
}

// TestKeepsRawVolumeUntilContainersAreStopped covers the shutdown path: a raw volume already being
// closed closes as soon as it is released, so it is held until RuntimeController has stopped every
// container.
func (suite *MountSuite) TestKeepsRawVolumeUntilContainersAreStopped() {
	const runtimeControllerName = "containers.RuntimeController"

	suite.readyRawVolume()
	suite.createDeviceInstance(0, true)

	lifecycleMD := containers.NewContainerLifecycle(containers.NamespaceName, containers.ContainerLifecycleID).Metadata()

	suite.AddFinalizer(lifecycleMD, runtimeControllerName)

	_, err := suite.State().Teardown(suite.Ctx(), lifecycleMD)
	suite.Require().NoError(err)

	_, err = suite.State().Teardown(suite.Ctx(), suite.rawVolumeMD())
	suite.Require().NoError(err)

	ctest.AssertResource(suite, containers.ContainerLifecycleID, func(lifecycle *containers.ContainerLifecycle, asrt *assert.Assertions) {
		asrt.True(lifecycle.Metadata().Finalizers().Has(mountControllerName))
	})

	suite.assertRawVolumeHeldBy(mountContainer)

	suite.RemoveFinalizer(lifecycleMD, runtimeControllerName)

	_, err = suite.State().WatchFor(suite.Ctx(), lifecycleMD, state.WithFinalizerEmpty())
	suite.Require().NoError(err)

	suite.assertRawVolumeHeldBy()
}
