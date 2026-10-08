// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

const (
	volumeFinalizer = "storage.StoragePoolVolumeController"

	// volumePool is the one pool every case in this suite uses. Named rather than passed: a pool
	// name threaded through every helper reads like a dimension these tests vary, and they do not.
	volumePool = "images"
)

// volumeClient records what the controller asks of the storage daemon, and refuses anything it asks
// without first holding the pool it is asking about.
type volumeClient struct {
	mu             sync.Mutex
	volumes        map[string]libvirtstorage.Volume
	calls          []string
	backingFiles   []string
	backingFormats []string
	openErr        error
	createErr      error
	resizeErr      error
	resizeRelease  <-chan struct{}
	resizeDone     chan struct{}
	opens          int
	checkHold      func(pool string) error
}

func (c *volumeClient) open(context.Context) (libvirtstorage.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.opens++

	if c.openErr != nil {
		return nil, c.openErr
	}

	return c, nil
}

func (c *volumeClient) key(pool libvirtstorage.Pool, name string) string {
	return pool.Name + "/" + name
}

func (c *volumeClient) Volume(pool libvirtstorage.Pool, name string) (libvirtstorage.Volume, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.checkHold(pool.Name); err != nil {
		return libvirtstorage.Volume{}, false, err
	}

	volume, found := c.volumes[c.key(pool, name)]

	return volume, found, nil
}

func (c *volumeClient) CreateVolume(pool libvirtstorage.Pool, name, format string, capacity uint64, backingFile, backingFormat string) (libvirtstorage.Volume, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.checkHold(pool.Name); err != nil {
		return libvirtstorage.Volume{}, err
	}

	c.calls = append(c.calls, "create:"+c.key(pool, name))
	c.backingFiles = append(c.backingFiles, backingFile)
	c.backingFormats = append(c.backingFormats, backingFormat)

	if c.createErr != nil {
		return libvirtstorage.Volume{}, c.createErr
	}

	volume := libvirtstorage.Volume{
		Name:     name,
		Path:     "/var/mnt/u-vms/" + pool.Name + "/" + name,
		Format:   format,
		Capacity: capacity,
	}

	if c.volumes == nil {
		c.volumes = map[string]libvirtstorage.Volume{}
	}

	c.volumes[c.key(pool, name)] = volume

	return volume, nil
}

func (c *volumeClient) ResizeVolume(pool libvirtstorage.Pool, name string, capacity uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.checkHold(pool.Name); err != nil {
		return err
	}

	c.calls = append(c.calls, "resize:"+c.key(pool, name))

	if c.resizeErr != nil {
		if c.resizeRelease != nil {
			key := c.key(pool, name)
			release := c.resizeRelease
			done := c.resizeDone

			// The transport has failed, but the server-side operation is still in flight.
			go func() {
				defer close(done)

				<-release
				c.mu.Lock()
				defer c.mu.Unlock()

				volume := c.volumes[key]
				volume.Capacity = capacity
				c.volumes[key] = volume
			}()
		}

		return c.resizeErr
	}

	volume := c.volumes[c.key(pool, name)]
	if capacity < volume.Capacity {
		return errors.New("a volume must never be shrunk")
	}

	volume.Capacity = capacity
	c.volumes[c.key(pool, name)] = volume

	return nil
}

func (*volumeClient) Pools() ([]libvirtstorage.Pool, error) {
	panic("StoragePoolVolumeController must not reconcile pool definitions")
}

func (*volumeClient) Ensure(libvirtstorage.Pool, string, func() error) error {
	panic("StoragePoolVolumeController must not reconcile pool definitions")
}

func (*volumeClient) Remove(libvirtstorage.Pool) error {
	panic("StoragePoolVolumeController must not reconcile pool definitions")
}

func (*volumeClient) Stop(libvirtstorage.Pool) error {
	panic("StoragePoolVolumeController must not reconcile pool definitions")
}

func (*volumeClient) Close() {}

func (c *volumeClient) recorded() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.calls)
}

func (c *volumeClient) recordedBackingFiles() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.backingFiles)
}

func (c *volumeClient) recordedBackingFormats() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.backingFormats)
}

func (c *volumeClient) volume(pool, name string) (libvirtstorage.Volume, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	volume, found := c.volumes[pool+"/"+name]

	return volume, found
}

func (c *volumeClient) setVolume(name string, volume libvirtstorage.Volume) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.volumes == nil {
		c.volumes = map[string]libvirtstorage.Volume{}
	}

	c.volumes[volumePool+"/"+name] = volume
}

func (c *volumeClient) setErrors(openErr, createErr, resizeErr error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.openErr, c.createErr, c.resizeErr = openErr, createErr, resizeErr
}

type StoragePoolVolumeSuite struct {
	ctest.DefaultSuite

	client *volumeClient
	logs   *observer.ObservedLogs
}

func (suite *StoragePoolVolumeSuite) SetupTest() {
	// Teed onto the suite's own logger so that "controller failed" -- which the runtime logs when
	// Run returns -- is observable. It is the only crisp signal of a restart; counting reconcile
	// passes is not, since the controller writes one of its own inputs.
	var observed zapcore.Core

	observed, suite.logs = observer.New(zapcore.ErrorLevel)
	suite.Logger = zap.New(zapcore.NewTee(zaptest.NewLogger(suite.T()).Core(), observed))

	suite.DefaultSuite.SetupTest()
	ctx := suite.Ctx()
	st := suite.State()
	suite.client = &volumeClient{
		checkHold: func(pool string) error {
			status, err := safe.StateGetByID[*storageres.StoragePoolStatus](ctx, st, pool)
			if err != nil {
				return err
			}

			if !status.Metadata().Finalizers().Has(volumeFinalizer) {
				return fmt.Errorf("volume operation in pool %q without holding it", pool)
			}

			return nil
		},
	}

	system := hardware.NewSystemInformation(hardware.SystemInformationID)
	system.TypedSpec().UUID = poolMachineUUID
	suite.Create(system)
}

