// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

const (
	poolMachineUUID = "c737f778-82a1-48dd-990b-67901031bcc5"
	poolFinalizer   = "storage.StoragePoolController"
)

// poolClient records libvirt operations; every operation on an active pool
// verifies the controller holds the backing mount.
type poolClient struct {
	pools     []libvirtstorage.Pool
	events    []string
	active    map[string]struct{}
	checkHold func(string) error
	changed   chan struct{}
	openErr   error
	removeErr error
	ensureErr error
	mu        sync.Mutex
	completed int
}

func (c *poolClient) find(name string) (libvirtstorage.Pool, bool) {
	i := slices.IndexFunc(c.pools, func(existing libvirtstorage.Pool) bool { return existing.Name == name })
	if i < 0 {
		return libvirtstorage.Pool{}, false
	}

	return c.pools[i], true
}

// stop deactivates a pool, checking the hold on its recorded target.
func (c *poolClient) stop(name string) error {
	if _, running := c.active[name]; !running {
		return nil
	}

	if existing, ok := c.find(name); ok {
		if err := c.checkHold(existing.Target); err != nil {
			return err
		}
	}

	c.events = append(c.events, "stop:"+name)
	delete(c.active, name)

	return nil
}

func (c *poolClient) open(context.Context) (libvirtstorage.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c, c.openErr
}

func (c *poolClient) Pools() ([]libvirtstorage.Pool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.pools), nil
}

func (c *poolClient) Ensure(p libvirtstorage.Pool, target string, prepare func() error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.checkHold(target); err != nil {
		return err
	}

	if c.ensureErr != nil {
		return c.ensureErr
	}

	if err := prepare(); err != nil {
		return err
	}

	c.events = append(c.events, "ensure:"+target)

	p.Target = target

	if i := slices.IndexFunc(c.pools, func(existing libvirtstorage.Pool) bool { return existing.Name == p.Name }); i >= 0 {
		c.pools[i] = p
	} else {
		c.pools = append(c.pools, p)
	}

	if c.active == nil {
		c.active = map[string]struct{}{}
	}

	c.active[p.Name] = struct{}{}

	return nil
}

func (c *poolClient) Remove(p libvirtstorage.Pool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !slices.ContainsFunc(c.pools, func(existing libvirtstorage.Pool) bool { return existing.Name == p.Name }) {
		return nil
	}

	c.events = append(c.events, "remove:"+p.Name)

	if c.removeErr != nil {
		return c.removeErr
	}

	if _, running := c.active[p.Name]; running {
		existing, _ := c.find(p.Name)
		if err := c.checkHold(existing.Target); err != nil {
			return err
		}
	}

	c.pools = slices.DeleteFunc(c.pools, func(existing libvirtstorage.Pool) bool { return existing.Name == p.Name })
	delete(c.active, p.Name)

	return nil
}

func (c *poolClient) Stop(p libvirtstorage.Pool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.stop(p.Name)
}

func (c *poolClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.completed = len(c.events)

	select {
	case c.changed <- struct{}{}:
	default:
	}
}

func (c *poolClient) completedEvents() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.events[:c.completed])
}

func (c *poolClient) setEnsureError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ensureErr = err
}

func (c *poolClient) setErrors(openErr, removeErr error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.openErr, c.removeErr = openErr, removeErr
}

type StoragePoolSuite struct {
	ctest.DefaultSuite

	client *poolClient
	spec   *storageres.StoragePoolSpec
	root   string
}

func (suite *StoragePoolSuite) SetupTest() {
	suite.DefaultSuite.SetupTest()
	suite.root = suite.T().TempDir()
	suite.client = &poolClient{changed: make(chan struct{}, 1), checkHold: suite.checkHold}

	system := hardware.NewSystemInformation(hardware.SystemInformationID)
	system.TypedSpec().UUID = poolMachineUUID
	suite.Create(system)

	suite.spec = storageres.NewStoragePoolSpec(storageres.NamespaceName, "images")
	suite.spec.TypedSpec().VolumeID = "u-vms"
	suite.Create(suite.spec)
	suite.volume("u-vms")
}

