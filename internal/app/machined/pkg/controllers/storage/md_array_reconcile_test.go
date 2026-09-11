// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	"github.com/siderolabs/talos/internal/pkg/md"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// fakeMDProvisioner is an in-memory MDProvisioner. It records mutations as sets
// so the reconcile loop re-running on every event stays assertion-friendly: a
// member added twice (because the fake never mirrors the resulting on-disk
// state) still reads as a single add.
type fakeMDProvisioner struct {
	mu sync.Mutex

	// findByMember maps member device path -> md node; a missing entry yields
	// md.ErrNotFound, matching the real FindDeviceByMember contract.
	findByMember map[string]string
	// details maps md node -> Detail returned by DetailDevice.
	details map[string]md.Detail
	// syncAction maps md node -> current sync action (default idle).
	syncAction map[string]md.SyncAction

	// createNode is the node Create returns; createErr overrides success.
	createNode string
	createErr  error
	// createNodes overrides the default node for tests with independent arrays.
	createNodes map[string]string

	creates map[string]md.CreateOptions    // name -> requested array shape
	adds    map[string]map[string]struct{} // device -> added members
	grows   map[string]int                 // device -> target raid devices
}

func newFakeMDProvisioner() *fakeMDProvisioner {
	return &fakeMDProvisioner{
		findByMember: map[string]string{},
		details:      map[string]md.Detail{},
		syncAction:   map[string]md.SyncAction{},
		createNode:   "/dev/md0",
		createNodes:  map[string]string{},
		creates:      map[string]md.CreateOptions{},
		adds:         map[string]map[string]struct{}{},
		grows:        map[string]int{},
	}
}

func (f *fakeMDProvisioner) Create(_ context.Context, name string, opts md.CreateOptions) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.creates[name]; !ok {
		recorded := opts
		recorded.Devices = append([]string(nil), opts.Devices...)
		f.creates[name] = recorded
	}

	node := f.createNode
	if override, ok := f.createNodes[name]; ok {
		node = override
	}

	if f.createErr != nil {
		return node, f.createErr
	}

	if _, ok := f.details[node]; !ok {
		f.details[node] = md.Detail{
			Level:       fmt.Sprintf("raid%d", opts.Level),
			RaidDevices: opts.RaidDevices,
			Members:     append([]string(nil), opts.Devices...),
			Name:        "talos:" + name,
			UUID:        "test-" + name,
			Metadata:    opts.Metadata,
		}
	}

	for _, member := range opts.Devices {
		f.findByMember[member] = node
	}

	return node, nil
}

func (f *fakeMDProvisioner) Add(_ context.Context, device string, members ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	set, ok := f.adds[device]
	if !ok {
		set = map[string]struct{}{}
		f.adds[device] = set
	}

	for _, m := range members {
		set[m] = struct{}{}
	}

	return nil
}

func (f *fakeMDProvisioner) Grow(_ context.Context, device string, raidDevices int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.grows[device] = raidDevices

	return nil
}

func (f *fakeMDProvisioner) DetailDevice(_ context.Context, device string) (md.Detail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if d, ok := f.details[device]; ok {
		return d, nil
	}

	return md.Detail{}, md.ErrNotFound
}

func (f *fakeMDProvisioner) FindDeviceByMember(member string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if dev, ok := f.findByMember[member]; ok {
		return dev, nil
	}

	return "", md.ErrNotFound
}

func (f *fakeMDProvisioner) IsSyncing(device string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	a := f.syncAction[device]

	return a != "" && a != md.SyncActionIdle, nil
}

func (f *fakeMDProvisioner) ArrayStateForDevice(string) (string, error) {
	return "clean", nil
}

func (f *fakeMDProvisioner) SyncActionForDevice(device string) (md.SyncAction, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if a, ok := f.syncAction[device]; ok {
		return a, nil
	}

	return md.SyncActionIdle, nil
}

//nolint:unparam
func (f *fakeMDProvisioner) created(name string) ([]string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	opts, ok := f.creates[name]

	return append([]string(nil), opts.Devices...), ok
}

func (f *fakeMDProvisioner) added(device string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]string, 0, len(f.adds[device]))
	for m := range f.adds[device] {
		out = append(out, m)
	}

	sort.Strings(out)

	return out
}

func (f *fakeMDProvisioner) grown(device string) (int, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	n, ok := f.grows[device]

	return n, ok
}