func (suite *StoragePoolVolumeSuite) start() {
	suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.StoragePoolVolumeController{Open: suite.client.open}))
}

func (suite *StoragePoolVolumeSuite) pool(ready bool) *storageres.StoragePoolStatus {
	status := storageres.NewStoragePoolStatus(storageres.NamespaceName, volumePool)
	status.TypedSpec().VolumeID = "u-vms"
	status.TypedSpec().TargetPath = "/var/mnt/u-vms/" + volumePool

	status.TypedSpec().Phase = storageres.StoragePoolPhaseNotReady
	if ready {
		status.TypedSpec().Phase = storageres.StoragePoolPhaseReady
	}

	if !ready {
		status.TypedSpec().Error = "waiting for backing volume"
	}

	suite.Create(status, state.WithCreateOwner("storage.StoragePoolController"))

	return status
}

func (suite *StoragePoolVolumeSuite) spec(pool, name, format string, capacity uint64) *storageres.StoragePoolVolumeSpec {
	spec := storageres.NewStoragePoolVolumeSpec(storageres.NamespaceName, storageres.StoragePoolVolumeID(pool, name))
	spec.TypedSpec().Pool = pool
	spec.TypedSpec().Name = name
	spec.TypedSpec().Format = format
	spec.TypedSpec().Capacity = capacity
	suite.Create(spec, state.WithCreateOwner("hypervisor.VirtualMachineDiskController"))

	return spec
}

// openDisk models an attached consumer without any hypervisor resources.
func (suite *StoragePoolVolumeSuite) openDisk(pool, volume string) *storageres.StoragePoolVolumeStatus {
	status := storageres.NewStoragePoolVolumeStatus(storageres.NamespaceName, storageres.StoragePoolVolumeID(pool, volume))
	status.TypedSpec().Pool = pool
	status.TypedSpec().Name = volume

	_, err := suite.State().Get(suite.Ctx(), status.Metadata())
	if state.IsNotFoundError(err) {
		suite.Create(status, state.WithCreateOwner(volumeFinalizer))
	} else {
		suite.Require().NoError(err)
	}

	suite.AddFinalizer(status.Metadata(), "test.NonHypervisorConsumer")

	return status
}

func (suite *StoragePoolVolumeSuite) assertReady(id string, check func(*storageres.StoragePoolVolumeStatusSpec, *assert.Assertions)) {
	ctest.AssertResource(suite, id, func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseReady, status.TypedSpec().Phase, status.TypedSpec().Error)

		if check != nil {
			check(status.TypedSpec(), asrt)
		}
	})
}

func (suite *StoragePoolVolumeSuite) assertError(id, reason string) {
	ctest.AssertResource(suite, id, func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Contains(status.TypedSpec().Error, reason)
	})
}

func (suite *StoragePoolVolumeSuite) assertPoolHold(held bool) {
	ctest.AssertResource(suite, volumePool, func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.Equal(held, status.Metadata().Finalizers().Has(volumeFinalizer))
	})
}

// assertNoRestartLoop fails if the controller returned from Run. A condition reported through the
// volume's status must not also be returned: that restarts the controller under backoff for as long
// as the condition lasts, which for a format mismatch or an oversized volume is forever.
func (suite *StoragePoolVolumeSuite) assertNoRestartLoop() {
	logs := suite.logs

	suite.Require().Never(func() bool { return logs.FilterMessage("controller failed").Len() > 0 },
		500*time.Millisecond, 50*time.Millisecond, "a reported condition must not restart the controller")
}

// tearDownPool starts a pool's teardown, keeping its status observable through another consumer's
// hold the way StoragePoolController's real consumers do.
func (suite *StoragePoolVolumeSuite) tearDownPool(pool *storageres.StoragePoolStatus) {
	suite.Require().NoError(suite.State().AddFinalizer(suite.Ctx(), pool.Metadata(), "test.OtherConsumer"))

	_, err := suite.State().Teardown(suite.Ctx(), pool.Metadata(), state.WithTeardownOwner("storage.StoragePoolController"))
	suite.Require().NoError(err)
}

func TestStoragePoolVolumeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &StoragePoolVolumeSuite{
		Timeout: 10 * time.Second,
	})
}

// withdrawSpec models its author giving the volume up: torn down first, and destroyed only once the
// hold on it comes back.
func (suite *StoragePoolVolumeSuite) withdrawSpec(spec *storageres.StoragePoolVolumeSpec) {
	_, err := suite.State().Teardown(suite.Ctx(), spec.Metadata(), state.WithTeardownOwner("hypervisor.VirtualMachineDiskController"))
	suite.Require().NoError(err)

	ctest.AssertResource(suite, spec.Metadata().ID(), func(spec *storageres.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.False(spec.Metadata().Finalizers().Has(volumeFinalizer))
	})

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), spec.Metadata(), state.WithDestroyOwner("hypervisor.VirtualMachineDiskController")))
}

func (suite *StoragePoolVolumeSuite) TestPoolPhasePropagationAndRecovery() {
	pool := suite.pool(false)
	suite.spec(volumePool, "phase.raw", "raw", 64<<20)
	suite.start()

	cases := []struct {
		poolPhase   storageres.StoragePoolPhase
		volumePhase storageres.StoragePoolVolumePhase
	}{
		{
			poolPhase:   storageres.StoragePoolPhaseUnknown,
			volumePhase: storageres.StoragePoolVolumePhaseNotReady,
		},
		{
			poolPhase:   storageres.StoragePoolPhaseNotReady,
			volumePhase: storageres.StoragePoolVolumePhaseNotReady,
		},
		{
			poolPhase:   storageres.StoragePoolPhaseObservationUnavailable,
			volumePhase: storageres.StoragePoolVolumePhaseObservationUnavailable,
		},
		{
			poolPhase:   storageres.StoragePoolPhaseReady,
			volumePhase: storageres.StoragePoolVolumePhaseReady,
		},
		{
			poolPhase:   storageres.StoragePoolPhaseNotReady,
			volumePhase: storageres.StoragePoolVolumePhaseNotReady,
		},
	}

	for _, tt := range cases {
		ctest.UpdateWithConflicts(suite, pool, func(current *storageres.StoragePoolStatus) error {
			current.TypedSpec().Phase = tt.poolPhase
			// Identical diagnostics must not determine phase classification.
			current.TypedSpec().Error = "storage observation unavailable"

			return nil
		}, state.WithUpdateOwner("storage.StoragePoolController"))
		ctest.AssertResource(suite, storageres.StoragePoolVolumeID(volumePool, "phase.raw"), func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(tt.volumePhase, status.TypedSpec().Phase)
		})
	}
}