func (suite *StoragePoolSuite) setVolumeID(volumeID string) {
	ctest.UpdateWithConflicts(suite, suite.spec, func(spec *storageres.StoragePoolSpec) error {
		spec.TypedSpec().VolumeID = volumeID

		return nil
	})
}

func (suite *StoragePoolSuite) start() {
	suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.StoragePoolController{Open: suite.client.open}))
}

func (suite *StoragePoolSuite) volume(id string) {
	suite.Require().NoError(os.Mkdir(filepath.Join(suite.root, id), 0o755))
}

// mount models the volume controllers' own mount: a request owned by
// VolumeConfigController and a status owned by MountStatusController.
func (suite *StoragePoolSuite) mount(volume string) *block.VolumeMountStatus {
	request := block.NewVolumeMountRequest(block.NamespaceName, volume)
	request.TypedSpec().VolumeID = volume
	request.TypedSpec().Requester = "block.VolumeConfigController"
	suite.Create(request, state.WithCreateOwner("block.VolumeConfigController"))

	mount := block.NewVolumeMountStatus(block.NamespaceName, volume)
	mount.TypedSpec().VolumeID = volume
	mount.TypedSpec().Target = filepath.Join(suite.root, volume)
	suite.Create(mount, state.WithCreateOwner("block.MountStatusController"))

	return mount
}

func (suite *StoragePoolSuite) checkHold(target string) error {
	volume := filepath.Base(filepath.Dir(target))

	mount, err := safe.StateGetByID[*block.VolumeMountStatus](suite.Ctx(), suite.State(), volume)
	if err != nil {
		return err
	}

	if !mount.Metadata().Finalizers().Has(poolFinalizer) {
		return fmt.Errorf("pool operation on %q without holding its backing mount", target)
	}

	return nil
}

func (suite *StoragePoolSuite) assertHold(volume string, held bool) {
	ctest.AssertResource(suite, volume, func(mount *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.Equal(held, mount.Metadata().Finalizers().Has(poolFinalizer))
	})
}

func (suite *StoragePoolSuite) assertReady(volume string) {
	ctest.AssertResource(suite, "images", func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Ready, status.TypedSpec().Error)
		asrt.Empty(status.TypedSpec().Error)
		asrt.Equal(volume, status.TypedSpec().VolumeID)
		asrt.Equal(filepath.Join(suite.root, volume, "images"), status.TypedSpec().TargetPath)
	})
}

func (suite *StoragePoolSuite) assertError(reason string) {
	ctest.AssertResource(suite, "images", func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Ready)
		asrt.Contains(status.TypedSpec().Error, reason)
	})
}

func (suite *StoragePoolSuite) assertNoRequests() {
	requests, err := safe.StateListAll[*block.VolumeMountRequest](suite.Ctx(), suite.State())
	suite.Require().NoError(err)

	for request := range requests.All() {
		suite.Require().Equal("block.VolumeConfigController", request.Metadata().Owner(), "the pool controller must not create mount requests")
		suite.Require().Empty(*request.Metadata().Finalizers())
	}
}

func (suite *StoragePoolSuite) waitForEvent(event string) {
	for !slices.Contains(suite.client.completedEvents(), event) {
		select {
		case <-suite.Ctx().Done():
			suite.Require().FailNow("controller did not complete expected pool operation", "%s: %v", event, suite.client.completedEvents())
		case <-suite.client.changed:
		}
	}
}

func (suite *StoragePoolSuite) activate() {
	suite.mount("u-vms")
	suite.start()
	suite.assertReady("u-vms")
	suite.assertHold("u-vms", true)
}

func (suite *StoragePoolSuite) TestWaitsForExistingMount() {
	suite.start()
	suite.assertError("waiting for backing volume")
	suite.assertNoRequests()
	suite.Require().NoDirExists(filepath.Join(suite.root, "u-vms", "images"))
	suite.Require().Empty(suite.client.completedEvents())

	suite.mount("u-vms")
	suite.assertReady("u-vms")
	suite.assertHold("u-vms", true)
	suite.assertNoRequests()
}