func (f *fakeMDProvisioner) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.findByMember = map[string]string{}
	f.details = map[string]md.Detail{}
	f.syncAction = map[string]md.SyncAction{}
	f.createNode = "/dev/md0"
	f.createErr = nil
	f.createNodes = map[string]string{}
	f.creates = map[string]md.CreateOptions{}
	f.adds = map[string]map[string]struct{}{}
	f.grows = map[string]int{}
}

type MDArrayReconcileSuite struct {
	ctest.DefaultSuite

	md *fakeMDProvisioner
}

func (suite *MDArrayReconcileSuite) SetupTest() {
	suite.md.reset()
	suite.DefaultSuite.SetupTest()
}

//nolint:unparam
func (suite *MDArrayReconcileSuite) createArraySpec(name, match string) {
	spec := storageres.NewMDArraySpec(storageres.NamespaceName, name)
	spec.TypedSpec().Level = storageres.MDLevelRAID1
	suite.Require().NoError(spec.TypedSpec().VolumeSelector.UnmarshalText([]byte(match)))

	suite.Create(spec)
}

func (suite *MDArrayReconcileSuite) eventually(check func() bool) {
	suite.AssertWithin(2*time.Second, 50*time.Millisecond, func() error {
		if check() {
			return nil
		}

		return retry.ExpectedErrorf("md provisioner state not yet reached")
	})
}

func (suite *MDArrayReconcileSuite) TestWaitsForEnoughMembers() {
	createDisk(&suite.DefaultSuite, "nvme0n1", "/dev/nvme0n1", "nvme")

	suite.createArraySpec("data", `disk.transport == "nvme"`)

	ctest.AssertResource(suite, "data", func(status *storageres.MDArrayStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.MDArrayPhaseWaiting, status.TypedSpec().Status)
		asrt.NotEmpty(status.TypedSpec().Error)
	})

	_, created := suite.md.created("data")
	suite.Assert().False(created, "create must not run below the minimum member count")
}

func (suite *MDArrayReconcileSuite) TestCreatesArrayFromScratch() {
	createDisk(&suite.DefaultSuite, "nvme0n1", "/dev/nvme0n1", "nvme")
	createDisk(&suite.DefaultSuite, "nvme1n1", "/dev/nvme1n1", "nvme")

	suite.createArraySpec("data", `disk.transport == "nvme"`)

	suite.eventually(func() bool {
		_, created := suite.md.created("data")

		return created
	})

	members, _ := suite.md.created("data")
	suite.Assert().Equal([]string{"/dev/nvme0n1", "/dev/nvme1n1"}, members)

	ctest.AssertResource(suite, "data", func(status *storageres.MDArrayStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.MDArrayPhaseReady, status.TypedSpec().Status)
		asrt.Equal(md.DevicePath("data"), status.TypedSpec().Device)
	})
}

func (suite *MDArrayReconcileSuite) TestExtendsExistingArray() {
	suite.md.findByMember["/dev/nvme0n1"] = "/dev/md0"
	suite.md.details["/dev/md0"] = md.Detail{
		Level:       "raid1",
		RaidDevices: 2,
		Members:     []string{"/dev/nvme0n1", "/dev/nvme1n1"},
	}

	createDisk(&suite.DefaultSuite, "nvme0n1", "/dev/nvme0n1", "nvme")
	createDisk(&suite.DefaultSuite, "nvme1n1", "/dev/nvme1n1", "nvme")
	createDisk(&suite.DefaultSuite, "nvme2n1", "/dev/nvme2n1", "nvme")

	suite.createArraySpec("data", `disk.transport == "nvme"`)

	suite.eventually(func() bool {
		_, grown := suite.md.grown("/dev/md0")

		return grown
	})

	suite.Assert().Equal([]string{"/dev/nvme2n1"}, suite.md.added("/dev/md0"))

	n, _ := suite.md.grown("/dev/md0")
	suite.Assert().Equal(3, n)

	_, created := suite.md.created("data")
	suite.Assert().False(created, "must extend the existing array, not create a new one")
}

