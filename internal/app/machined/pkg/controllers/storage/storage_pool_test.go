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

	"github.com/cosi-project/runtime/pkg/resource"
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
	pools         []libvirtstorage.Pool
	events        []string
	active        map[string]struct{}
	checkHold     func(string) error
	changed       chan struct{}
	openErr       error
	poolsErr      error
	removeErr     error
	ensureErr     error
	stopErr       error
	ensureOutcome libvirtstorage.OperationOutcome
	removeOutcome libvirtstorage.OperationOutcome
	stopOutcome   libvirtstorage.OperationOutcome
	mu            sync.Mutex
	completed     int
	beforeEnsure  func(string)
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

	return slices.Clone(c.pools), c.poolsErr
}

func (c *poolClient) Ensure(p libvirtstorage.Pool, target string, prepare func() error) error {
	c.mu.Lock()
	beforeEnsure := c.beforeEnsure
	c.mu.Unlock()

	if beforeEnsure != nil {
		beforeEnsure(target)
	}

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
	p.Type = "dir"
	p.Active = true
	p.Persistent = true
	p.Autostart = false

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

func (c *poolClient) EnsureOperation(p libvirtstorage.Pool, target string, prepare func() error) (libvirtstorage.OperationOutcome, error) {
	err := c.Ensure(p, target, prepare)
	if err != nil {
		return c.ensureOutcome, err
	}

	return libvirtstorage.Finished, nil
}

func (c *poolClient) RemoveOperation(p libvirtstorage.Pool) (libvirtstorage.OperationOutcome, error) {
	err := c.Remove(p)
	if err != nil {
		return c.removeOutcome, err
	}

	return libvirtstorage.Finished, nil
}

func (c *poolClient) StopOperation(p libvirtstorage.Pool) (libvirtstorage.OperationOutcome, error) {
	c.mu.Lock()
	if c.stopErr != nil {
		c.events = append(c.events, "stop:"+p.Name)
		err, outcome := c.stopErr, c.stopOutcome
		c.mu.Unlock()

		return outcome, err
	}
	c.mu.Unlock()

	err := c.Stop(p)
	if err != nil {
		return libvirtstorage.NoMutationSubmitted, err
	}

	return libvirtstorage.Finished, nil
}

func (*poolClient) ResizeVolumeOperation(libvirtstorage.Pool, string, uint64) (libvirtstorage.OperationOutcome, error) {
	panic("StoragePoolController must not resize volumes")
}

// StoragePoolController reconciles pool definitions and nothing within them. A volume call here
// would mean it had grown a second responsibility, and with it a second writer of the same pool.
func (*poolClient) Volume(libvirtstorage.Pool, string) (libvirtstorage.Volume, bool, error) {
	panic("StoragePoolController must not touch volumes")
}

func (*poolClient) CreateVolume(libvirtstorage.Pool, string, string, uint64, string, string) (libvirtstorage.Volume, error) {
	panic("StoragePoolController must not touch volumes")
}

func (*poolClient) ResizeVolume(libvirtstorage.Pool, string, uint64) error {
	panic("StoragePoolController must not touch volumes")
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

func (c *poolClient) setPoolsError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.poolsErr = err
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
		asrt.True(status.TypedSpec().Phase == storageres.StoragePoolPhaseReady, status.TypedSpec().Error)
		asrt.Empty(status.TypedSpec().Error)
		asrt.Equal(volume, status.TypedSpec().VolumeID)
		asrt.Equal(filepath.Join(suite.root, volume, "images"), status.TypedSpec().TargetPath)
	})
}

func (suite *StoragePoolSuite) assertError(reason string) {
	ctest.AssertResource(suite, "images", func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Phase == storageres.StoragePoolPhaseReady)
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

func (suite *StoragePoolSuite) TestInactiveMatchingPoolActivatesWithoutStop() {
	suite.activate()
	suite.client.mu.Lock()
	pool := suite.client.pools[0]
	pool.Active = false
	suite.client.pools[0] = pool
	delete(suite.client.active, pool.Name)
	suite.client.mu.Unlock()
	suite.volume("x-wakeup")
	suite.mount("x-wakeup")
	suite.waitForEvent("ensure:" + filepath.Join(suite.root, "u-vms", "images"))
	suite.Require().Eventually(func() bool {
		suite.client.mu.Lock()
		defer suite.client.mu.Unlock()

		return suite.client.pools[0].Active
	}, time.Second, 10*time.Millisecond)
	suite.Require().NotContains(suite.client.completedEvents(), "stop:images")
	suite.assertReady("u-vms")
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
	suite.client.removeOutcome = libvirtstorage.Unknown
	suite.Destroy(suite.spec)

	suite.waitForEvent("remove:images")
	suite.assertHold("u-vms", true)

	suite.client.setErrors(nil, nil)
	suite.volume("x-wakeup")
	suite.mount("x-wakeup")
	ctest.AssertResource(suite, "images", func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Has(poolExclusion))
	})
	suite.assertHold("u-vms", true)
	suite.Require().Equal([]string{"ensure:" + filepath.Join(suite.root, "u-vms", "images"), "remove:images"}, suite.client.completedEvents())
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