func (suite *StoragePoolSuite) TestMissingDaemonIsNotReady() {
	suite.mount("u-vms")
	suite.client.setErrors(os.ErrNotExist, nil)
	suite.start()
	suite.assertError("waiting for storage daemon")
	suite.assertHold("u-vms", false)
	suite.Require().NoDirExists(filepath.Join(suite.root, "u-vms", "images"))

	suite.client.setErrors(nil, nil)
	suite.assertReady("u-vms")
}

func (suite *StoragePoolSuite) TestRemovalPreservesData() {
	suite.activate()
	data := filepath.Join(suite.root, "u-vms", "images", "vm.qcow2")
	suite.Require().NoError(os.WriteFile(data, []byte("valuable"), 0o600))
	suite.Destroy(suite.spec)

	suite.waitForEvent("remove:images")
	ctest.AssertNoResource[*storageres.StoragePoolStatus](suite, "images")
	suite.assertHold("u-vms", false)
	suite.assertNoRequests()

	contents, err := os.ReadFile(data)
	suite.Require().NoError(err)
	suite.Require().Equal("valuable", string(contents))
}

func (suite *StoragePoolSuite) TestFailedRemovalKeepsHold() {
	suite.activate()
	suite.client.setErrors(nil, errors.New("daemon disconnected"))
	suite.Destroy(suite.spec)

	suite.waitForEvent("remove:images")
	suite.assertHold("u-vms", true)

	suite.client.setErrors(nil, nil)
	suite.assertHold("u-vms", false)
	ctest.AssertNoResource[*storageres.StoragePoolStatus](suite, "images")
}

func (suite *StoragePoolSuite) TestRetargetStopsBeforeRelease() {
	suite.activate()
	suite.volume("x-new")
	suite.setVolumeID("x-new")

	suite.waitForEvent("stop:images")
	suite.assertHold("u-vms", false)
	suite.assertError("waiting for backing volume")

	suite.mount("x-new")
	suite.assertReady("x-new")
	suite.assertHold("x-new", true)
	suite.Require().NotContains(suite.client.completedEvents(), "remove:images")
}

func (suite *StoragePoolSuite) TestSpecTeardownRemovesPool() {
	suite.activate()
	_, err := suite.State().Teardown(suite.Ctx(), suite.spec.Metadata())
	suite.Require().NoError(err)

	suite.waitForEvent("remove:images")
	ctest.AssertNoResource[*storageres.StoragePoolStatus](suite, "images")
	suite.assertHold("u-vms", false)
	suite.Destroy(suite.spec)
}

// Tearing down one volume must not disturb a pool on an unrelated volume.
func (suite *StoragePoolSuite) TestMountTeardownIsolatedPerVolume() {
	suite.volume("u-other")

	other := storageres.NewStoragePoolSpec(storageres.NamespaceName, "backups")
	other.TypedSpec().VolumeID = "u-other"
	suite.Create(other)

	images := suite.mount("u-vms")
	suite.mount("u-other")
	suite.start()
	suite.assertReady("u-vms")
	ctest.AssertResource(suite, "backups", func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Ready, status.TypedSpec().Error)
	})

	_, err := suite.State().Teardown(suite.Ctx(), images.Metadata(), state.WithTeardownOwner("block.MountStatusController"))
	suite.Require().NoError(err)

	suite.waitForEvent("stop:images")
	suite.assertHold("u-vms", false)
	suite.assertHold("u-other", true)
	suite.Require().NotContains(suite.client.completedEvents(), "stop:backups", "an unrelated pool must not be stopped")
}

func (suite *StoragePoolSuite) TestMountTeardownStopsPool() {
	mount := suite.mount("u-vms")
	suite.start()
	suite.assertReady("u-vms")
	suite.AddFinalizer(mount.Metadata(), "another-consumer")

	_, err := suite.State().Teardown(suite.Ctx(), mount.Metadata(), state.WithTeardownOwner("block.MountStatusController"))
	suite.Require().NoError(err)

	suite.waitForEvent("stop:images")
	suite.assertHold("u-vms", false)
	suite.assertError("not mounted for writing")
	suite.Require().NotContains(suite.client.completedEvents(), "remove:images")

	ctest.AssertResource(suite, "u-vms", func(mount *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(mount.Metadata().Finalizers().Has("another-consumer"), "other consumers' holds must stay")
	})
}