func (suite *MDArrayReconcileSuite) TestNoOpWhenObservedMatchesDesired() {
	suite.md.findByMember["/dev/nvme0n1"] = "/dev/md0"
	suite.md.details["/dev/md0"] = md.Detail{
		Level:       "raid1",
		RaidDevices: 2,
		Members:     []string{"/dev/nvme0n1", "/dev/nvme1n1"},
	}

	createDisk(&suite.DefaultSuite, "nvme0n1", "/dev/nvme0n1", "nvme")
	createDisk(&suite.DefaultSuite, "nvme1n1", "/dev/nvme1n1", "nvme")

	suite.createArraySpec("data", `disk.transport == "nvme"`)

	ctest.AssertResource(suite, "data", func(status *storageres.MDArrayStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.MDArrayPhaseReady, status.TypedSpec().Status)
		asrt.Equal(2, status.TypedSpec().RaidDevices)
	})

	suite.Assert().Empty(suite.md.added("/dev/md0"))

	_, grown := suite.md.grown("/dev/md0")
	suite.Assert().False(grown)
}

func (suite *MDArrayReconcileSuite) TestRebuildingWhileSyncing() {
	suite.md.findByMember["/dev/nvme0n1"] = "/dev/md0"
	suite.md.details["/dev/md0"] = md.Detail{
		Level:       "raid1",
		RaidDevices: 2,
		Members:     []string{"/dev/nvme0n1", "/dev/nvme1n1"},
	}
	suite.md.syncAction["/dev/md0"] = md.SyncActionResync

	createDisk(&suite.DefaultSuite, "nvme0n1", "/dev/nvme0n1", "nvme")
	createDisk(&suite.DefaultSuite, "nvme1n1", "/dev/nvme1n1", "nvme")
	createDisk(&suite.DefaultSuite, "nvme2n1", "/dev/nvme2n1", "nvme")

	suite.createArraySpec("data", `disk.transport == "nvme"`)

	ctest.AssertResource(suite, "data", func(status *storageres.MDArrayStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.MDArrayPhaseRebuilding, status.TypedSpec().Status)
	})

	suite.Assert().Empty(suite.md.added("/dev/md0"), "members must not be added while the array is syncing")
}

func (suite *MDArrayReconcileSuite) TestReportsSyncActionAsRebuilding() {
	suite.md.findByMember["/dev/nvme0n1"] = "/dev/md0"
	suite.md.details["/dev/md0"] = md.Detail{
		Level:       "raid1",
		RaidDevices: 2,
		Members:     []string{"/dev/nvme0n1", "/dev/nvme1n1"},
	}
	suite.md.syncAction["/dev/md0"] = md.SyncActionRecover

	createDisk(&suite.DefaultSuite, "nvme0n1", "/dev/nvme0n1", "nvme")
	createDisk(&suite.DefaultSuite, "nvme1n1", "/dev/nvme1n1", "nvme")

	suite.createArraySpec("data", `disk.transport == "nvme"`)

	ctest.AssertResource(suite, "data", func(status *storageres.MDArrayStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.MDArrayPhaseRebuilding, status.TypedSpec().Status)
		asrt.Equal(string(md.SyncActionRecover), status.TypedSpec().SyncAction)
	})
}

func (suite *MDArrayReconcileSuite) TestWaitsForEncryptedMemberToBeOpened() {
	// mdadm must never write a superblock onto a volume's ciphertext: the member
	// is withheld until the volume reports the device it was opened as.
	createDisk(&suite.DefaultSuite, "vdb", "/dev/vdb", "virtio")
	createRawVolumePartition(&suite.DefaultSuite, "vdb1", "/dev/vdb1", "/dev/vdb", "r-mirror0")
	createPartition(&suite.DefaultSuite, "vdb2", "/dev/vdb2", "/dev/vdb", "r-mirror1")
	createVolumeStatus(
		&suite.DefaultSuite, "r-mirror1",
		block.VolumePhaseWaiting, block.EncryptionProviderNone,
		"/dev/vdb2", "",
	)

	suite.createArraySpec("data", `volume.partition_label.startsWith("r-mirror")`)

	ctest.AssertResource(suite, "data", func(status *storageres.MDArrayStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.MDArrayPhaseWaiting, status.TypedSpec().Status)
	})

	_, created := suite.md.created("data")
	suite.Assert().False(created, "create must not run while a member is still locked")

	// Once the volume is opened, the array is built on the opened device.
	ctest.UpdateWithConflicts(suite, block.NewVolumeStatus(block.NamespaceName, "r-mirror1"), func(vs *block.VolumeStatus) error {
		vs.TypedSpec().Phase = block.VolumePhaseReady
		vs.TypedSpec().EncryptionProvider = block.EncryptionProviderLUKS2
		vs.TypedSpec().MountLocation = "/dev/dm-0"

		return nil
	})

	suite.eventually(func() bool {
		_, created := suite.md.created("data")

		return created
	})

	members, _ := suite.md.created("data")
	suite.Assert().Equal([]string{"/dev/dm-0", "/dev/vdb1"}, members)
}