// A consumer keeps the old pool path live even after configuration retargets it.
// The controller must not redefine the directory or release its mount until the consumer lets go.
func (suite *StoragePoolSuite) TestHeldRetargetWaitsForConsumerAcrossPasses() {
	suite.activate()
	suite.holdPoolStatus()
	suite.volume("x-new")
	suite.mount("x-new")
	suite.setVolumeID("x-new")

	suite.assertError("held by a consumer")
	suite.assertHold("u-vms", true)
	suite.assertHold("x-new", false)

	// Force another pass while the hold remains; the old target must still be the definition.
	suite.volume("another")
	suite.mount("another")
	suite.assertError("held by a consumer")
	suite.assertHold("u-vms", true)

	pools, err := suite.client.Pools()
	suite.Require().NoError(err)
	suite.Require().Len(pools, 1)
	suite.Require().Equal(filepath.Join(suite.root, "u-vms", "images"), pools[0].Target)
	suite.Require().NotContains(suite.client.completedEvents(), "ensure:"+filepath.Join(suite.root, "x-new", "images"))

	suite.releasePoolStatus()
	suite.assertReady("x-new")
	suite.assertHold("u-vms", false)
	suite.assertHold("x-new", true)
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
		asrt.True(status.TypedSpec().Phase == storageres.StoragePoolPhaseReady, status.TypedSpec().Error)
	})

	_, err := suite.State().Teardown(suite.Ctx(), images.Metadata(), state.WithTeardownOwner("block.MountStatusController"))
	suite.Require().NoError(err)

	suite.waitForEvent("stop:images")
	suite.assertHold("u-vms", false)
	suite.assertHold("u-other", true)
	suite.Require().NotContains(suite.client.completedEvents(), "stop:backups", "an unrelated pool must not be stopped")
}

func (suite *StoragePoolSuite) TestLostStopReplyKeepsMountAndExclusion() {
	mount := suite.mount("u-vms")
	suite.start()
	suite.assertReady("u-vms")
	suite.client.mu.Lock()
	suite.client.stopErr = errors.New("stop reply lost")
	suite.client.stopOutcome = libvirtstorage.Unknown
	suite.client.mu.Unlock()

	_, err := suite.State().Teardown(suite.Ctx(), mount.Metadata(), state.WithTeardownOwner("block.MountStatusController"))
	suite.Require().NoError(err)
	suite.waitForEvent("stop:images")
	suite.assertHold("u-vms", true)
	ctest.AssertResource(suite, "images", func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Has(poolExclusion))
	})
	suite.client.mu.Lock()
	suite.client.stopErr = nil
	suite.client.mu.Unlock()
	suite.volume("x-wakeup")
	suite.mount("x-wakeup")
	suite.assertHold("u-vms", true)
	suite.Require().Equal([]string{"ensure:" + filepath.Join(suite.root, "u-vms", "images"), "stop:images"}, suite.client.completedEvents())
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
		fixture.start()
		fixture.assertError("waiting for storage daemon")
		synctest.Wait()

		// Daemon recovery emits no resource event: only backoff can pick it up.
		fixture.client.setPoolsError(errors.New("inventory unavailable"))
		fixture.client.setErrors(nil, nil)
		synctest.Wait()
		fixture.Require().Empty(fixture.client.completedEvents(), "failed inventory must not submit a mutation")
		fixture.client.setPoolsError(nil)
		fixture.assertReady("u-vms")
		synctest.Wait()

		events := fixture.client.completedEvents()

		synctest.Sleep(10 * time.Second)
		fixture.Require().Equal(events, fixture.client.completedEvents(), "a healthy pool must not be reconciled by a periodic timer")

		// A submitted removal with a lost reply cannot be retried just because
		// the next daemon connection succeeds: the first operation may still run.
		fixture.client.setErrors(nil, errors.New("daemon disconnected"))
		fixture.client.removeOutcome = libvirtstorage.Unknown
		fixture.Destroy(fixture.spec)
		fixture.waitForEvent("remove:images")
		synctest.Wait()
		fixture.assertHold("u-vms", true)

		fixture.client.setErrors(nil, nil)
		fixture.volume("x-wakeup")
		fixture.mount("x-wakeup")
		synctest.Wait()
		ctest.AssertResource(fixture, "images", func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
			asrt.True(status.Metadata().Finalizers().Has(poolExclusion))
		})
		fixture.assertHold("u-vms", true)
		fixture.Require().Equal([]string{"ensure:" + filepath.Join(fixture.root, "u-vms", "images"), "remove:images"}, fixture.client.completedEvents())
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
		fixture.Require().True(status.TypedSpec().Phase == storageres.StoragePoolPhaseReady, status.TypedSpec().Error)
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