func (suite *StoragePoolVolumeSuite) TestReleasesPendingPoolWhenNoGuestUsesVolume() {
	pool := suite.pool(true)
	suite.spec("images", "vm1__data.qcow2", "qcow2", 64<<20)
	suite.start()
	suite.assertReady(storageres.StoragePoolVolumeID("images", "vm1__data.qcow2"), nil)
	suite.assertPoolHold(true)

	ctest.UpdateWithConflicts(suite, pool, func(status *storageres.StoragePoolStatus) error {
		status.TypedSpec().Phase = storageres.StoragePoolPhaseNotReady
		status.TypedSpec().Error = "pool retarget held"

		return nil
	}, state.WithUpdateOwner("storage.StoragePoolController"))

	suite.assertError(storageres.StoragePoolVolumeID("images", "vm1__data.qcow2"), "pool retarget held")
	suite.assertPoolHold(false)
}

func (suite *StoragePoolVolumeSuite) TestPendingPoolKeepsGuestHoldUntilDiskReleased() {
	pool := suite.pool(true)
	suite.spec("images", "vm1__data.qcow2", "qcow2", 64<<20)
	disk := suite.openDisk("images", "vm1__data.qcow2")
	suite.start()
	suite.assertReady(storageres.StoragePoolVolumeID("images", "vm1__data.qcow2"), nil)

	ctest.UpdateWithConflicts(suite, pool, func(status *storageres.StoragePoolStatus) error {
		status.TypedSpec().Phase = storageres.StoragePoolPhaseNotReady
		status.TypedSpec().Error = "pool retarget held"

		return nil
	}, state.WithUpdateOwner("storage.StoragePoolController"))

	suite.assertError(storageres.StoragePoolVolumeID("images", "vm1__data.qcow2"), "pool retarget held")
	suite.assertPoolHold(true)

	suite.RemoveFinalizer(disk.Metadata(), "test.NonHypervisorConsumer")
	suite.assertPoolHold(false)
}

func (suite *StoragePoolVolumeSuite) TestCreatesWhenAbsent() {
	suite.pool(true)
	suite.spec("images", "vm1__data.qcow2", "qcow2", 64<<20)
	suite.start()

	id := storageres.StoragePoolVolumeID("images", "vm1__data.qcow2")
	suite.assertReady(id, func(spec *storageres.StoragePoolVolumeStatusSpec, asrt *assert.Assertions) {
		asrt.Equal("/var/mnt/u-vms/images/vm1__data.qcow2", spec.Path)
		asrt.Equal("qcow2", spec.Format)
		asrt.Equal(uint64(64<<20), spec.Capacity)
		asrt.Zero(spec.PendingCapacity)
		asrt.Empty(spec.Error)
	})

	suite.assertPoolHold(true)
	suite.Require().Equal([]string{"create:images/vm1__data.qcow2"}, suite.client.recorded())
	suite.Require().Equal([]string{""}, suite.client.recordedBackingFiles())
	suite.Require().Equal([]string{""}, suite.client.recordedBackingFormats())
}