const (
	bootSerialSelector = `disk.serial == "BOOT-A" || disk.serial == "BOOT-B"`
	dataSerialSelector = `disk.serial == "DATA-A" || disk.serial == "DATA-B"`
	// Use nonexistent device names so even a privileged test run cannot trigger
	// a change event on an actual host MD array.
	bootTestDevice = "/dev/md-test-boot"
	dataTestDevice = "/dev/md-test-data"
)

type serialDiskFixture struct {
	device string
	serial string
}

func (suite *MDArrayReconcileSuite) createSerialDisks(disks ...serialDiskFixture) {
	for _, disk := range disks {
		createDiskWithSerial(&suite.DefaultSuite, filepath.Base(disk.device), disk.device, "virtio", disk.serial)
	}
}

func (suite *MDArrayReconcileSuite) createSerialMirrorSpecs() {
	for _, array := range []struct {
		name     string
		selector string
		metadata storageres.MDMetadata
	}{
		{"boot", bootSerialSelector, storageres.MDMetadata10},
		{"data", dataSerialSelector, storageres.MDMetadata12},
	} {
		spec := storageres.NewMDArraySpec(storageres.NamespaceName, array.name)
		spec.TypedSpec().Level = storageres.MDLevelRAID1
		spec.TypedSpec().Metadata = array.metadata
		suite.Require().NoError(spec.TypedSpec().VolumeSelector.UnmarshalText([]byte(array.selector)))
		suite.Create(spec)
	}
}

// observeSerialMirror changes only the simulated mdadm/sysfs observation. The
// production controller still decides whether to create, add, or grow an array.
func (suite *MDArrayReconcileSuite) observeSerialMirror(name, device string, members []string, action md.SyncAction) {
	suite.md.mu.Lock()
	defer suite.md.mu.Unlock()

	metadata := "1.0"
	if name == "data" {
		metadata = "1.2"
	}

	suite.md.details[device] = md.Detail{
		Level:       "raid1",
		RaidDevices: 2,
		Members:     append([]string(nil), members...),
		Name:        "talos:" + name,
		UUID:        "test-" + name,
		Metadata:    metadata,
	}
	suite.md.syncAction[device] = action

	for member, arrayDevice := range suite.md.findByMember {
		if arrayDevice == device {
			delete(suite.md.findByMember, member)
		}
	}

	for _, member := range members {
		suite.md.findByMember[member] = device
	}
}

func (suite *MDArrayReconcileSuite) assertSerialMirror(name string, members []string, phase storageres.MDArrayPhase, action md.SyncAction) {
	ctest.AssertResource(suite, name, func(status *storageres.MDArrayStatus, asrt *assert.Assertions) {
		spec := status.TypedSpec()
		asrt.Equal(storageres.MDLevelRAID1, spec.Level)
		asrt.Equal(phase, spec.Status)
		asrt.ElementsMatch(members, spec.Members)
		asrt.Equal(2, spec.RaidDevices)
		asrt.Equal(md.DevicePath(name), spec.Device)
		asrt.Equal("talos:"+name, spec.Name)
		asrt.Equal("test-"+name, spec.UUID)
		asrt.Equal(string(action), spec.SyncAction)
		asrt.Empty(spec.Error)

		if name == "boot" {
			asrt.Equal("1.0", spec.Metadata)
		} else {
			asrt.Equal("1.2", spec.Metadata)
		}
	})
}

// assertNoSerialMirrorMutations checks every array, not just the expected name,
// so an unintended mutation of the other mirror or a decoy cannot pass.
func (suite *MDArrayReconcileSuite) assertNoSerialMirrorMutations() {
	suite.md.mu.Lock()
	defer suite.md.mu.Unlock()

	suite.Assert().Empty(suite.md.creates)
	suite.Assert().Empty(suite.md.adds)
	suite.Assert().Empty(suite.md.grows)
}