// A volume arriving after the pool's hold census but before Ensure must not
// act on the old Ready identity. This uses the real shared COSI state and both
// controllers; only the daemon operation is paused.
func TestPoolRetargetExcludesLateVolume(t *testing.T) {
	t.Parallel()

	fixture := &StoragePoolSuite{Timeout: 10 * time.Second}
	fixture.SetT(t)

	fixture.SetupTest()
	defer fixture.TearDownTest()

	fixture.mount("u-vms")
	fixture.volume("x-new")
	fixture.mount("x-new")
	fixture.start()
	fixture.assertReady("u-vms")

	entered := make(chan struct{})
	release := make(chan struct{})

	var enterOnce, releaseOnce sync.Once

	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()

	fixture.client.mu.Lock()
	fixture.client.beforeEnsure = func(target string) {
		if target != filepath.Join(fixture.root, "x-new", "images") {
			return
		}

		enterOnce.Do(func() {
			close(entered)

			select {
			case <-release:
			case <-fixture.Ctx().Done():
			}
		})
	}
	fixture.client.mu.Unlock()

	fixture.setVolumeID("x-new")

	select {
	case <-entered:
	case <-fixture.Ctx().Done():
		fixture.Require().FailNow("pool did not reach retarget operation")
	}

	ctx, st := fixture.Ctx(), fixture.State()
	poolStatus, err := safe.StateGetByID[*storageres.StoragePoolStatus](ctx, st, "images")
	fixture.Require().NoError(err)
	fixture.Require().True(poolStatus.Metadata().Finalizers().Has("storage.StoragePoolController/mutating/backing"),
		"pool retarget must publish exclusion before Ensure")

	volumeClient := &volumeClient{checkHold: func(pool string) error {
		status, err := safe.StateGetByID[*storageres.StoragePoolStatus](ctx, st, pool)
		if err != nil {
			return err
		}

		if !status.Metadata().Finalizers().Has(volumeFinalizer) {
			return fmt.Errorf("volume operation without pool hold")
		}

		return nil
	}}
	request := storageres.NewStoragePoolVolumeSpec(storageres.NamespaceName, "images/late.raw")
	request.TypedSpec().Pool = "images"
	request.TypedSpec().Name = "late.raw"
	request.TypedSpec().Format = "raw"
	request.TypedSpec().Capacity = 64 << 20
	fixture.Create(request, state.WithCreateOwner("hypervisor.VirtualMachineDiskController"))
	fixture.Require().NoError(fixture.Runtime().RegisterController(&storagectrl.StoragePoolVolumeController{Open: volumeClient.open}))

	ctest.AssertResource(fixture, request.Metadata().ID(), func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.NotEqual(storageres.StoragePoolVolumePhaseReady, status.TypedSpec().Phase,
			"late volume must not use the old Ready pool while its backing changes")
	})
	fixture.Require().Empty(volumeClient.recorded(), "late volume must not be created on old backing")
	unblock()
	fixture.assertReady("x-new")
	ctest.AssertResource(fixture, request.Metadata().ID(), func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseReady, status.TypedSpec().Phase)
	})
}

func TestStoragePoolSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &StoragePoolSuite{
		Timeout: 10 * time.Second,
	})
}

// consumerFinalizer stands in for whatever holds a pool's status while a guest has a volume of that
// pool open -- StoragePoolVolumeController in practice.
const consumerFinalizer = "storage.StoragePoolVolumeController"

