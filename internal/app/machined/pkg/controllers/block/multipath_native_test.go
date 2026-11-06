// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/go-blockdevice/v2/devicemapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/dmmultipath"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/sysblock"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

type NativeMultipathSuite struct {
	ctest.DefaultSuite
}

// TestNativeMultipathSuite runs against real SCSI disks reachable via multiple paths: it needs a
// machine with a logical unit which is reported by at least two SCSI disks with the same NAA or
// EUI-64 identifier, e.g. a QEMU machine with the same image attached as two scsi-hd devices with
// the same "wwn".
//
// The test writes a partition table to that logical unit.
func TestNativeMultipathSuite(t *testing.T) {
	suite.Run(t, new(NativeMultipathSuite))
}

// TestAssemble checks that Talos assembles a multipath device over the paths of a disk, never puts
// the paths to use, keeps the paths up to date, and leaves the multipath device to multipathd once
// it is running.
//
//nolint:gocyclo,cyclop,maintidx
func (suite *NativeMultipathSuite) TestAssemble() {
	if os.Geteuid() != 0 {
		suite.T().Skip("skipping test; must be root")
	}

	if _, err := os.Stat(devicemapper.ControlPath); err != nil {
		suite.T().Skip("skipping test; device-mapper control device is not available")
	}

	wwid, paths := findMultipathDisk(suite.T())
	uuid := sysblock.MultipathUUID(wwid)

	suite.T().Logf("multipath disk %s, paths %v", wwid, paths)

	// the table is written via each path, so that the kernel partitions of each path are in place,
	// just like after the kernel scanned the partition table of each path on boot
	for _, path := range paths {
		writeMETAPartition(suite.T(), filepath.Join("/dev", path))
	}

	suite.T().Cleanup(func() {
		for _, path := range paths {
			setSCSIDeviceState(suite.T(), path, "running")
		}

		if mpath := findMultipathDevice(uuid); mpath != "" {
			removeMultipathDevice(suite.T(), mpath)
		}
	})

	for _, ctrl := range []controller.Controller{
		&blockctrls.DevicesController{},
		&blockctrls.MultipathController{Enabled: true},
		&blockctrls.DiscoveryController{NativeMultipath: true},
		&blockctrls.DisksController{NativeMultipath: true},
	} {
		suite.Require().NoError(suite.Runtime().RegisterController(ctrl))
	}

	// the multipath device is assembled over every path, and it is the only disk of the logical unit
	var mpath string

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		mpath = findMultipathDevice(uuid)
		if !assert.NotEmpty(collect, mpath, "multipath device is not assembled") {
			return
		}

		disk, err := safe.StateGetByID[*block.Disk](suite.Ctx(), suite.State(), mpath)
		if !assert.NoError(collect, err) {
			return
		}

		assert.Equal(collect, "mpath", disk.TypedSpec().DeviceMapperKind)
		assert.Equal(collect, wwid, disk.TypedSpec().DeviceMapperName)
		assert.ElementsMatch(collect, paths, disk.TypedSpec().SecondaryDisks)
		assert.NotEmpty(collect, disk.TypedSpec().WWID, "the WWID is inherited from the paths")
		assert.NotEmpty(collect, disk.TypedSpec().Model, "the model is inherited from the paths")
	}, 30*time.Second, 100*time.Millisecond)

	for _, path := range paths {
		ctest.AssertNoResource[*block.Disk](suite, path)
	}

	// the partitions are only discovered on the multipath device
	suite.assertMETAOnlyOn(mpath, paths)

	pathA, pathB := paths[0], paths[1]

	// a path which can't be reached is dropped, and the disk stays readable via the other path
	setSCSIDeviceState(suite.T(), pathB, "offline")

	suite.assertActivePaths(mpath, pathA)
	suite.Require().NoError(readFirstBlock(filepath.Join("/dev", mpath)))

	// once it is reachable again, it is added back
	setSCSIDeviceState(suite.T(), pathB, "running")

	suite.assertActivePaths(mpath, pathA, pathB)

	// a path which the kernel failed, while its device stayed reachable, is reinstated
	pathBDevNo := devNo(suite.T(), pathB)

	suite.Require().NoError(dmmultipath.FailPath(uuid, pathBDevNo))
	suite.assertPathState(uuid, pathBDevNo, false, time.Second)
	suite.assertPathState(uuid, pathBDevNo, true, 30*time.Second)

	// once multipathd is running, the multipath device is left to it
	multipathd := v1alpha1.NewService(blockctrls.MultipathdServiceID)
	multipathd.TypedSpec().Running = true
	suite.Create(multipathd)

	// give the controller a chance to observe multipathd
	time.Sleep(time.Second)

	suite.Require().NoError(dmmultipath.FailPath(uuid, pathBDevNo))

	// a few rounds of resync, the path is never reinstated
	time.Sleep(25 * time.Second)
	suite.assertPathState(uuid, pathBDevNo, false, time.Second)

	// multipathd stops, the multipath device is managed by Talos again
	suite.Destroy(multipathd)
	suite.assertPathState(uuid, pathBDevNo, true, 30*time.Second)

	// the disks which can't be paths of a multipath device are left alone
	disks, err := safe.StateListAll[*block.Disk](suite.Ctx(), suite.State())
	suite.Require().NoError(err)

	for disk := range disks.All() {
		suite.T().Logf("disk %s: device mapper kind %q, secondaries %v", disk.Metadata().ID(), disk.TypedSpec().DeviceMapperKind, disk.TypedSpec().SecondaryDisks)
	}
}