func (suite *MDArrayReconcileSuite) TestCreatesIndependentMirrorsByExactSerial() {
	suite.md.createNodes["boot"] = bootTestDevice
	suite.md.createNodes["data"] = dataTestDevice

	// All disks have the same transport. Discovery order, device order, serial
	// prefixes, and an absent serial must not broaden either exact selector.
	suite.createSerialDisks(
		serialDiskFixture{"/dev/vdf", "DATA-A"},
		serialDiskFixture{"/dev/vdb", "BOOT-B"},
		serialDiskFixture{"/dev/vdg", ""},
		serialDiskFixture{"/dev/vda", "BOOT-A-extra"},
		serialDiskFixture{"/dev/vdd", "BOOT-A"},
		serialDiskFixture{"/dev/vde", "data-a"},
		serialDiskFixture{"/dev/vdc", "DATA-B"},
	)
	suite.createSerialMirrorSpecs()

	suite.assertSerialMirror("boot", []string{"/dev/vdb", "/dev/vdd"}, storageres.MDArrayPhaseReady, md.SyncActionIdle)
	suite.assertSerialMirror("data", []string{"/dev/vdc", "/dev/vdf"}, storageres.MDArrayPhaseReady, md.SyncActionIdle)

	suite.md.mu.Lock()
	defer suite.md.mu.Unlock()

	suite.Assert().Equal(map[string]md.CreateOptions{
		"boot": {Level: 1, Metadata: "1.0", RaidDevices: 2, Devices: []string{"/dev/vdb", "/dev/vdd"}},
		"data": {Level: 1, Metadata: "1.2", RaidDevices: 2, Devices: []string{"/dev/vdc", "/dev/vdf"}},
	}, suite.md.creates)
	suite.Assert().Empty(suite.md.adds)
	suite.Assert().Empty(suite.md.grows)
}

func (suite *MDArrayReconcileSuite) TestSerialMirrorsIgnoreRenamedReorderedDevices() {
	// Model rediscovery after a cold boot: serials and MD identities persist,
	// but member paths and discovery order differ from the initial-create test.
	// This controller test does not boot a VM or simulate firmware behavior.
	suite.observeSerialMirror("boot", bootTestDevice, []string{"/dev/vdh", "/dev/vdc"}, md.SyncActionIdle)
	suite.observeSerialMirror("data", dataTestDevice, []string{"/dev/vdg", "/dev/vdb"}, md.SyncActionIdle)
	suite.createSerialDisks(
		serialDiskFixture{"/dev/vdh", "BOOT-B"},
		serialDiskFixture{"/dev/vdg", "DATA-B"},
		serialDiskFixture{"/dev/vdf", ""},
		serialDiskFixture{"/dev/vde", "DATA-A-extra"},
		serialDiskFixture{"/dev/vdc", "BOOT-A"},
		serialDiskFixture{"/dev/vdb", "DATA-A"},
		serialDiskFixture{"/dev/vda", "boot-a"},
	)
	suite.createSerialMirrorSpecs()

	suite.assertSerialMirror("boot", []string{"/dev/vdc", "/dev/vdh"}, storageres.MDArrayPhaseReady, md.SyncActionIdle)
	suite.assertSerialMirror("data", []string{"/dev/vdb", "/dev/vdg"}, storageres.MDArrayPhaseReady, md.SyncActionIdle)
	suite.assertNoSerialMirrorMutations()
}

func (suite *MDArrayReconcileSuite) assertIncompleteBootSerialSelection(secondSerial string, absent, existing bool) {
	// A healthy data mirror and prefix-matching decoy must never satisfy the
	// incomplete system selector, even when a system array already exists.
	suite.observeSerialMirror("data", dataTestDevice, []string{"/dev/vdc", "/dev/vdd"}, md.SyncActionIdle)

	if existing {
		suite.observeSerialMirror("boot", bootTestDevice, []string{"/dev/vda"}, md.SyncActionIdle)
	}

	suite.createSerialDisks(
		serialDiskFixture{"/dev/vda", "BOOT-A"},
		serialDiskFixture{"/dev/vdc", "DATA-A"},
		serialDiskFixture{"/dev/vdd", "DATA-B"},
		serialDiskFixture{"/dev/vde", "BOOT-B-extra"},
	)

	if !absent {
		suite.createSerialDisks(serialDiskFixture{"/dev/vdb", secondSerial})
	}

	suite.createSerialMirrorSpecs()

	ctest.AssertResource(suite, "boot", func(status *storageres.MDArrayStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.MDArrayPhaseWaiting, status.TypedSpec().Status)
		asrt.Equal([]string{"/dev/vda"}, status.TypedSpec().Members)
		asrt.Equal("waiting for enough member disks: matched 1, required 2", status.TypedSpec().Error)
	})
	suite.assertSerialMirror("data", []string{"/dev/vdc", "/dev/vdd"}, storageres.MDArrayPhaseReady, md.SyncActionIdle)
	suite.assertNoSerialMirrorMutations()
}

