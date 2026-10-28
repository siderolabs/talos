// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

func (suite *InstanceSuite) createRawVolumeDeviceSpec() {
	suite.createSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Devices = []containers.ContainerDeviceSpec{rawVolumeDevice()}
	})
	suite.markImageReady()
}

func (suite *InstanceSuite) createReadyRawVolume(mountLocation string) {
	suite.createNamedReadyRawVolume(testRawVolumeName, mountLocation)
}

func (suite *InstanceSuite) createNamedReadyRawVolume(name, mountLocation string) {
	status := block.NewVolumeStatus(block.NamespaceName, "r-"+name)
	status.Metadata().Labels().Set(block.RawVolumeLabel, "")
	status.TypedSpec().Phase = block.VolumePhaseReady
	status.TypedSpec().Location = testRawVolumeLocation
	status.TypedSpec().MountLocation = mountLocation

	suite.Require().NoError(suite.State().Create(suite.Ctx(), status))
}

func namedRawVolumeDevice(name string) containers.ContainerDeviceSpec {
	return containers.ContainerDeviceSpec{
		Kind:        containers.DeviceKindRawVolume,
		VolumeName:  name,
		Destination: "/dev/data",
	}
}

// assertRunsOnDevice asserts that an instance of the container uses the raw volume device at source.
func (suite *InstanceSuite) assertRunsOnDevice(containerID, source string) {
	suite.AssertWithin(10*time.Second, 50*time.Millisecond, func() error {
		instances, err := safe.StateListAll[*containers.ContainerInstanceSpec](suite.Ctx(), suite.State())
		if err != nil {
			return err
		}

		for instance := range instances.All() {
			if instance.TypedSpec().ContainerID == containerID &&
				len(instance.TypedSpec().Devices) == 1 &&
				instance.TypedSpec().Devices[0].Source == source {
				return nil
			}
		}

		return retry.ExpectedErrorf("no instance of %q runs on %q", containerID, source)
	})
}

func (suite *InstanceSuite) assertInstanceDeviceSource(generation uint64, source string) {
	ctest.AssertResource(suite, containers.InstanceID(testContainer, generation), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal([]containers.ResolvedDeviceSpec{
			{
				Kind:        containers.DeviceKindRawVolume,
				Source:      source,
				Destination: "/dev/data",
				VolumeID:    testRawVolumeID,
				VolumeName:  testRawVolumeName,
			},
		}, instance.TypedSpec().Devices)
	})
}

func (suite *InstanceSuite) TestRawVolumeDeviceStaysPending() {
	suite.createRawVolumeDeviceSpec()

	suite.assertWaitingFor("devices")
	suite.tick()
	suite.assertNoInstance(0)
}

func (suite *InstanceSuite) TestStartsOnceRawVolumeIsReady() {
	suite.createRawVolumeDeviceSpec()
	suite.assertWaitingFor("devices")

	suite.createReadyRawVolume(testRawVolumeMountLocation)

	suite.assertInstanceDeviceSource(0, testRawVolumeMountLocation)
	suite.assertWaitingFor()
}

// TestRawVolumeDeviceChangeReplacesInstance covers the device behind a raw volume changing, e.g. the
// encrypted volume being opened again as another dm device: the instance has to be replaced, as the
// device node it was started with is no longer the raw volume.
func (suite *InstanceSuite) TestRawVolumeDeviceChangeReplacesInstance() {
	suite.createRawVolumeDeviceSpec()
	suite.createReadyRawVolume(testRawVolumeMountLocation)
	suite.assertInstanceDeviceSource(0, testRawVolumeMountLocation)

	ctest.UpdateWithConflicts(suite, block.NewVolumeStatus(block.NamespaceName, testRawVolumeID),
		func(status *block.VolumeStatus) error {
			status.TypedSpec().MountLocation = "/dev/dm-1"

			return nil
		})

	suite.assertInstanceDeviceSource(1, "/dev/dm-1")
	suite.assertNoInstance(0)
}

// TestRawVolumeSwapBetweenContainers covers two running containers swapping their raw volumes: each
// replacement waits for the raw volume the other container holds, so the old instances have to give
// theirs up without waiting for their replacements.
func (suite *InstanceSuite) TestRawVolumeSwapBetweenContainers() {
	const (
		first  = "osd-a"
		second = "osd-b"
	)

	suite.createNamedReadyRawVolume("x", "/dev/dm-0")
	suite.createNamedReadyRawVolume("y", "/dev/dm-1")

	suite.createNamedSpec(first, func(spec *containers.ContainerSpecSpec) {
		spec.Devices = []containers.ContainerDeviceSpec{namedRawVolumeDevice("x")}
	})
	suite.markNamedImageReady(first, testDigest)

	suite.createNamedSpec(second, func(spec *containers.ContainerSpecSpec) {
		spec.Devices = []containers.ContainerDeviceSpec{namedRawVolumeDevice("y")}
	})
	suite.markNamedImageReady(second, testDigest)

	suite.assertRunsOnDevice(first, "/dev/dm-0")
	suite.assertRunsOnDevice(second, "/dev/dm-1")

	for containerID, volumeName := range map[string]string{first: "y", second: "x"} {
		ctest.UpdateWithConflicts(suite, containers.NewContainerSpec(containers.NamespaceName, containerID),
			func(spec *containers.ContainerSpec) error {
				spec.TypedSpec().Devices = []containers.ContainerDeviceSpec{namedRawVolumeDevice(volumeName)}

				return nil
			})
	}

	suite.assertRunsOnDevice(first, "/dev/dm-1")
	suite.assertRunsOnDevice(second, "/dev/dm-0")
}

// TestAddedRawVolumeWaitsWithoutStopping covers a raw volume added to a running container: nothing is
// revoked, so the running instance is kept until the replacement can start.
func (suite *InstanceSuite) TestAddedRawVolumeWaitsWithoutStopping() {
	suite.createRawVolumeDeviceSpec()
	suite.createReadyRawVolume(testRawVolumeMountLocation)
	suite.assertInstanceDeviceSource(0, testRawVolumeMountLocation)

	suite.updateSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Devices = append(spec.Devices, containers.ContainerDeviceSpec{
			Kind:        containers.DeviceKindRawVolume,
			VolumeName:  "extra",
			Destination: "/dev/extra",
		})
	})

	suite.assertWaitingFor("devices")
	suite.tick()
	suite.assertInstanceDeviceSource(0, testRawVolumeMountLocation)
	suite.assertNoInstance(1)

	suite.createNamedReadyRawVolume("extra", "/dev/dm-1")

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 1), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Len(instance.TypedSpec().Devices, 2)
	})
}