func (suite *NativeMultipathSuite) assertMETAOnlyOn(mpath string, paths []string) {
	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		volumes, err := safe.StateListAll[*block.DiscoveredVolume](suite.Ctx(), suite.State())
		require.NoError(collect, err)

		var parents []string

		for dv := range volumes.All() {
			assert.NotContains(collect, paths, dv.Metadata().ID())
			assert.NotContains(collect, paths, dv.TypedSpec().Parent)

			if dv.TypedSpec().PartitionLabel == "META" {
				parents = append(parents, dv.TypedSpec().Parent)
			}
		}

		assert.Equal(collect, []string{mpath}, parents)
	}, 30*time.Second, 100*time.Millisecond)
}

func (suite *NativeMultipathSuite) assertActivePaths(mpath string, paths ...string) {
	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		disk, err := safe.StateGetByID[*block.Disk](suite.Ctx(), suite.State(), mpath)
		if !assert.NoError(collect, err) {
			return
		}

		assert.ElementsMatch(collect, paths, disk.TypedSpec().SecondaryDisks)
	}, 30*time.Second, 100*time.Millisecond)
}

func (suite *NativeMultipathSuite) assertPathState(uuid, devNo string, active bool, timeout time.Duration) {
	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		states, err := dmmultipath.PathStates(uuid)
		if !assert.NoError(collect, err) {
			return
		}

		assert.Equal(collect, active, states[devNo], "path %s, states %v", devNo, states)
	}, timeout, 100*time.Millisecond)
}

// findMultipathDisk finds the first logical unit which is reachable via more than one path.
func findMultipathDisk(t *testing.T) (string, []string) {
	t.Helper()

	entries, err := os.ReadDir("/sys/block")
	require.NoError(t, err)

	groups := map[string][]string{}

	for _, entry := range entries {
		if wwid, ok := sysblock.MultipathWWID(filepath.Join("/sys/block", entry.Name())); ok {
			groups[wwid] = append(groups[wwid], entry.Name())
		}
	}

	wwids := make([]string, 0, len(groups))

	for wwid, paths := range groups {
		if len(paths) > 1 {
			wwids = append(wwids, wwid)
		}
	}

	if len(wwids) == 0 {
		t.Skip("skipping test; no disk is reachable via multiple paths")
	}

	slices.Sort(wwids)

	paths := groups[wwids[0]]
	slices.Sort(paths)

	return wwids[0], paths
}

// findMultipathDevice returns the kernel name of the multipath device with the UUID, if it is assembled.
func findMultipathDevice(uuid string) string {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if sysblock.DeviceMapperUUID(filepath.Join("/sys/block", entry.Name())) == uuid {
			return entry.Name()
		}
	}

	return ""
}

func setSCSIDeviceState(t *testing.T, path, state string) {
	t.Helper()

	// the kernel only takes the state with a trailing newline
	require.NoError(t, os.WriteFile(filepath.Join("/sys/block", path, "device", "state"), []byte(state+"\n"), 0o644))
}

func devNo(t *testing.T, path string) string {
	t.Helper()

	var st unix.Stat_t

	require.NoError(t, unix.Stat(filepath.Join("/dev", path), &st))

	return fmt.Sprintf("%d:%d", unix.Major(st.Rdev), unix.Minor(st.Rdev))
}

func readFirstBlock(devPath string) error {
	f, err := os.Open(devPath)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	_, err = f.ReadAt(make([]byte, 4096), 0)

	return err
}