func (suite *MDArrayReconcileSuite) TestWrongSerialDoesNotCreate() {
	suite.assertIncompleteBootSerialSelection("WRONG", false, false)
}

func (suite *MDArrayReconcileSuite) TestMissingSerialDoesNotCreate() {
	suite.assertIncompleteBootSerialSelection("", false, false)
}

func (suite *MDArrayReconcileSuite) TestAbsentSerialMemberDoesNotCreate() {
	suite.assertIncompleteBootSerialSelection("", true, false)
}

func (suite *MDArrayReconcileSuite) TestWrongSerialDoesNotGrowExistingArray() {
	suite.assertIncompleteBootSerialSelection("WRONG", false, true)
}

func (suite *MDArrayReconcileSuite) TestMissingSerialDoesNotGrowExistingArray() {
	suite.assertIncompleteBootSerialSelection("", false, true)
}

func (suite *MDArrayReconcileSuite) TestAbsentSerialMemberDoesNotGrowExistingArray() {
	suite.assertIncompleteBootSerialSelection("", true, true)
}

func (suite *MDArrayReconcileSuite) TestTwoMemberSerialMirrorRejoinsWithoutGrowing() {
	suite.observeSerialMirror("boot", bootTestDevice, []string{"/dev/vda"}, md.SyncActionIdle)
	suite.observeSerialMirror("data", dataTestDevice, []string{"/dev/vdc", "/dev/vdd"}, md.SyncActionIdle)
	suite.createSerialDisks(
		serialDiskFixture{"/dev/vda", "BOOT-A"},
		serialDiskFixture{"/dev/vdb", "BOOT-B-extra"},
		serialDiskFixture{"/dev/vdc", "DATA-A"},
		serialDiskFixture{"/dev/vdd", "DATA-B"},
	)
	suite.createSerialMirrorSpecs()

	ctest.AssertResource(suite, "boot", func(status *storageres.MDArrayStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.MDArrayPhaseWaiting, status.TypedSpec().Status)
		asrt.Equal([]string{"/dev/vda"}, status.TypedSpec().Members)
	})
	suite.assertSerialMirror("data", []string{"/dev/vdc", "/dev/vdd"}, storageres.MDArrayPhaseReady, md.SyncActionIdle)
	suite.assertNoSerialMirrorMutations()

	// The returning member has a different device name; the old pathname now
	// belongs to a decoy. Only its exact serial may authorize the add operation.
	suite.createSerialDisks(serialDiskFixture{"/dev/vdf", "BOOT-B"})
	suite.eventually(func() bool {
		return len(suite.md.added(bootTestDevice)) > 0
	})

	// These are explicit fixture observations, not evidence of real resync.
	members := []string{"/dev/vda", "/dev/vdf"}
	suite.observeSerialMirror("boot", bootTestDevice, members, md.SyncActionRecover)

	refresh := storageres.NewMDRefreshRequest(storageres.NamespaceName, storageres.RefreshID)
	refresh.TypedSpec().Request = 1
	suite.Create(refresh)
	suite.assertSerialMirror("boot", members, storageres.MDArrayPhaseRebuilding, md.SyncActionRecover)

	suite.observeSerialMirror("boot", bootTestDevice, members, md.SyncActionIdle)
	ctest.UpdateWithConflicts(suite, refresh, func(request *storageres.MDRefreshRequest) error {
		request.TypedSpec().Request++

		return nil
	})
	suite.assertSerialMirror("boot", members, storageres.MDArrayPhaseReady, md.SyncActionIdle)
	suite.assertSerialMirror("data", []string{"/dev/vdc", "/dev/vdd"}, storageres.MDArrayPhaseReady, md.SyncActionIdle)

	suite.md.mu.Lock()
	defer suite.md.mu.Unlock()

	suite.Assert().Empty(suite.md.creates, "rejoin must retain the original arrays")
	suite.Assert().Equal(map[string]map[string]struct{}{
		bootTestDevice: {"/dev/vdf": {}},
	}, suite.md.adds)
	suite.Assert().Empty(suite.md.grows, "rejoining a missing slot must retain two configured RAID devices")
}

func TestMDArrayReconcileSuite(t *testing.T) {
	t.Parallel()

	provisioner := newFakeMDProvisioner()

	s := &MDArrayReconcileSuite{md: provisioner}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 5 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.MDArrayReconcileController{
				State: suite.State(),
				MD:    provisioner,
			}))
		},
	}

	suite.Run(t, s)
}