func (suite *StoragePoolVolumeSuite) TestCreatePassesBackingFile() {
	suite.pool(true)
	spec := suite.spec(volumePool, "overlay.qcow2", "qcow2", 64<<20)
	ctest.UpdateWithConflicts(suite, spec, func(current *storageres.StoragePoolVolumeSpec) error {
		current.TypedSpec().BackingFile = "/var/mnt/u-vms/images/base.qcow2"
		current.TypedSpec().BackingFormat = "qcow2"

		return nil
	}, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController"))
	suite.start()

	suite.assertReady(storageres.StoragePoolVolumeID(volumePool, "overlay.qcow2"), nil)
	suite.Require().Equal([]string{"/var/mnt/u-vms/images/base.qcow2"}, suite.client.recordedBackingFiles())
	suite.Require().Equal([]string{"qcow2"}, suite.client.recordedBackingFormats())
}

// The ordinary case after any reboot: the volume is already there, and the same names mean the same
// disk. Adoption must cost no daemon call at all.
func (suite *StoragePoolVolumeSuite) TestAdoptsAnExistingVolume() {
	suite.pool(true)
	suite.client.setVolume("vm1__data.qcow2", libvirtstorage.Volume{
		Name: "vm1__data.qcow2", Path: "/var/mnt/u-vms/images/vm1__data.qcow2", Format: "qcow2", Capacity: 64 << 20,
	})
	suite.spec("images", "vm1__data.qcow2", "qcow2", 64<<20)
	suite.start()

	suite.assertReady(storageres.StoragePoolVolumeID("images", "vm1__data.qcow2"), nil)
	suite.Require().Empty(suite.client.recorded(), "an adopted volume must be neither created nor resized")
}

type volumePublicationController struct {
	controller.Controller
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

type volumePublicationRuntime struct {
	controller.Runtime
	observer *volumePublicationController
}

func (r volumePublicationRuntime) Modify(ctx context.Context, empty resource.Resource, update func(resource.Resource) error, options ...controller.ModifyOption) error {
	if empty.Metadata().Type() == storageres.StoragePoolVolumeStatusType && empty.Metadata().ID() == "images/failure.raw" {
		r.observer.once.Do(func() {
			close(r.observer.entered)

			select {
			case <-r.observer.release:
			case <-ctx.Done():
			}
		})
	}

	return r.Runtime.Modify(ctx, empty, update, options...)
}

func (c *volumePublicationController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	return c.Controller.Run(ctx, volumePublicationRuntime{Runtime: r, observer: c}, logger)
}

// A failed transport call need not mean the server stopped resizing. A Ready
// observation from before the call cannot be admitted during status publication.
func (suite *StoragePoolVolumeSuite) TestResizeFailureKeepsMarkerUntilStatusPublication() {
	pool := suite.pool(true)
	suite.client.setVolume("failure.raw", libvirtstorage.Volume{
		Name: "failure.raw", Path: "/var/mnt/u-vms/images/failure.raw", Format: "raw", Capacity: 64 << 20,
	})
	suite.client.setErrors(nil, nil, errors.New("resize disconnected"))
	suite.spec(volumePool, "failure.raw", "raw", 128<<20)

	previous := storageres.NewStoragePoolVolumeStatus(storageres.NamespaceName, "images/failure.raw")
	previous.TypedSpec().Pool = volumePool
	previous.TypedSpec().Name = "failure.raw"
	previous.TypedSpec().Path = "/var/mnt/u-vms/images/failure.raw"
	previous.TypedSpec().Format = "raw"
	previous.TypedSpec().Capacity = 64 << 20
	previous.TypedSpec().Phase = storageres.StoragePoolVolumePhaseReady
	suite.Create(previous, state.WithCreateOwner(volumeFinalizer))

	entered := make(chan struct{})
	release := make(chan struct{})

	defer close(release)

	suite.Require().NoError(suite.Runtime().RegisterController(&volumePublicationController{
		Controller: &storagectrl.StoragePoolVolumeController{Open: suite.client.open},
		entered:    entered,
		release:    release,
	}))

	select {
	case <-entered:
	case <-suite.Ctx().Done():
		suite.Require().FailNow("resize did not reach status publication")
	}

	status, err := safe.StateGetByID[*storageres.StoragePoolStatus](suite.Ctx(), suite.State(), pool.Metadata().ID())
	suite.Require().NoError(err)
	observed, err := safe.StateGetByID[*storageres.StoragePoolVolumeStatus](suite.Ctx(), suite.State(), previous.Metadata().ID())
	suite.Require().NoError(err)
	suite.Require().Equal(storageres.StoragePoolVolumePhaseReady, observed.TypedSpec().Phase,
		"the old Ready observation remains until status publication")
	suite.Require().True(status.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("failure.raw")),
		"resize failure must retain admission exclusion before error status publication")
}

func (suite *StoragePoolVolumeSuite) TestAmbiguousResizeFailureKeepsAdmissionClosed() {
	pool := suite.pool(true)
	suite.client.setVolume("failure.raw", libvirtstorage.Volume{
		Name:     "failure.raw",
		Path:     "/var/mnt/u-vms/images/failure.raw",
		Format:   "raw",
		Capacity: 64 << 20,
	})
	suite.client.setErrors(nil, nil, errors.New("resize disconnected"))
	suite.spec(volumePool, "failure.raw", "raw", 128<<20)
	suite.start()
	suite.assertError("images/failure.raw", "resize disconnected")
	ctest.AssertResource(suite, pool.Metadata().ID(), func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("failure.raw")))
	})

	// Neither a later successful lookup nor a recovered transport proves that the
	// first resize has stopped executing server-side. No retry or admission yet.
	// Even observing the requested capacity does not prove that the daemon
	// finished its in-flight resize before this observation.
	suite.client.setVolume("failure.raw", libvirtstorage.Volume{
		Name:     "failure.raw",
		Path:     "/var/mnt/u-vms/images/failure.raw",
		Format:   "raw",
		Capacity: 128 << 20,
	})
	suite.client.setErrors(nil, nil, nil)
	other := suite.spec(volumePool, "wake.raw", "raw", 64<<20)
	suite.assertReady(other.Metadata().ID(), nil)
	suite.Require().Equal([]string{"resize:images/failure.raw", "create:images/wake.raw"}, suite.client.recorded())
	ctest.AssertResource(suite, "images/failure.raw", func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.NotEqual(storageres.StoragePoolVolumePhaseReady, status.TypedSpec().Phase)
	})
	ctest.AssertResource(suite, pool.Metadata().ID(), func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("failure.raw")))
	})
}

// Neither withdrawal nor a daemon outage proves an unattributed mutation finished.
func (suite *StoragePoolVolumeSuite) TestKeepsUnattributedMutationDuringOutageAndTeardown() {
	pool := suite.pool(false)
	ctest.UpdateWithConflicts(suite, pool, func(status *storageres.StoragePoolStatus) error {
		status.TypedSpec().Phase = storageres.StoragePoolPhaseObservationUnavailable

		return nil
	}, state.WithUpdateOwner("storage.StoragePoolController"))
	suite.AddFinalizer(pool.Metadata(), storageres.StoragePoolVolumeMutationFinalizer("withdrawn.raw"))
	suite.tearDownPool(pool)
	suite.start()
	ctest.AssertResource(suite, pool.Metadata().ID(), func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("withdrawn.raw")))
		asrt.True(status.Metadata().Finalizers().Has("test.OtherConsumer"))
	})
	suite.Require().Empty(suite.client.recorded())
	suite.client.mu.Lock()
	defer suite.client.mu.Unlock()

	suite.Equal(0, suite.client.opens, "retaining an uncertain marker must not dial storage")
}

