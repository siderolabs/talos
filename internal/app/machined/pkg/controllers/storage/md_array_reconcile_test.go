// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
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

	creates map[string][]string            // name -> members
	adds    map[string]map[string]struct{} // device -> added members
	grows   map[string]int                 // device -> target raid devices
}

func newFakeMDProvisioner() *fakeMDProvisioner {
	return &fakeMDProvisioner{
		findByMember: map[string]string{},
		details:      map[string]md.Detail{},
		syncAction:   map[string]md.SyncAction{},
		createNode:   "/dev/md0",
		creates:      map[string][]string{},
		adds:         map[string]map[string]struct{}{},
		grows:        map[string]int{},
	}
}

func (f *fakeMDProvisioner) Create(_ context.Context, name string, opts md.CreateOptions) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.creates[name]; !ok {
		f.creates[name] = append([]string(nil), opts.Devices...)
	}

	if f.createErr != nil {
		return f.createNode, f.createErr
	}

	if _, ok := f.details[f.createNode]; !ok {
		f.details[f.createNode] = md.Detail{
			Level:       "raid1",
			RaidDevices: len(opts.Devices),
			Members:     append([]string(nil), opts.Devices...),
		}
	}

	return f.createNode, nil
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

	members, ok := f.creates[name]

	return append([]string(nil), members...), ok
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
	f.creates = map[string][]string{}
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