// On shutdown virtstoraged stops before volumes are finalized. The hold must
// still be released or volume teardown blocks until its deadline.
func (suite *StoragePoolSuite) TestMountTeardownWithoutDaemonReleasesHold() {
	mount := suite.mount("u-vms")
	suite.start()
	suite.assertReady("u-vms")

	suite.client.setErrors(os.ErrNotExist, nil)
	_, err := suite.State().Teardown(suite.Ctx(), mount.Metadata(), state.WithTeardownOwner("block.MountStatusController"))
	suite.Require().NoError(err)

	suite.assertHold("u-vms", false)
	suite.Require().NotContains(suite.client.completedEvents(), "remove:images", "a dead daemon cannot undefine; the definition must survive reboot")
}

func (suite *StoragePoolSuite) TestTearingDownMountIsNotUsed() {
	mount := suite.mount("u-vms")
	_, err := suite.State().Teardown(suite.Ctx(), mount.Metadata(), state.WithTeardownOwner("block.MountStatusController"))
	suite.Require().NoError(err)
	suite.start()

	suite.assertError("not mounted for writing")
	suite.assertHold("u-vms", false)
	suite.Require().Empty(suite.client.completedEvents())
}

// A mount that turns read-only under an active pool must stop it and release
// the hold, not keep libvirt writing through a hold we should not keep.
func (suite *StoragePoolSuite) TestMountTurningReadOnlyStopsPool() {
	suite.activate()

	ctest.UpdateWithConflicts(suite, block.NewVolumeMountStatus(block.NamespaceName, "u-vms"), func(mount *block.VolumeMountStatus) error {
		mount.TypedSpec().ReadOnly = true

		return nil
	}, state.WithUpdateOwner("block.MountStatusController"))

	suite.waitForEvent("stop:images")
	suite.assertHold("u-vms", false)
	suite.assertError("not mounted for writing")
}

func (suite *StoragePoolSuite) TestReadOnlyMountIsNotUsed() {
	mount := suite.mount("u-vms")
	ctest.UpdateWithConflicts(suite, mount, func(mount *block.VolumeMountStatus) error {
		mount.TypedSpec().ReadOnly = true

		return nil
	}, state.WithUpdateOwner("block.MountStatusController"))
	suite.start()

	suite.assertError("not mounted for writing")
	suite.assertHold("u-vms", false)
}

func (suite *StoragePoolSuite) TestRejectsTargetSymlink() {
	suite.mount("u-vms")
	suite.Require().NoError(os.Symlink(suite.T().TempDir(), filepath.Join(suite.root, "u-vms", "images")))
	suite.start()

	suite.assertError("not a directory")
	suite.Require().Empty(suite.client.completedEvents())
}

func (suite *StoragePoolSuite) TestStaleDiscoveryDoesNotTouchForeignPools() {
	suite.Destroy(suite.spec)

	stale := libvirtstorage.Pool{Name: "stale", UUID: libvirtstorage.UUID(uuid.MustParse(poolMachineUUID), "stale")}
	foreign := libvirtstorage.Pool{Name: "foreign", UUID: uuid.New()}
	suite.client.pools = []libvirtstorage.Pool{stale, foreign}
	data := filepath.Join(suite.root, "u-vms", "stale", "vm.qcow2")
	suite.Require().NoError(os.Mkdir(filepath.Dir(data), 0o755))
	suite.Require().NoError(os.WriteFile(data, []byte("valuable"), 0o600))
	suite.start()

	suite.waitForEvent("remove:stale")
	suite.Require().Equal([]string{"remove:stale"}, suite.client.completedEvents())

	pools, err := suite.client.Pools()
	suite.Require().NoError(err)
	suite.Require().Equal([]libvirtstorage.Pool{foreign}, pools)

	contents, err := os.ReadFile(data)
	suite.Require().NoError(err)
	suite.Require().Equal("valuable", string(contents))
}