func (suite *StoragePoolVolumeSuite) TestKeepsUnattributedMutationOnRetarget() {
	oldPool := suite.pool(true)
	suite.AddFinalizer(oldPool.Metadata(), volumeFinalizer)
	suite.AddFinalizer(oldPool.Metadata(), storageres.StoragePoolVolumeMutationFinalizer("old.raw"))

	newPool := storageres.NewStoragePoolStatus(storageres.NamespaceName, "other")
	newPool.TypedSpec().Phase = storageres.StoragePoolPhaseReady
	suite.Create(newPool, state.WithCreateOwner("storage.StoragePoolController"))
	suite.spec("other", "new.raw", "raw", 64<<20)
	suite.start()
	suite.assertReady("other/new.raw", nil)
	ctest.AssertResource(suite, oldPool.Metadata().ID(), func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.False(status.Metadata().Finalizers().Has(volumeFinalizer))
		asrt.True(status.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("old.raw")))
	})
	ctest.AssertResource(suite, newPool.Metadata().ID(), func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Has(volumeFinalizer))
	})
}

func (suite *StoragePoolVolumeSuite) TestWithdrawAndRecreateDuringAmbiguousResize() {
	pool := suite.pool(true)
	suite.client.setVolume("pending.raw", libvirtstorage.Volume{
		Name:     "pending.raw",
		Path:     "/var/mnt/u-vms/images/pending.raw",
		Format:   "raw",
		Capacity: 64 << 20,
	})

	release := make(chan struct{})
	done := make(chan struct{})

	var releaseOnce sync.Once

	unblock := func() { releaseOnce.Do(func() { close(release) }) }

	suite.T().Cleanup(func() {
		unblock()

		select {
		case <-done:
		case <-time.After(time.Second):
			suite.T().Error("fake daemon resize failed to finish")
		}
	})

	suite.client.resizeRelease = release
	suite.client.resizeDone = done
	suite.client.setErrors(nil, nil, errors.New("resize disconnected"))
	request := suite.spec(volumePool, "pending.raw", "raw", 128<<20)
	suite.start()
	suite.assertError(request.Metadata().ID(), "resize disconnected")
	suite.withdrawSpec(request)
	ctest.AssertNoResource[*storageres.StoragePoolVolumeStatus](suite, request.Metadata().ID())
	ctest.AssertResource(suite, pool.Metadata().ID(), func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("pending.raw")))
	})

	suite.client.setErrors(nil, nil, nil)
	recreated := suite.spec(volumePool, "pending.raw", "raw", 128<<20)
	suite.assertError(recreated.Metadata().ID(), "requires confirming the daemon is quiescent")
	suite.Require().Equal([]string{"resize:images/pending.raw"}, suite.client.recorded(),
		"a recreated request must not reuse a volume while the old resize runs")

	// Even after the fake server completes, observing the target capacity cannot
	// establish completion of a real unacknowledged RPC.
	unblock()
	<-done

	completed, exists := suite.client.volume(volumePool, "pending.raw")
	suite.Require().True(exists)
	suite.Require().Equal(uint64(128<<20), completed.Capacity)
	wake := suite.spec(volumePool, "wake.raw", "raw", 64<<20)
	suite.assertReady(wake.Metadata().ID(), nil)
	suite.assertError(recreated.Metadata().ID(), "requires confirming the daemon is quiescent")
	suite.Require().Equal([]string{"resize:images/pending.raw", "create:images/wake.raw"}, suite.client.recorded())
}

func (suite *StoragePoolVolumeSuite) TestGrowsAVolumeNothingHasOpen() {
	suite.pool(true)
	suite.client.setVolume("vm1__data.qcow2", libvirtstorage.Volume{
		Name: "vm1__data.qcow2", Path: "/var/mnt/u-vms/images/vm1__data.qcow2", Format: "qcow2", Capacity: 64 << 20,
	})
	suite.spec("images", "vm1__data.qcow2", "qcow2", 128<<20)
	suite.start()

	suite.assertReady(storageres.StoragePoolVolumeID("images", "vm1__data.qcow2"),
		func(spec *storageres.StoragePoolVolumeStatusSpec, asrt *assert.Assertions) {
			asrt.Equal(uint64(128<<20), spec.Capacity)
			asrt.Zero(spec.PendingCapacity)
			// A ready status with no path renders as <source file=''/>.
			asrt.Equal("/var/mnt/u-vms/images/vm1__data.qcow2", spec.Path)
			asrt.Equal("qcow2", spec.Format)
		})

	suite.Require().Equal([]string{"resize:images/vm1__data.qcow2"}, suite.client.recorded())
	ctest.AssertResource(suite, volumePool, func(pool *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.False(pool.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("vm1__data.qcow2")),
			"acknowledged resize must retire its marker")
	})
}

// Growing a volume QEMU has open corrupts qcow2 metadata and silently lies for raw. The growth is
// remembered instead, and the disk stays attachable so the guest is not stopped over it.
func (suite *StoragePoolVolumeSuite) TestDefersAGrowthWhileTheVolumeIsOpen() {
	suite.pool(true)
	suite.client.setVolume("vm1__data.qcow2", libvirtstorage.Volume{
		Name: "vm1__data.qcow2", Path: "/var/mnt/u-vms/images/vm1__data.qcow2", Format: "qcow2", Capacity: 64 << 20,
	})
	suite.openDisk("images", "vm1__data.qcow2")
	suite.spec("images", "vm1__data.qcow2", "qcow2", 128<<20)
	suite.start()

	id := storageres.StoragePoolVolumeID("images", "vm1__data.qcow2")
	suite.assertReady(id, func(spec *storageres.StoragePoolVolumeStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(uint64(64<<20), spec.Capacity)
		asrt.Equal(uint64(128<<20), spec.PendingCapacity)
	})
	suite.assertError(id, "detach all consumers to apply growth")

	suite.Require().Empty(suite.client.recorded(), "a volume a guest has open must not be resized")

	volume, _ := suite.client.volume("images", "vm1__data.qcow2")
	suite.Require().Equal(uint64(64<<20), volume.Capacity)

	suite.assertNoRestartLoop()
}