func (suite *StoragePoolSuite) holdPoolStatus() {
	status, err := safe.StateGetByID[*storageres.StoragePoolStatus](suite.Ctx(), suite.State(), "images")
	suite.Require().NoError(err)
	suite.Require().NoError(suite.State().AddFinalizer(suite.Ctx(), status.Metadata(), consumerFinalizer))
}

func (suite *StoragePoolSuite) releasePoolStatus() {
	status, err := safe.StateGetByID[*storageres.StoragePoolStatus](suite.Ctx(), suite.State(), "images")
	suite.Require().NoError(err)
	suite.Require().NoError(suite.State().RemoveFinalizer(suite.Ctx(), status.Metadata(), consumerFinalizer))
}

// Removing the pool's configuration while a consumer still holds its status must not undefine the
// pool, and above all must not release the mount: a guest holds its disk open by descriptor, so
// unmounting underneath it is what actually breaks.
func (suite *StoragePoolSuite) TestHeldPoolSurvivesConfigurationRemoval() {
	suite.activate()
	suite.holdPoolStatus()

	suite.Destroy(suite.spec)

	// The status is withdrawn so the holder notices, but it cannot go away while held.
	ctest.AssertResource(suite, "images", func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, status.Metadata().Phase())
	})

	suite.assertHold("u-vms", true)
	suite.Require().NotContains(suite.client.completedEvents(), "remove:images",
		"a pool a guest may be writing into must not be undefined")

	suite.releasePoolStatus()

	ctest.AssertNoResource[*storageres.StoragePoolStatus](suite, "images")
	suite.waitForEvent("remove:images")
	suite.assertHold("u-vms", false)
}

// Outage must not erase the old volume identity needed to defer a held retarget on recovery.
func (suite *StoragePoolSuite) TestHeldRetargetAcrossDaemonOutage() {
	suite.activate()
	suite.holdPoolStatus()
	suite.volume("x-new")
	suite.mount("x-new")
	suite.client.setErrors(os.ErrNotExist, nil)
	suite.setVolumeID("x-new")

	suite.assertError("waiting for storage daemon")
	ctest.AssertResource(suite, "images", func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.Equal("u-vms", status.TypedSpec().VolumeID)
		asrt.Equal(filepath.Join(suite.root, "u-vms", "images"), status.TypedSpec().TargetPath)
		asrt.Equal(storageres.StoragePoolPhaseNotReady, status.TypedSpec().Phase,
			"retarget is intentional withdrawal, not uncertain observation")
	})

	suite.client.setErrors(nil, nil)
	suite.assertError("held by a consumer")
	suite.assertHold("u-vms", true)
	suite.assertHold("x-new", false)

	suite.releasePoolStatus()
	suite.assertReady("x-new")
	suite.assertHold("u-vms", false)
}

// The same guard on the shutdown path: with the daemon gone the pools cannot be enumerated, so the
// status's own record of where the pool was put is what a mount is matched against.
func (suite *StoragePoolSuite) TestHeldPoolKeepsItsMountWithoutTheDaemon() {
	suite.activate()
	suite.holdPoolStatus()

	mount, err := safe.StateGetByID[*block.VolumeMountStatus](suite.Ctx(), suite.State(), "u-vms")
	suite.Require().NoError(err)

	_, err = suite.State().Teardown(suite.Ctx(), mount.Metadata(), state.WithTeardownOwner("block.MountStatusController"))
	suite.Require().NoError(err)

	// With the daemon still up, the pool is held, so the mount is kept. The hold staying put is a
	// negative, and a negative asserted against a controller which has not run yet proves nothing,
	// so a pass after the teardown is waited for first.
	suite.assertError("is not mounted for writing")
	suite.assertHold("u-vms", true)

	// Now take the daemon away, which is the path that used to release unconditionally. Setting the
	// error emits no resource event of its own, so an unrelated mount is what wakes the controller;
	// without it this races the pass the teardown triggered. The error is distinct so that observing
	// it proves the pass ran after the daemon went.
	suite.client.setErrors(errors.New("daemon stopped after the mount began tearing down"), nil)
	suite.volume("x-data")
	suite.mount("x-data")

	suite.assertError("daemon stopped after the mount began tearing down")
	suite.assertHold("u-vms", true)

	suite.releasePoolStatus()
	suite.assertHold("u-vms", false)
}