func (suite *StoragePoolSuite) TestInvalidMachineUUID() {
	ctest.UpdateWithConflicts(suite, hardware.NewSystemInformation(hardware.SystemInformationID), func(system *hardware.SystemInformation) error {
		system.TypedSpec().UUID = uuid.Nil.String()

		return nil
	})
	suite.mount("u-vms")
	suite.start()

	ctest.AssertNoResource[*storageres.StoragePoolStatus](suite, "images")
	suite.assertHold("u-vms", false)
	suite.Require().Empty(suite.client.completedEvents(), "invalid machine identity must not claim libvirt pools")
}

func TestStoragePoolRetryWithoutPolling(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		fixture := &StoragePoolSuite{Timeout: time.Minute}
		fixture.SetT(t)
		fixture.SetupTest()

		defer fixture.TearDownTest()

		fixture.mount("u-vms")
		fixture.client.setErrors(os.ErrNotExist, nil)
		fixture.client.setEnsureError(errors.New("ensure failed"))
		fixture.start()
		fixture.assertError("waiting for storage daemon")
		synctest.Wait()

		// Daemon recovery emits no resource event: only backoff can pick it up.
		fixture.client.setErrors(nil, nil)
		fixture.assertError("ensure failed")
		synctest.Wait()
		fixture.client.setEnsureError(nil)
		fixture.assertReady("u-vms")
		synctest.Wait()

		events := fixture.client.completedEvents()

		synctest.Sleep(10 * time.Second)
		fixture.Require().Equal(events, fixture.client.completedEvents(), "a healthy pool must not be reconciled by a periodic timer")

		// Removal must retry through backoff after the last spec is gone.
		fixture.client.setErrors(nil, errors.New("daemon disconnected"))
		fixture.Destroy(fixture.spec)
		fixture.waitForEvent("remove:images")
		synctest.Wait()
		fixture.assertHold("u-vms", true)

		fixture.client.setErrors(nil, nil)
		ctest.AssertNoResource[*storageres.StoragePoolStatus](fixture, "images")
		fixture.assertHold("u-vms", false)
	})
}

func TestStoragePoolRetargetOrdering(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		fixture := &StoragePoolSuite{Timeout: 10 * time.Second}
		fixture.SetT(t)
		fixture.SetupTest()

		defer fixture.TearDownTest()

		fixture.volume("x-data")
		fixture.mount("u-vms")
		fixture.mount("x-data")
		fixture.start()
		fixture.assertReady("u-vms")
		fixture.assertHold("u-vms", true)
		fixture.assertHold("x-data", false)

		synctest.Wait()
		fixture.setVolumeID("x-data")
		synctest.Wait()

		status, err := safe.StateGetByID[*storageres.StoragePoolStatus](fixture.Ctx(), fixture.State(), "images")
		fixture.Require().NoError(err)
		fixture.Require().True(status.TypedSpec().Ready, status.TypedSpec().Error)
		fixture.Require().Equal(filepath.Join(fixture.root, "x-data", "images"), status.TypedSpec().TargetPath)

		fixture.assertHold("u-vms", false)
		fixture.assertHold("x-data", true)

		events := fixture.client.completedEvents()
		oldEnsure := slices.Index(events, "ensure:"+filepath.Join(fixture.root, "u-vms", "images"))
		stop := slices.Index(events, "stop:images")
		newEnsure := slices.Index(events, "ensure:"+filepath.Join(fixture.root, "x-data", "images"))
		fixture.Require().GreaterOrEqual(oldEnsure, 0)
		fixture.Require().Greater(stop, oldEnsure, "the pool must stop before its old hold is released")
		fixture.Require().Greater(newEnsure, stop)
		fixture.Require().NotContains(events, "remove:images", "retargeting keeps the persistent definition")
	})
}

func TestStoragePoolSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &StoragePoolSuite{
		Timeout: 10 * time.Second,
	})
}