// A smaller size is reported and ignored. Refusing the disk outright would stop a running guest
// over an edited number, which is worse than the mismatch.
func (suite *StoragePoolVolumeSuite) TestRefusesToShrinkButStaysReady() {
	suite.pool(true)
	suite.client.setVolume("vm1__data.qcow2", libvirtstorage.Volume{
		Name: "vm1__data.qcow2", Path: "/var/mnt/u-vms/images/vm1__data.qcow2", Format: "qcow2", Capacity: 128 << 20,
	})
	suite.spec("images", "vm1__data.qcow2", "qcow2", 64<<20)
	suite.start()

	id := storageres.StoragePoolVolumeID("images", "vm1__data.qcow2")
	suite.assertReady(id, func(spec *storageres.StoragePoolVolumeStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(uint64(128<<20), spec.Capacity)
	})
	suite.assertError(id, "never shrunk")
	suite.Require().Empty(suite.client.recorded())
	suite.assertNoRestartLoop()
}

// A format mismatch is the one case that does withhold the disk: attaching a qcow2 file as raw
// hands the guest its header as block zero.
func (suite *StoragePoolVolumeSuite) TestRefusesAFormatMismatch() {
	suite.pool(true)
	suite.client.setVolume("vm1__data.raw", libvirtstorage.Volume{
		Name: "vm1__data.raw", Path: "/var/mnt/u-vms/images/vm1__data.raw", Format: "qcow2", Capacity: 64 << 20,
	})
	suite.spec("images", "vm1__data.raw", "raw", 64<<20)
	suite.start()

	id := storageres.StoragePoolVolumeID("images", "vm1__data.raw")
	ctest.AssertResource(suite, id, func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseNotReady, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Error, "move the file aside")
	})
	suite.Require().Empty(suite.client.recorded(), "a volume of the wrong format must be neither rewritten nor deleted")
	suite.assertNoRestartLoop()
}

func (suite *StoragePoolVolumeSuite) TestWaitsForThePool() {
	suite.pool(false)
	suite.spec("images", "vm1__data.qcow2", "qcow2", 64<<20)
	suite.start()

	suite.assertError(storageres.StoragePoolVolumeID("images", "vm1__data.qcow2"), "is not ready")
	suite.Require().Empty(suite.client.recorded())
}

func (suite *StoragePoolVolumeSuite) TestWaitsForAnUndeclaredPool() {
	suite.spec("nowhere", "vm1__data.qcow2", "qcow2", 64<<20)
	suite.start()

	suite.assertError(storageres.StoragePoolVolumeID("nowhere", "vm1__data.qcow2"), "not declared by a StoragePool document")
	ctest.AssertResource(suite, storageres.StoragePoolVolumeID("nowhere", "vm1__data.qcow2"), func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.NotEqual(storageres.StoragePoolVolumePhaseObservationUnavailable, status.TypedSpec().Phase)
	})
	suite.Require().Zero(suite.client.opens, "a volume with no pool must not dial the storage daemon")
}

// Withdrawing the spec withdraws the status and the hold -- and nothing else. There is no verb on
// the storage client which could delete the volume, and there must not be.
func (suite *StoragePoolVolumeSuite) TestSpecRemovalRetainsTheVolume() {
	suite.pool(true)
	spec := suite.spec("images", "vm1__data.qcow2", "qcow2", 64<<20)
	suite.start()

	id := storageres.StoragePoolVolumeID("images", "vm1__data.qcow2")
	suite.assertReady(id, nil)
	suite.assertPoolHold(true)

	suite.withdrawSpec(spec)

	ctest.AssertNoResource[*storageres.StoragePoolVolumeStatus](suite, id)
	suite.assertPoolHold(false)

	_, found := suite.client.volume("images", "vm1__data.qcow2")
	suite.Require().True(found, "removing the configuration must never delete a volume")
	suite.Require().Equal([]string{"create:images/vm1__data.qcow2"}, suite.client.recorded())
}

// The pool is held once for all its volumes, and released only when the last one goes.
func (suite *StoragePoolVolumeSuite) TestPoolIsHeldUntilTheLastVolumeGoes() {
	suite.pool(true)
	first := suite.spec("images", "vm1__data.qcow2", "qcow2", 64<<20)
	suite.spec("images", "vm2__data.qcow2", "qcow2", 64<<20)
	suite.start()

	suite.assertReady(storageres.StoragePoolVolumeID("images", "vm1__data.qcow2"), nil)
	suite.assertReady(storageres.StoragePoolVolumeID("images", "vm2__data.qcow2"), nil)

	suite.withdrawSpec(first)

	ctest.AssertNoResource[*storageres.StoragePoolVolumeStatus](suite, storageres.StoragePoolVolumeID("images", "vm1__data.qcow2"))
	suite.assertPoolHold(true)
}

// A pool on its way out is refused rather than held: taking a hold now would block its teardown for
// good.
func (suite *StoragePoolVolumeSuite) TestRefusesToHoldAPoolGoingAway() {
	pool := suite.pool(true)
	suite.spec("images", "vm1__data.qcow2", "qcow2", 64<<20)
	suite.start()

	id := storageres.StoragePoolVolumeID("images", "vm1__data.qcow2")
	suite.assertReady(id, nil)

	suite.tearDownPool(pool)

	suite.assertError(id, "is going away")
	suite.assertPoolHold(false)
}

// The same pool, but with a guest still writing into it. Here the hold must stay: releasing it lets
// StoragePoolController remove the pool and give the mount back while QEMU has the qcow2 open by
// file descriptor. The consumer-held volume status keeps the pool in place.
func (suite *StoragePoolVolumeSuite) TestKeepsAPoolGoingAwayHeldForAGuest() {
	pool := suite.pool(true)
	suite.client.setVolume("vm1__data.qcow2", libvirtstorage.Volume{
		Name: "vm1__data.qcow2", Path: "/var/mnt/u-vms/images/vm1__data.qcow2", Format: "qcow2", Capacity: 64 << 20,
	})

	spec := suite.spec("images", "vm1__data.qcow2", "qcow2", 64<<20)
	suite.start()

	id := storageres.StoragePoolVolumeID("images", "vm1__data.qcow2")
	suite.assertReady(id, nil)
	suite.assertPoolHold(true)

	disk := suite.openDisk("images", "vm1__data.qcow2")

	// The configuration drops both the pool and the disk, which is what a guest outliving its own
	// spec looks like from here.
	suite.tearDownPool(pool)
	_, err := suite.State().Teardown(suite.Ctx(), spec.Metadata(), state.WithTeardownOwner("hypervisor.VirtualMachineDiskController"))
	suite.Require().NoError(err)

	ctest.AssertResource(suite, id, func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, status.Metadata().Phase())
	})

	ctx := suite.Ctx()
	st := suite.State()
	suite.Require().Never(func() bool {
		status, err := safe.StateGetByID[*storageres.StoragePoolStatus](ctx, st, "images")

		return err == nil && !status.Metadata().Finalizers().Has(volumeFinalizer)
	}, time.Second, 50*time.Millisecond, "a pool a guest is writing into must stay held")

	// It does come back once the domain lets the disk go.
	suite.Require().NoError(suite.State().RemoveFinalizer(suite.Ctx(), disk.Metadata(), "test.NonHypervisorConsumer"))
	suite.assertPoolHold(false)
}

func (suite *StoragePoolVolumeSuite) TestDaemonRecoveryWithoutResourceEvent() {
	client := suite.client
	logs := suite.logs

	client.setErrors(errors.New("daemon offline"), nil, nil)
	suite.pool(true)
	suite.spec(volumePool, "retry.raw", "raw", 64<<20)
	suite.start()

	ctest.AssertResource(suite, "images/retry.raw", func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseObservationUnavailable, status.TypedSpec().Phase)
	})
	suite.Require().Eventually(func() bool {
		return logs.FilterMessage("controller failed").Len() > 0
	}, 3*time.Second, 10*time.Millisecond, "daemon-open failure must return a retryable error")

	// Recovery changes only the external daemon, never a watched resource.
	client.setErrors(nil, nil, nil)
	suite.assertReady("images/retry.raw", nil)
}

func (suite *StoragePoolVolumeSuite) TestRecoversCreateFailureWithoutResourceEvent() {
	client := suite.client
	client.setErrors(nil, errors.New("create disconnected"), nil)
	suite.pool(true)
	suite.spec(volumePool, "create-retry.raw", "raw", 64<<20)
	suite.start()
	ctest.AssertResource(suite, "images/create-retry.raw", func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseNotReady, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Error, "create disconnected")
	})
	client.setErrors(nil, nil, nil)
	suite.assertReady("images/create-retry.raw", nil)
}

func (suite *StoragePoolVolumeSuite) TestDaemonOutagePreservesObservedVolume() {
	client := suite.client
	suite.pool(true)
	suite.spec(volumePool, "observed.raw", "raw", 64<<20)
	suite.start()
	suite.assertReady("images/observed.raw", nil)
	before, err := safe.StateGetByID[*storageres.StoragePoolVolumeStatus](suite.Ctx(), suite.State(), "images/observed.raw")
	suite.Require().NoError(err)
	client.setErrors(errors.New("daemon offline"), nil, nil)
	suite.spec(volumePool, "wake.raw", "raw", 64<<20)
	ctest.AssertResource(suite, "images/observed.raw", func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseObservationUnavailable, status.TypedSpec().Phase)
		asrt.Equal(before.TypedSpec().Path, status.TypedSpec().Path)
		asrt.Equal(before.TypedSpec().Capacity, status.TypedSpec().Capacity)
		asrt.Equal(before.TypedSpec().Format, status.TypedSpec().Format)
		asrt.Equal(before.TypedSpec().Pool, status.TypedSpec().Pool)
		asrt.Equal(before.TypedSpec().Name, status.TypedSpec().Name)
	})
	client.setErrors(nil, nil, nil)
	suite.assertReady("images/observed.raw", nil)
}

func (suite *StoragePoolVolumeSuite) TestConsumerHoldRetainsWithdrawnVolumeAndPool() {
	suite.pool(true)
	request := suite.spec(volumePool, "held.raw", "raw", 64<<20)
	suite.start()
	suite.assertReady("images/held.raw", nil)

	status := storageres.NewStoragePoolVolumeStatus(storageres.NamespaceName, "images/held.raw")
	suite.AddFinalizer(status.Metadata(), "test.NonHypervisorConsumer")
	_, err := suite.State().Teardown(suite.Ctx(), request.Metadata(), state.WithTeardownOwner("hypervisor.VirtualMachineDiskController"))
	suite.Require().NoError(err)
	ctest.AssertResource(suite, "images/held.raw", func(current *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, current.Metadata().Phase())
		asrt.True(current.Metadata().Finalizers().Has("test.NonHypervisorConsumer"))
	})
	suite.assertPoolHold(true)
	ctx := suite.Ctx()
	st := suite.State()
	suite.Require().Never(func() bool {
		pool, err := safe.StateGetByID[*storageres.StoragePoolStatus](ctx, st, volumePool)

		return err != nil || !pool.Metadata().Finalizers().Has(volumeFinalizer)
	}, 300*time.Millisecond, 10*time.Millisecond, "held status must retain its backing pool after request withdrawal")
	suite.RemoveFinalizer(status.Metadata(), "test.NonHypervisorConsumer")
	ctest.AssertNoResource[*storageres.StoragePoolVolumeStatus](suite, "images/held.raw")
	suite.assertPoolHold(false)
	ctest.AssertResource(suite, request.Metadata().ID(), func(current *storageres.StoragePoolVolumeSpec, asrt *assert.Assertions) {
		asrt.False(current.Metadata().Finalizers().Has(volumeFinalizer))
	})
}

type volumeTeardownController struct {
	controller.Controller
	beforeTeardown func()
	settled        chan struct{}
}

type volumeTeardownRuntime struct {
	controller.Runtime
	observer *volumeTeardownController
}

func (r volumeTeardownRuntime) Teardown(ctx context.Context, pointer resource.Pointer, options ...controller.DeleteOption) (bool, error) {
	if pointer.Type() == storageres.StoragePoolVolumeStatusType && pointer.ID() == "images/race.raw" {
		r.observer.beforeTeardown()
	}

	return r.Runtime.Teardown(ctx, pointer, options...)
}

func (r volumeTeardownRuntime) ResetRestartBackoff() {
	r.Runtime.ResetRestartBackoff()

	select {
	case r.observer.settled <- struct{}{}:
	default:
	}
}

func (c *volumeTeardownController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	return c.Controller.Run(ctx, volumeTeardownRuntime{
		Runtime:  r,
		observer: c,
	}, logger)
}

func (suite *StoragePoolVolumeSuite) TestConsumerHoldBetweenCensusAndTeardown() {
	ctx := suite.Ctx()
	st := suite.State()
	entered := make(chan struct{})
	release := make(chan struct{})
	settled := make(chan struct{}, 1)

	var enterOnce, releaseOnce sync.Once

	unblock := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	defer unblock()

	suite.pool(true)
	request := suite.spec(volumePool, "race.raw", "raw", 64<<20)
	suite.Require().NoError(suite.Runtime().RegisterController(&volumeTeardownController{
		Controller: &storagectrl.StoragePoolVolumeController{
			Open: suite.client.open,
		},
		beforeTeardown: func() {
			enterOnce.Do(func() {
				close(entered)

				select {
				case <-release:
				case <-ctx.Done():
				}
			})
		},
		settled: settled,
	}))
	suite.assertReady("images/race.raw", nil)

	_, err := st.Teardown(ctx, request.Metadata(), state.WithTeardownOwner("hypervisor.VirtualMachineDiskController"))
	suite.Require().NoError(err)

	select {
	case <-entered:
	case <-ctx.Done():
		suite.Require().FailNow("storage did not reach output teardown")
	}

	// Storage already read an empty usage census. Admission still may commit
	// while the status is Running, so pool release needs a fresh census.
	status := storageres.NewStoragePoolVolumeStatus(storageres.NamespaceName, "images/race.raw")
	suite.AddFinalizer(status.Metadata(), "test.NonHypervisorConsumer")

	select {
	case <-settled:
	default:
	}

	unblock()

	select {
	case <-settled:
	case <-ctx.Done():
		suite.Require().FailNow("storage did not finish teardown reconciliation")
	}

	pool, err := safe.StateGetByID[*storageres.StoragePoolStatus](ctx, st, volumePool)
	suite.Require().NoError(err)
	suite.True(pool.Metadata().Finalizers().Has(volumeFinalizer), "late consumer hold must retain backing pool")
	suite.RemoveFinalizer(status.Metadata(), "test.NonHypervisorConsumer")
	ctest.AssertNoResource[*storageres.StoragePoolVolumeStatus](suite, status.Metadata().ID())
	suite.assertPoolHold(false)
}

func (suite *StoragePoolVolumeSuite) TestHeldVolumePoolRetargetWaitsForDetach() {
	suite.assertHeldVolumeRetarget("replacement", "retarget.raw")
}

func (suite *StoragePoolVolumeSuite) TestHeldVolumeRenameWaitsForDetach() {
	suite.assertHeldVolumeRetarget(volumePool, "renamed.raw")
}

func (suite *StoragePoolVolumeSuite) assertHeldVolumeRetarget(pool, name string) {
	suite.pool(true)

	replacement := storageres.NewStoragePoolStatus(storageres.NamespaceName, "replacement")
	replacement.TypedSpec().Phase = storageres.StoragePoolPhaseReady
	replacement.TypedSpec().TargetPath = "/replacement"
	suite.Create(replacement)

	request := suite.spec(volumePool, "retarget.raw", "raw", 64<<20)
	suite.start()
	suite.assertReady(request.Metadata().ID(), nil)

	status, err := safe.StateGetByID[*storageres.StoragePoolVolumeStatus](suite.Ctx(), suite.State(), request.Metadata().ID())
	suite.Require().NoError(err)

	original := *status.TypedSpec()
	calls := suite.client.recorded()
	suite.AddFinalizer(status.Metadata(), "test.NonHypervisorConsumer")
	ctest.UpdateWithConflicts(suite, request, func(current *storageres.StoragePoolVolumeSpec) error {
		current.TypedSpec().Pool = pool
		current.TypedSpec().Name = name

		return nil
	}, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController"))
	ctest.AssertResource(suite, status.Metadata().ID(), func(current *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, current.Metadata().Phase())
		asrt.Equal(original, *current.TypedSpec(), "attached status must retain original backing identity")
		asrt.True(current.Metadata().Finalizers().Has("test.NonHypervisorConsumer"))
	})
	suite.assertPoolHold(true)
	suite.Require().Equal(calls, suite.client.recorded(), "replacement must not be provisioned before detach")
	suite.RemoveFinalizer(status.Metadata(), "test.NonHypervisorConsumer")
	suite.assertReady(request.Metadata().ID(), func(current *storageres.StoragePoolVolumeStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(pool, current.Pool)
		asrt.Equal(name, current.Name)
	})
	suite.assertPoolHold(pool == volumePool)
}

func (suite *StoragePoolVolumeSuite) TestReportsAnUnavailableDaemon() {
	suite.client.setErrors(errors.New("dial unix: no such file"), nil, nil)
	suite.pool(true)
	suite.spec("images", "vm1__data.qcow2", "qcow2", 64<<20)
	suite.start()

	suite.assertError(storageres.StoragePoolVolumeID("images", "vm1__data.qcow2"), "waiting for storage daemon")
	ctest.AssertResource(suite, storageres.StoragePoolVolumeID("images", "vm1__data.qcow2"), func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseObservationUnavailable, status.TypedSpec().Phase)
	})
}
