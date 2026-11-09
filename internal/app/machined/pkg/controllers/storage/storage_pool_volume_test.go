// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	"github.com/siderolabs/talos/internal/pkg/libvirt/storage/copyowner"
	"github.com/siderolabs/talos/internal/pkg/qemuimg"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
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
	mu               sync.Mutex
	volumes          map[string]libvirtstorage.Volume
	calls            []string
	backingFiles     []string
	backingFormats   []string
	openErr          error
	readErr          error
	createErr        error
	resizeErr        error
	resizeOutcome    libvirtstorage.OperationOutcome
	resizeRelease    <-chan struct{}
	resizeDone       chan struct{}
	copyStarted      chan struct{}
	copyContexts     chan context.Context
	copyRelease      <-chan struct{}
	copyPool         libvirtstorage.Pool
	copyName         string
	copySource       string
	copyExpected     string
	copyPreparations int
	opens            int
	checkHold        func(pool string) error
}

func (c *volumeClient) open(context.Context) (libvirtstorage.VolumeClient, error) {
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

	if c.readErr != nil {
		return libvirtstorage.Volume{}, false, c.readErr
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

func (c *volumeClient) CopyVolume(pool libvirtstorage.Pool, name, source, expected string) (libvirtstorage.Volume, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.checkHold(pool.Name); err != nil {
		return libvirtstorage.Volume{}, err
	}

	if c.readErr != nil {
		return libvirtstorage.Volume{}, c.readErr
	}

	if previous, found := c.volumes[c.key(pool, name)]; found {
		return previous, nil
	}

	if source == "" {
		return libvirtstorage.Volume{}, libvirtstorage.ErrCopySourceRequired
	}

	c.calls = append(c.calls, "copy:"+c.key(pool, name)+":"+source+":"+expected)
	volume := libvirtstorage.Volume{
		Name:     name,
		Path:     "/var/mnt/u-vms/" + pool.Name + "/" + name,
		Format:   "raw",
		Capacity: 8,
	}

	if c.volumes == nil {
		c.volumes = map[string]libvirtstorage.Volume{}
	}

	c.volumes[c.key(pool, name)] = volume

	return volume, nil
}

func (c *volumeClient) AdoptCopy(pool libvirtstorage.Pool, name string, _ *libvirtstorage.CopyCache) (libvirtstorage.Volume, bool, error) {
	v, err := c.CopyVolume(pool, name, "", "")
	if errors.Is(err, libvirtstorage.ErrCopySourceRequired) {
		return libvirtstorage.Volume{}, false, nil
	}

	return v, err == nil, err
}

func (c *volumeClient) PrepareCopy(pool libvirtstorage.Pool, name, source, expected string) (*libvirtstorage.CopyOperation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.copyPreparations++
	c.copyPool, c.copyName, c.copySource, c.copyExpected = pool, name, source, expected

	return nil, nil
}

func (c *volumeClient) FinishCopy(ctx context.Context, _ libvirtstorage.Pool, _ *libvirtstorage.CopyOperation, validate func() error) (libvirtstorage.Volume, error) {
	if c.copyContexts != nil {
		select {
		case c.copyContexts <- ctx:
		default:
		}
	}

	if c.copyStarted != nil {
		select {
		case c.copyStarted <- struct{}{}:
		default:
		}

		select {
		case <-c.copyRelease:
		case <-ctx.Done():
			return libvirtstorage.Volume{}, ctx.Err()
		}
	}

	if err := validate(); err != nil {
		return libvirtstorage.Volume{}, err
	}

	c.mu.Lock()
	pool, name, source, expected := c.copyPool, c.copyName, c.copySource, c.copyExpected
	c.mu.Unlock()

	return c.CopyVolume(pool, name, source, expected)
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

func (c *volumeClient) ResizeVolumeOperation(pool libvirtstorage.Pool, name string, capacity uint64) (libvirtstorage.OperationOutcome, error) {
	err := c.ResizeVolume(pool, name, capacity)
	c.mu.Lock()
	defer c.mu.Unlock()

	if err != nil {
		return c.resizeOutcome, err
	}

	return libvirtstorage.Finished, nil
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

	observed, suite.logs = observer.New(zapcore.DebugLevel)
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

func (suite *StoragePoolVolumeSuite) TestAdoptedWritableImageGrowsWithoutSource() {
	suite.pool(true)
	spec := suite.spec(volumePool, "guest.img", "raw", 16)
	ctest.UpdateWithConflicts(suite, spec, func(current *storageres.StoragePoolVolumeSpec) error {
		current.TypedSpec().Copy = &storageres.StoragePoolVolumeCopySpec{Image: true}

		return nil
	}, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController"))
	suite.client.setVolume("guest.img", libvirtstorage.Volume{
		Name:     "guest.img",
		Path:     "/var/mnt/u-vms/" + volumePool + "/guest.img",
		Format:   "raw",
		Capacity: 8,
	})
	suite.start()
	ctest.AssertResource(suite, spec.Metadata().ID(), func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseReady, status.TypedSpec().Phase, status.TypedSpec().Error)
		asrt.EqualValues(16, status.TypedSpec().Capacity)
	})
}

func (suite *StoragePoolVolumeSuite) TestAdoptedWritableImageNeverShrinks() {
	suite.pool(true)
	spec := suite.spec(volumePool, "guest.img", "raw", 4)
	ctest.UpdateWithConflicts(suite, spec, func(current *storageres.StoragePoolVolumeSpec) error {
		current.TypedSpec().Copy = &storageres.StoragePoolVolumeCopySpec{Image: true}

		return nil
	}, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController"))
	suite.client.setVolume("guest.img", libvirtstorage.Volume{
		Name:     "guest.img",
		Path:     "/var/mnt/u-vms/" + volumePool + "/guest.img",
		Format:   "raw",
		Capacity: 8,
	})
	suite.start()
	ctest.AssertResource(suite, spec.Metadata().ID(), func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseReady, status.TypedSpec().Phase, status.TypedSpec().Error)
		asrt.EqualValues(8, status.TypedSpec().Capacity)

		if asrt.NotNil(status.TypedSpec().Copy) {
			asrt.Equal(storageres.StoragePoolVolumeCopyPhaseSucceeded, status.TypedSpec().Copy.Phase)
		}
	})
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
func (suite *StoragePoolVolumeSuite) openDisk(volume string) *storageres.StoragePoolVolumeStatus {
	status := storageres.NewStoragePoolVolumeStatus(storageres.NamespaceName, storageres.StoragePoolVolumeID("images", volume))
	status.TypedSpec().Pool = "images"
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

func (suite *StoragePoolVolumeSuite) TestCopyWorkerDebugLifecycle() {
	suite.pool(true)
	request := suite.copySpec("progress.raw")
	suite.start()
	suite.assertReady(request.Metadata().ID(), nil)
	suite.Require().Eventually(func() bool {
		return suite.logs.FilterMessage("copy provisioning succeeded").Len() == 1
	}, time.Second, time.Millisecond)

	for _, message := range []string{"copy worker started", "copy worker finished", "copy provisioning succeeded"} {
		entries := suite.logs.FilterMessage(message).All()
		suite.Require().Len(entries, 1, message)
		suite.Require().Equal(zapcore.DebugLevel, entries[0].Level)
		suite.Require().Equal(volumePool, entries[0].ContextMap()["storage_pool"])
		suite.Require().Equal("progress.raw", entries[0].ContextMap()["volume"])
	}
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

func (suite *StoragePoolVolumeSuite) TestCopiesImageInsteadOfCreatingOverlay() {
	suite.pool(true)
	spec := suite.spec(volumePool, "vm__install.raw", "raw", 0)
	ctest.UpdateWithConflicts(suite, spec, func(current *storageres.StoragePoolVolumeSpec) error {
		current.TypedSpec().Copy = &storageres.StoragePoolVolumeCopySpec{
			Source: "/library/object",
			Digest: "sha256:123",
		}

		return nil
	}, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController"))
	suite.start()

	suite.assertReady(spec.Metadata().ID(), nil)
	suite.Equal([]string{"copy:images/vm__install.raw:/library/object:sha256:123"}, suite.client.recorded())
}

func (suite *StoragePoolVolumeSuite) TestBlockedCopyDoesNotBlockBlankCreation() {
	suite.pool(true)

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	suite.client.copyStarted = started

	suite.client.copyRelease = release
	defer close(release)

	spec := suite.spec(volumePool, "a-copy.raw", "raw", 0)
	ctest.UpdateWithConflicts(suite, spec, func(current *storageres.StoragePoolVolumeSpec) error {
		current.TypedSpec().Copy = &storageres.StoragePoolVolumeCopySpec{Source: "/library/object"}

		return nil
	}, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController"))
	suite.start()
	suite.Require().Eventually(func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)

	blank := suite.spec(volumePool, "z-blank.raw", "raw", 4096)
	ctest.AssertResource(suite, blank.Metadata().ID(), func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseReady, status.TypedSpec().Phase)
	})
}

func (suite *StoragePoolVolumeSuite) TestCopyObservationOutage() {
	suite.pool(true)
	suite.client.readErr = libvirtstorage.ErrVolumeObservationUnavailable
	spec := suite.spec(volumePool, "copy.raw", "raw", 0)
	ctest.UpdateWithConflicts(suite, spec, func(current *storageres.StoragePoolVolumeSpec) error {
		current.TypedSpec().Copy = &storageres.StoragePoolVolumeCopySpec{}

		return nil
	}, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController"))
	suite.start()

	ctest.AssertResource(suite, spec.Metadata().ID(), func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseObservationUnavailable, status.TypedSpec().Phase)
	})
}

type completedVolumeFixture struct {
	request *storageres.StoragePoolVolumeSpec
	pool    *storageres.StoragePoolStatus
	disk    *hypervisor.VirtualMachineDiskStatus
	volume  *storageres.StoragePoolVolumeStatus
}

func (suite *StoragePoolVolumeSuite) completedVolume(name, owner, target string) completedVolumeFixture {
	suite.client.setErrors(nil, nil, nil)
	suite.Require().NoError(os.MkdirAll(target, 0o700))

	pool := storageres.NewStoragePoolStatus(storageres.NamespaceName, name)
	*pool.TypedSpec() = storageres.StoragePoolStatusSpec{
		VolumeID:   name + "-backing",
		TargetPath: target,
		Phase:      storageres.StoragePoolPhaseReady,
	}
	suite.Create(pool, state.WithCreateOwner("storage.StoragePoolController"))
	suite.Require().NoError(os.WriteFile(filepath.Join(target, "copy.raw"), []byte("image"), 0o600))
	suite.Require().NoError(os.Mkdir(filepath.Join(target, ".copy-on-use"), 0o700))

	witness := fmt.Sprintf("%x", sha256.Sum256([]byte("copy.raw")))
	suite.Require().NoError(os.Link(filepath.Join(target, "copy.raw"), filepath.Join(target, ".copy-on-use", witness)))

	suite.client.mu.Lock()
	if suite.client.volumes == nil {
		suite.client.volumes = map[string]libvirtstorage.Volume{}
	}

	suite.client.volumes[name+"/copy.raw"] = libvirtstorage.Volume{
		Name:     "copy.raw",
		Path:     filepath.Join(target, "copy.raw"),
		Format:   "raw",
		Capacity: 5,
	}
	suite.client.mu.Unlock()

	key := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
	disk := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, name+"/install")
	disk.TypedSpec().Pool = name
	disk.TypedSpec().Volume = "copy.raw"
	disk.Metadata().Labels().Set(libvirtstorage.CopyBindingLabel, key)
	suite.Create(disk, state.WithCreateOwner("hypervisor.VirtualMachineDiskController"))

	request := storageres.NewStoragePoolVolumeSpec(storageres.NamespaceName, name+"/copy.raw")
	*request.TypedSpec() = storageres.StoragePoolVolumeSpecSpec{
		Pool:   name,
		Name:   "copy.raw",
		Format: "raw",
		Copy: &storageres.StoragePoolVolumeCopySpec{
			Source: "/library/object",
		},
	}
	request.Metadata().Labels().Set(libvirtstorage.CopyBindingLabel, key)
	suite.Create(request, state.WithCreateOwner(owner))
	suite.assertCopyPhase(request.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseSucceeded)
	suite.assertReady(request.Metadata().ID(), nil)
	volume, err := safe.StateGetByID[*storageres.StoragePoolVolumeStatus](suite.Ctx(), suite.State(), request.Metadata().ID())
	suite.Require().NoError(err)

	return completedVolumeFixture{
		request: request,
		pool:    pool,
		disk:    disk,
		volume:  volume,
	}
}

func (suite *StoragePoolVolumeSuite) TestCompletedCopyOutageFences() {
	suite.start()

	target := suite.T().TempDir()

	for _, tc := range []struct {
		name   string
		change func(completedVolumeFixture)
	}{
		{name: "changed-source", change: func(f completedVolumeFixture) {
			suite.changeCopySpec(f.request, func(current *storageres.StoragePoolVolumeSpec) {
				current.TypedSpec().Copy.Source = "/library/replacement"
			})
		}},
		{name: "changed-digest", change: func(f completedVolumeFixture) {
			suite.changeCopySpec(f.request, func(current *storageres.StoragePoolVolumeSpec) {
				current.TypedSpec().Copy.Digest = "sha256:replacement"
			})
		}},
		{name: "request-incarnation", change: func(f completedVolumeFixture) {
			suite.forceDestroy(f.request.Metadata(), "hypervisor.VirtualMachineDiskController")
			replacement := storageres.NewStoragePoolVolumeSpec(storageres.NamespaceName, f.request.Metadata().ID())
			*replacement.TypedSpec() = *f.request.TypedSpec()
			key, _ := f.request.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
			replacement.Metadata().Labels().Set(libvirtstorage.CopyBindingLabel, key)
			suite.Create(replacement, state.WithCreateOwner("hypervisor.VirtualMachineDiskController"))
			suite.NotEqual(f.request.Metadata().Created(), replacement.Metadata().Created())
		}},
		{name: "changed-backing", change: func(f completedVolumeFixture) {
			ctest.UpdateWithConflicts(suite, f.pool, func(current *storageres.StoragePoolStatus) error {
				current.TypedSpec().VolumeID = "replacement"

				return nil
			}, state.WithUpdateOwner("storage.StoragePoolController"))
		}},
		{name: "changed-target", change: func(f completedVolumeFixture) {
			ctest.UpdateWithConflicts(suite, f.pool, func(current *storageres.StoragePoolStatus) error {
				current.TypedSpec().TargetPath = filepath.Join(target, "replacement")

				return nil
			}, state.WithUpdateOwner("storage.StoragePoolController"))
		}},
		{name: "pool-incarnation", change: func(f completedVolumeFixture) {
			suite.forceDestroy(f.pool.Metadata(), "storage.StoragePoolController")

			replacement := storageres.NewStoragePoolStatus(storageres.NamespaceName, f.pool.Metadata().ID())
			*replacement.TypedSpec() = *f.pool.TypedSpec()
			suite.Create(replacement, state.WithCreateOwner("storage.StoragePoolController"))
			suite.NotEqual(f.pool.Metadata().Created(), replacement.Metadata().Created())
		}},
		{name: "pool-teardown", change: func(f completedVolumeFixture) {
			_, err := suite.State().Teardown(suite.Ctx(), f.pool.Metadata(), state.WithTeardownOwner("storage.StoragePoolController"))
			suite.Require().NoError(err)
		}},
		{name: "missing-pool", change: func(f completedVolumeFixture) {
			suite.forceDestroy(f.pool.Metadata(), "storage.StoragePoolController")
		}},
		{name: "pool-mutation", change: func(f completedVolumeFixture) {
			suite.AddFinalizer(f.pool.Metadata(), "storage.StoragePoolController/mutating/backing")
		}},
		{name: "volume-mutation", change: func(f completedVolumeFixture) {
			suite.AddFinalizer(f.pool.Metadata(), storageres.StoragePoolVolumeMutationFinalizer("copy.raw"))
		}},
		{name: "missing-copy", change: func(f completedVolumeFixture) {
			ctest.UpdateWithConflicts(suite, f.volume, func(current *storageres.StoragePoolVolumeStatus) error {
				current.TypedSpec().Copy = nil

				return nil
			}, state.WithUpdateOwner(volumeFinalizer))
		}},
		{name: "waiting-copy", change: func(f completedVolumeFixture) {
			ctest.UpdateWithConflicts(suite, f.volume, func(current *storageres.StoragePoolVolumeStatus) error {
				current.TypedSpec().Copy.Phase = storageres.StoragePoolVolumeCopyPhaseWaiting
				current.TypedSpec().Error = "test: unqualified observation"

				return nil
			}, state.WithUpdateOwner(volumeFinalizer))
		}},
		{name: "missing-status", change: func(f completedVolumeFixture) {
			suite.Destroy(f.volume, state.WithDestroyOwner(volumeFinalizer))
		}},
		{name: "missing-binding", change: func(f completedVolumeFixture) {
			suite.changeCopySpec(f.request, func(current *storageres.StoragePoolVolumeSpec) {
				current.Metadata().Labels().Delete(libvirtstorage.CopyBindingLabel)
			})
		}},
		{name: "changed-binding", change: func(f completedVolumeFixture) {
			suite.changeCopySpec(f.request, func(current *storageres.StoragePoolVolumeSpec) {
				current.Metadata().Labels().Set(libvirtstorage.CopyBindingLabel, strings.Repeat("f", 64))
			})
		}},
		{name: "missing-disk", change: func(f completedVolumeFixture) {
			suite.Destroy(f.disk, state.WithDestroyOwner("hypervisor.VirtualMachineDiskController"))
		}},
		{name: "foreign-disk", change: func(f completedVolumeFixture) {
			suite.Destroy(f.disk, state.WithDestroyOwner("hypervisor.VirtualMachineDiskController"))

			replacement := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, f.disk.Metadata().ID())
			*replacement.TypedSpec() = *f.disk.TypedSpec()
			key, _ := f.disk.Metadata().Labels().Get(libvirtstorage.CopyBindingLabel)
			replacement.Metadata().Labels().Set(libvirtstorage.CopyBindingLabel, key)
			suite.Create(replacement, state.WithCreateOwner("external.Producer"))
		}},
		{name: "disk-teardown", change: func(f completedVolumeFixture) {
			_, err := suite.State().Teardown(suite.Ctx(), f.disk.Metadata(), state.WithTeardownOwner("hypervisor.VirtualMachineDiskController"))
			suite.Require().NoError(err)
		}},
	} {
		suite.Run(tc.name, func() {
			f := suite.completedVolume(tc.name, "hypervisor.VirtualMachineDiskController", filepath.Join(target, tc.name))
			suite.loseCompletedCopyObservation(f)
			tc.change(f)
			ctest.AssertResource(suite, f.request.Metadata().ID(), func(current *storageres.StoragePoolVolumeStatus, a *assert.Assertions) {
				a.NotEqual("test: unqualified observation", current.TypedSpec().Error, "wait for the producer to consume the changed observation")

				if a.NotNil(current.TypedSpec().Copy) {
					a.Equal(storageres.StoragePoolVolumeCopyPhaseWaiting, current.TypedSpec().Copy.Phase)
				}
			})
		})
	}
}

func (suite *StoragePoolVolumeSuite) loseCompletedCopyObservation(f completedVolumeFixture) {
	suite.client.setErrors(errors.New("storage observation outage"), nil, nil)
	suite.changeCopySpec(f.request, func(current *storageres.StoragePoolVolumeSpec) {
		current.Metadata().Labels().Set("test/wake", "outage")
	})
	ctest.AssertResource(suite, f.request.Metadata().ID(), func(current *storageres.StoragePoolVolumeStatus, a *assert.Assertions) {
		a.Equal(storageres.StoragePoolVolumePhaseObservationUnavailable, current.TypedSpec().Phase)
		a.Equal(f.volume.TypedSpec().Copy, current.TypedSpec().Copy)
	})
}

func (suite *StoragePoolVolumeSuite) TestCompletedCopyOutageRequestWithdrawal() {
	suite.start()

	target := suite.T().TempDir()

	for _, name := range []string{"remove-copy", "withdraw-request", "missing-request"} {
		suite.Run(name, func() {
			f := suite.completedVolume(name, "hypervisor.VirtualMachineDiskController", filepath.Join(target, name))
			suite.loseCompletedCopyObservation(f)

			switch name {
			case "remove-copy":
				suite.changeCopySpec(f.request, func(current *storageres.StoragePoolVolumeSpec) {
					current.TypedSpec().Copy = nil
				})
				ctest.AssertResource(suite, f.request.Metadata().ID(), func(current *storageres.StoragePoolVolumeStatus, a *assert.Assertions) {
					a.Nil(current.TypedSpec().Copy)
				})
			case "withdraw-request":
				_, err := suite.State().Teardown(suite.Ctx(), f.request.Metadata(), state.WithTeardownOwner("hypervisor.VirtualMachineDiskController"))
				suite.Require().NoError(err)
				ctest.AssertNoResource[*storageres.StoragePoolVolumeStatus](suite, f.request.Metadata().ID())
			case "missing-request":
				suite.forceDestroy(f.request.Metadata(), "hypervisor.VirtualMachineDiskController")
				ctest.AssertNoResource[*storageres.StoragePoolVolumeStatus](suite, f.request.Metadata().ID())
			}
		})
	}
}

func (suite *StoragePoolVolumeSuite) TestCompletedCopyOutageRejectsForeignRequest() {
	suite.start()
	f := suite.completedVolume("foreign-owner", "external.Producer", suite.T().TempDir())
	suite.client.setErrors(errors.New("storage observation outage"), nil, nil)
	ctest.UpdateWithConflicts(suite, f.pool, func(current *storageres.StoragePoolStatus) error {
		current.Metadata().Labels().Set("test/wake", "outage")

		return nil
	}, state.WithUpdateOwner("storage.StoragePoolController"))
	suite.assertCopyPhase(f.request.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseWaiting)
}

func (suite *StoragePoolVolumeSuite) TestCompletedCopyPoolObservationOutage() {
	suite.start()
	f := suite.completedVolume("pool-outage", "hypervisor.VirtualMachineDiskController", suite.T().TempDir())
	ctest.UpdateWithConflicts(suite, f.pool, func(current *storageres.StoragePoolStatus) error {
		current.TypedSpec().Phase = storageres.StoragePoolPhaseObservationUnavailable

		return nil
	}, state.WithUpdateOwner("storage.StoragePoolController"))
	ctest.AssertResource(suite, f.request.Metadata().ID(), func(current *storageres.StoragePoolVolumeStatus, a *assert.Assertions) {
		a.Equal(storageres.StoragePoolVolumePhaseObservationUnavailable, current.TypedSpec().Phase)
		a.Equal(f.volume.TypedSpec().Copy, current.TypedSpec().Copy)
	})
	ctest.UpdateWithConflicts(suite, f.pool, func(current *storageres.StoragePoolStatus) error {
		current.TypedSpec().Phase = storageres.StoragePoolPhaseReady

		return nil
	}, state.WithUpdateOwner("storage.StoragePoolController"))
	suite.assertReady(f.request.Metadata().ID(), nil)
	suite.assertCopyPhase(f.request.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseSucceeded)
}

func (suite *StoragePoolVolumeSuite) TestCompletedCopyRetrySupersedesProofDuringOutage() {
	release := make(chan struct{})

	releaseCopy := sync.OnceFunc(func() { close(release) })
	defer releaseCopy()

	suite.client.copyStarted = make(chan struct{}, 1)
	suite.client.copyRelease = release
	suite.start()

	f := suite.completedVolume("retry-completed", "hypervisor.VirtualMachineDiskController", suite.T().TempDir())
	suite.client.mu.Lock()
	delete(suite.client.volumes, f.request.Metadata().ID())
	suite.client.mu.Unlock()
	suite.changeCopySpec(f.request, func(current *storageres.StoragePoolVolumeSpec) {
		current.Metadata().Labels().Set("test/wake", "retry")
	})

	select {
	case <-suite.client.copyStarted:
	case <-time.After(time.Second):
		suite.FailNow("retry did not enter the copy worker")
	}

	suite.assertCopyPhase(f.request.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseRunning)
	suite.client.setErrors(errors.New("storage observation outage"), nil, nil)
	suite.changeCopySpec(f.request, func(current *storageres.StoragePoolVolumeSpec) {
		current.Metadata().Labels().Set("test/wake", "outage")
	})
	suite.assertCopyPhase(f.request.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseRunning)
	releaseCopy()
	// Finishing the worker during an outage cannot reuse success from before
	// that retry. A fresh, qualified observation is required to recover it.
	suite.assertCopyPhase(f.request.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseWaiting)
	suite.client.setErrors(nil, nil, nil)
	suite.assertReady(f.request.Metadata().ID(), nil)
	suite.assertCopyPhase(f.request.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseSucceeded)
}

func (suite *StoragePoolVolumeSuite) TestCopyWithoutSourceNeverCreatesBlankVolume() {
	suite.pool(true)
	spec := suite.spec(volumePool, "copy.raw", "raw", 0)
	ctest.UpdateWithConflicts(suite, spec, func(current *storageres.StoragePoolVolumeSpec) error {
		current.TypedSpec().Copy = &storageres.StoragePoolVolumeCopySpec{}

		return nil
	}, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController"))
	suite.start()

	ctest.AssertResource(suite, spec.Metadata().ID(), func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseNotReady, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Error, libvirtstorage.ErrCopySourceRequired.Error())

		if asrt.NotNil(status.TypedSpec().Copy) {
			asrt.Equal(storageres.StoragePoolVolumeCopyPhaseSourceRequired, status.TypedSpec().Copy.Phase)
		}
	})
	suite.Empty(suite.client.recorded())
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
	disk := suite.openDisk("vm1__data.qcow2")
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
	id      string
	fail    bool
}

type volumePublicationRuntime struct {
	controller.Runtime
	observer *volumePublicationController
}

func (r volumePublicationRuntime) Modify(ctx context.Context, empty resource.Resource, update func(resource.Resource) error, options ...controller.ModifyOption) error {
	if empty.Metadata().Type() == storageres.StoragePoolVolumeStatusType && empty.Metadata().ID() == r.observer.id {
		first := false

		r.observer.once.Do(func() {
			first = true

			close(r.observer.entered)

			select {
			case <-r.observer.release:
			case <-ctx.Done():
			}
		})

		if first && r.observer.fail {
			return errors.New("status publication rejected")
		}
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
	suite.client.resizeOutcome = libvirtstorage.Unknown
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
		id:         "images/failure.raw",
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

func (suite *StoragePoolVolumeSuite) TestResizeMarkerSurvivesSuccessfulStatusPublication() {
	suite.assertResizeMarkerDuringPublication(false, nil)
}

func (suite *StoragePoolVolumeSuite) TestResizeMarkerSurvivesFailedStatusPublication() {
	suite.assertResizeMarkerDuringPublication(true, nil)
}

func (suite *StoragePoolVolumeSuite) TestTerminalFailedResizeRetainsMarkerUntilErrorStatusPublication() {
	suite.assertResizeMarkerDuringPublication(false, errors.New("terminal resize rejection"))
}

func (suite *StoragePoolVolumeSuite) TestTerminalFailedResizePublicationFailureKeepsMarker() {
	suite.assertResizeMarkerDuringPublication(true, errors.New("terminal resize rejection"))
}

func (suite *StoragePoolVolumeSuite) assertResizeMarkerDuringPublication(fail bool, resizeErr error) {
	pool := suite.pool(true)
	suite.client.resizeOutcome = libvirtstorage.Finished
	suite.client.setErrors(nil, nil, resizeErr)
	suite.client.setVolume("publish.raw", libvirtstorage.Volume{
		Name: "publish.raw", Path: "/var/mnt/u-vms/images/publish.raw", Format: "raw", Capacity: 64 << 20,
	})

	previous := storageres.NewStoragePoolVolumeStatus(storageres.NamespaceName, "images/publish.raw")
	previous.TypedSpec().Pool, previous.TypedSpec().Name = volumePool, "publish.raw"
	previous.TypedSpec().Phase = storageres.StoragePoolVolumePhaseReady
	previous.TypedSpec().Capacity = 64 << 20
	suite.Create(previous, state.WithCreateOwner(volumeFinalizer))
	suite.spec(volumePool, "publish.raw", "raw", 128<<20)

	entered, release := make(chan struct{}), make(chan struct{})

	var once sync.Once

	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()

	suite.Require().NoError(suite.Runtime().RegisterController(&volumePublicationController{
		Controller: &storagectrl.StoragePoolVolumeController{Open: suite.client.open},
		entered:    entered, release: release, id: previous.Metadata().ID(), fail: fail,
	}))

	select {
	case <-entered:
	case <-suite.Ctx().Done():
		suite.FailNow("resize did not reach status publication")
	}

	observed, err := safe.StateGetByID[*storageres.StoragePoolVolumeStatus](suite.Ctx(), suite.State(), previous.Metadata().ID())
	suite.Require().NoError(err)
	suite.Equal(storageres.StoragePoolVolumePhaseReady, observed.TypedSpec().Phase)
	suite.Equal(uint64(64<<20), observed.TypedSpec().Capacity)
	current, err := safe.StateGetByID[*storageres.StoragePoolStatus](suite.Ctx(), suite.State(), pool.Metadata().ID())
	suite.Require().NoError(err)
	suite.True(current.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("publish.raw")), "marker must cover stale Ready publication window")
	unblock()

	if fail {
		suite.Require().Eventually(func() bool { return suite.logs.FilterMessage("controller failed").Len() > 0 }, 3*time.Second, 10*time.Millisecond)
		current, err = safe.StateGetByID[*storageres.StoragePoolStatus](suite.Ctx(), suite.State(), pool.Metadata().ID())
		suite.Require().NoError(err)
		suite.True(current.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("publish.raw")), "failed publication cannot retire marker")
	} else {
		if resizeErr != nil {
			suite.assertError(previous.Metadata().ID(), resizeErr.Error())
			ctest.AssertResource(suite, previous.Metadata().ID(), func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
				asrt.Equal(storageres.StoragePoolVolumePhaseNotReady, status.TypedSpec().Phase)
			})
		} else {
			suite.assertReady(previous.Metadata().ID(), func(spec *storageres.StoragePoolVolumeStatusSpec, asrt *assert.Assertions) {
				asrt.Equal(uint64(128<<20), spec.Capacity)
			})
		}

		ctest.AssertResource(suite, pool.Metadata().ID(), func(current *storageres.StoragePoolStatus, asrt *assert.Assertions) {
			asrt.False(current.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("publish.raw")))
		})
	}
}

// A pre-submission error does not make a failed status write into an uncertain
// daemon mutation. The same request must be able to retry after publication fails.
func (suite *StoragePoolVolumeSuite) TestNoSubmittedResizePublicationFailureReleasesMarkerAndRetries() {
	pool := suite.pool(true)
	suite.client.setVolume("retry-publish.raw", libvirtstorage.Volume{
		Name: "retry-publish.raw", Path: "/var/mnt/u-vms/images/retry-publish.raw", Format: "raw", Capacity: 64 << 20,
	})
	suite.client.resizeOutcome = libvirtstorage.NoMutationSubmitted
	suite.client.setErrors(nil, nil, errors.New("lookup failed before submission"))

	previous := storageres.NewStoragePoolVolumeStatus(storageres.NamespaceName, "images/retry-publish.raw")
	previous.TypedSpec().Pool, previous.TypedSpec().Name = volumePool, "retry-publish.raw"
	previous.TypedSpec().Phase = storageres.StoragePoolVolumePhaseReady
	previous.TypedSpec().Capacity = 64 << 20
	suite.Create(previous, state.WithCreateOwner(volumeFinalizer))
	suite.spec(volumePool, "retry-publish.raw", "raw", 128<<20)

	entered, release := make(chan struct{}), make(chan struct{})

	var once sync.Once

	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()

	suite.Require().NoError(suite.Runtime().RegisterController(&volumePublicationController{
		Controller: &storagectrl.StoragePoolVolumeController{Open: suite.client.open},
		entered:    entered, release: release, id: previous.Metadata().ID(), fail: true,
	}))

	select {
	case <-entered:
	case <-suite.Ctx().Done():
		suite.FailNow("pre-submission resize did not reach status publication")
	}

	current, err := safe.StateGetByID[*storageres.StoragePoolStatus](suite.Ctx(), suite.State(), pool.Metadata().ID())
	suite.Require().NoError(err)
	suite.True(current.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("retry-publish.raw")), "marker covers status publication")
	unblock()
	suite.Require().Eventually(func() bool { return suite.logs.FilterMessage("controller failed").Len() > 0 }, 3*time.Second, 10*time.Millisecond)
	ctest.AssertResource(suite, pool.Metadata().ID(), func(current *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.False(current.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("retry-publish.raw")))
	})
	suite.client.setErrors(nil, nil, nil)
	suite.assertReady(previous.Metadata().ID(), func(spec *storageres.StoragePoolVolumeStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(uint64(128<<20), spec.Capacity)
	})
}

type volumeInvalidOutcomeClient struct{ libvirtstorage.VolumeClient }

func (volumeInvalidOutcomeClient) ResizeVolumeOperation(libvirtstorage.Pool, string, uint64) (libvirtstorage.OperationOutcome, error) {
	return libvirtstorage.Unknown, nil
}

func (suite *StoragePoolVolumeSuite) TestUnknownNilResizeOutcomeNeverPublishesReady() {
	pool := suite.pool(true)
	suite.client.setVolume("unknown-nil.raw", libvirtstorage.Volume{
		Name: "unknown-nil.raw", Path: "/var/mnt/u-vms/images/unknown-nil.raw", Format: "raw", Capacity: 64 << 20,
	})
	request := suite.spec(volumePool, "unknown-nil.raw", "raw", 128<<20)
	suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.StoragePoolVolumeController{
		Open: func(ctx context.Context) (libvirtstorage.VolumeClient, error) {
			client, err := suite.client.open(ctx)
			if err != nil {
				return nil, err
			}

			return volumeInvalidOutcomeClient{client}, nil
		},
	}))
	suite.assertError(request.Metadata().ID(), "uncertain")
	ctest.AssertResource(suite, request.Metadata().ID(), func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.NotEqual(storageres.StoragePoolVolumePhaseReady, status.TypedSpec().Phase)
	})
	ctest.AssertResource(suite, pool.Metadata().ID(), func(current *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(current.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("unknown-nil.raw")))
	})
}

func (suite *StoragePoolVolumeSuite) TestNotSubmittedResizeRetriesWithoutOrphanMarker() {
	pool := suite.pool(true)
	suite.client.setVolume("retry.raw", libvirtstorage.Volume{
		Name: "retry.raw", Path: "/var/mnt/u-vms/images/retry.raw", Format: "raw", Capacity: 64 << 20,
	})
	suite.client.resizeOutcome = libvirtstorage.NoMutationSubmitted
	suite.client.setErrors(nil, nil, errors.New("lookup failed before submission"))
	suite.spec(volumePool, "retry.raw", "raw", 128<<20)
	suite.start()
	suite.assertError("images/retry.raw", "lookup failed before submission")
	ctest.AssertResource(suite, pool.Metadata().ID(), func(current *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.False(current.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("retry.raw")))
	})
	suite.client.setErrors(nil, nil, nil)
	suite.assertReady("images/retry.raw", func(spec *storageres.StoragePoolVolumeStatusSpec, asrt *assert.Assertions) {
		asrt.Equal(uint64(128<<20), spec.Capacity)
	})
	suite.Require().GreaterOrEqual(len(suite.client.recorded()), 2, "pre-submission failure must be retried")
}

func (suite *StoragePoolVolumeSuite) TestFinishedResizeErrorReleasesMarkerAfterPublication() {
	pool := suite.pool(true)
	suite.client.setVolume("finished.raw", libvirtstorage.Volume{
		Name: "finished.raw", Path: "/var/mnt/u-vms/images/finished.raw", Format: "raw", Capacity: 64 << 20,
	})
	suite.client.resizeOutcome = libvirtstorage.Finished
	suite.client.setErrors(nil, nil, errors.New("terminal resize rejection"))
	suite.spec(volumePool, "finished.raw", "raw", 128<<20)
	suite.start()
	suite.assertError("images/finished.raw", "terminal resize rejection")
	ctest.AssertResource(suite, pool.Metadata().ID(), func(current *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.False(current.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("finished.raw")))
	})
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
	suite.client.resizeOutcome = libvirtstorage.Unknown
	suite.spec(volumePool, "failure.raw", "raw", 128<<20)
	suite.start()
	// The first error may already have advanced to the persistent uncertain-resize observation.
	suite.assertError("images/failure.raw", "resize")
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
	suite.client.resizeOutcome = libvirtstorage.Unknown
	suite.client.setErrors(nil, nil, errors.New("resize disconnected"))
	request := suite.spec(volumePool, "pending.raw", "raw", 128<<20)
	suite.start()
	// Reconciliation may already have replaced the transport error with persistent uncertainty.
	suite.assertError(request.Metadata().ID(), "resize")
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

func (suite *StoragePoolVolumeSuite) TestInheritedResizeMarkerBlocksMatchingCapacityAndRecreatedRequest() {
	pool := suite.pool(true)
	suite.AddFinalizer(pool.Metadata(), storageres.StoragePoolVolumeMutationFinalizer("orphan.raw"))
	suite.client.setVolume("orphan.raw", libvirtstorage.Volume{
		Name: "orphan.raw", Path: "/var/mnt/u-vms/images/orphan.raw", Format: "raw", Capacity: 128 << 20,
	})
	request := suite.spec(volumePool, "orphan.raw", "raw", 128<<20)
	suite.start()
	suite.assertError(request.Metadata().ID(), "requires confirming the daemon is quiescent")
	suite.Require().Empty(suite.client.recorded())
	suite.withdrawSpec(request)
	ctest.AssertNoResource[*storageres.StoragePoolVolumeStatus](suite, request.Metadata().ID())
	recreated := suite.spec(volumePool, "orphan.raw", "raw", 128<<20)
	suite.assertError(recreated.Metadata().ID(), "requires confirming the daemon is quiescent")
	ctest.AssertResource(suite, pool.Metadata().ID(), func(current *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(current.Metadata().Finalizers().Has(storageres.StoragePoolVolumeMutationFinalizer("orphan.raw")))
	})
	suite.Require().Empty(suite.client.recorded())
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
	suite.openDisk("vm1__data.qcow2")
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

	suite.assertError(storageres.StoragePoolVolumeID("nowhere", "vm1__data.qcow2"), "not declared by a StoragePoolConfig document")
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

	disk := suite.openDisk("vm1__data.qcow2")

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
		client.mu.Lock()
		defer client.mu.Unlock()

		return client.opens >= 3
	}, 3*time.Second, 10*time.Millisecond, "daemon-open failure must retry without a new resource event")
	suite.Require().Zero(logs.FilterMessage("controller failed").Len(), "daemon retries must not restart the copy-owning controller")

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

func (c *volumeClient) setReadError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.readErr = err
}

func (suite *StoragePoolVolumeSuite) TestPermanentVolumeReadErrorIsNotAnOutage() {
	suite.pool(true)
	suite.spec(volumePool, "observed.raw", "raw", 64<<20)
	suite.start()
	suite.assertReady("images/observed.raw", nil)

	suite.client.setReadError(errors.New("invalid volume description"))
	suite.spec(volumePool, "wake.raw", "raw", 64<<20)
	ctest.AssertResource(suite, "images/observed.raw", func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseNotReady, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Error, "invalid volume description")
		asrt.Empty(status.TypedSpec().Path)
	})
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

func (suite *StoragePoolVolumeSuite) TestCopyBindingFollowsCurrentDiskIntent() {
	target := suite.T().TempDir()
	key := strings.Repeat("a", 64)
	pool := suite.pool(true)
	ctest.UpdateWithConflicts(suite, pool, func(current *storageres.StoragePoolStatus) error {
		current.TypedSpec().TargetPath = target

		return nil
	}, state.WithUpdateOwner("storage.StoragePoolController"))
	suite.Require().NoError(os.Mkdir(filepath.Join(target, ".copy-on-use"), 0o700))

	for _, name := range []string{"a.raw", "b.raw"} {
		suite.Require().NoError(os.WriteFile(filepath.Join(target, name), []byte("image"), 0o600))
		witness := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
		suite.Require().NoError(os.Link(filepath.Join(target, name), filepath.Join(target, ".copy-on-use", witness)))
		spec := suite.spec(volumePool, name, "raw", 0)
		ctest.UpdateWithConflicts(suite, spec, func(current *storageres.StoragePoolVolumeSpec) error {
			current.TypedSpec().Copy = &storageres.StoragePoolVolumeCopySpec{Source: "/library/object", Digest: "sha256:123"}
			current.Metadata().Labels().Set(libvirtstorage.CopyBindingLabel, key)

			return nil
		}, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController"))
	}

	suite.start()
	suite.assertError(storageres.StoragePoolVolumeID(volumePool, "a.raw"), "awaiting current disk")
	suite.assertCopyPhase(storageres.StoragePoolVolumeID(volumePool, "a.raw"), storageres.StoragePoolVolumeCopyPhaseWaiting)

	disk := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, "vm/install")
	disk.Metadata().Labels().Set(libvirtstorage.CopyBindingLabel, key)
	disk.TypedSpec().Pool = volumePool
	disk.TypedSpec().Volume = "a.raw"
	suite.Create(disk, state.WithCreateOwner("hypervisor.VirtualMachineDiskController"))
	suite.assertReady(storageres.StoragePoolVolumeID(volumePool, "a.raw"), nil)
	ctest.UpdateWithConflicts(suite, disk, func(current *hypervisor.VirtualMachineDiskStatus) error {
		current.TypedSpec().Volume = "b.raw"

		return nil
	}, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController"))
	suite.assertReady(storageres.StoragePoolVolumeID(volumePool, "b.raw"), nil)
	suite.assertCopyPhase(storageres.StoragePoolVolumeID(volumePool, "b.raw"), storageres.StoragePoolVolumeCopyPhaseSucceeded)
	suite.assertError(storageres.StoragePoolVolumeID(volumePool, "a.raw"), "awaiting current disk")
	suite.assertCopyPhase(storageres.StoragePoolVolumeID(volumePool, "a.raw"), storageres.StoragePoolVolumeCopyPhaseWaiting)

	bound, err := copyowner.ReadBinding(target, key)
	suite.Require().NoError(err)
	suite.Equal("b.raw", bound, "late A reconciliation cannot overwrite B's completed binding")

	// Model Record succeeding followed by a failed deferred fence removal.
	// The unchanged-binding path must retry that acknowledgement, not strand it.
	suite.AddFinalizer(disk.Metadata(), libvirtstorage.CopyBindingPublicationFinalizer)
	ctest.AssertResource(suite, disk.Metadata().ID(), func(current *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
		asrt.False(current.Metadata().Finalizers().Has(libvirtstorage.CopyBindingPublicationFinalizer))
	})
	// A withdrawn intent must not strand a fence whose prior removal failed.
	suite.AddFinalizer(disk.Metadata(), libvirtstorage.CopyBindingPublicationFinalizer)
	_, err = suite.State().Teardown(suite.Ctx(), disk.Metadata(), state.WithTeardownOwner("hypervisor.VirtualMachineDiskController"))
	suite.Require().NoError(err)
	ctest.AssertResource(suite, disk.Metadata().ID(), func(current *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, current.Metadata().Phase())
		asrt.False(current.Metadata().Finalizers().Has(libvirtstorage.CopyBindingPublicationFinalizer))
	})

	suite.assertNoRestartLoop()
}

// drainingCopySession acknowledges cancellation only after its stage cleanup is allowed to finish.
type drainingCopySession struct {
	libvirtstorage.VolumeClient

	started  chan struct{}
	canceled chan struct{}
	release  chan struct{}
	once     sync.Once
}

func (client *drainingCopySession) FinishCopy(ctx context.Context, _ libvirtstorage.Pool, _ *libvirtstorage.CopyOperation, _ func() error) (libvirtstorage.Volume, error) {
	client.once.Do(func() { close(client.started) })
	<-ctx.Done()
	close(client.canceled)
	<-client.release

	return libvirtstorage.Volume{}, ctx.Err()
}

func (suite *StoragePoolVolumeSuite) TestCopyWithdrawalRetainsHoldsUntilWorkerDrains() {
	pool := suite.pool(true)
	spec := suite.spec(volumePool, "draining.raw", "raw", 0)
	current, err := safe.StateGetByID[*storageres.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), spec.Metadata().ID())
	suite.Require().NoError(err)

	current.TypedSpec().Copy = &storageres.StoragePoolVolumeCopySpec{
		Source: "/library/object",
	}
	suite.Require().NoError(suite.State().Update(suite.Ctx(), current, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController")))

	client := &drainingCopySession{
		VolumeClient: suite.client,
		started:      make(chan struct{}),
		canceled:     make(chan struct{}),
		release:      make(chan struct{}),
	}

	var release sync.Once
	defer release.Do(func() { close(client.release) })

	suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.StoragePoolVolumeController{
		Open: func(context.Context) (libvirtstorage.VolumeClient, error) {
			return client, nil
		},
	}))

	select {
	case <-client.started:
	case <-time.After(time.Second):
		suite.FailNow("copy worker did not start")
	}

	_, err = suite.State().Teardown(suite.Ctx(), spec.Metadata(), state.WithTeardownOwner("hypervisor.VirtualMachineDiskController"))
	suite.Require().NoError(err)

	select {
	case <-client.canceled:
	case <-time.After(time.Second):
		suite.FailNow("copy withdrawal did not cancel the worker")
	}

	ctx, st := suite.Ctx(), suite.State()
	suite.Require().Never(func() bool {
		observed, readErr := safe.StateGetByID[*storageres.StoragePoolStatus](ctx, st, pool.Metadata().ID())

		return readErr != nil || !observed.Metadata().Finalizers().Has(volumeFinalizer)
	}, 100*time.Millisecond, time.Millisecond, "pool must stay held until copy cleanup acknowledges cancellation")

	observed, err := safe.StateGetByID[*storageres.StoragePoolVolumeSpec](ctx, st, spec.Metadata().ID())
	suite.Require().NoError(err)
	suite.Require().True(observed.Metadata().Finalizers().Has(volumeFinalizer), "request must stay held while worker drains")

	release.Do(func() { close(client.release) })
	suite.assertPoolHold(false)
	suite.withdrawSpec(spec)
}

func (suite *StoragePoolVolumeSuite) TestCopyAdmissionRotatesAcrossPools() {
	suite.pool(true)

	other := storageres.NewStoragePoolStatus(storageres.NamespaceName, "other")
	*other.TypedSpec() = storageres.StoragePoolStatusSpec{
		Phase:      storageres.StoragePoolPhaseReady,
		TargetPath: "/var/mnt/other",
		VolumeID:   "u-other",
	}
	suite.Create(other, state.WithCreateOwner("storage.StoragePoolController"))

	first := suite.copySpec("first.raw")
	second := suite.copySpec("second.raw")
	third := storageres.NewStoragePoolVolumeSpec(storageres.NamespaceName, "other/only.raw")
	*third.TypedSpec() = storageres.StoragePoolVolumeSpecSpec{
		Pool:   "other",
		Name:   "only.raw",
		Format: "raw",
		Copy:   &storageres.StoragePoolVolumeCopySpec{Source: "/library/object"},
	}
	suite.Create(third, state.WithCreateOwner("hypervisor.VirtualMachineDiskController"))
	suite.start()

	for _, request := range []*storageres.StoragePoolVolumeSpec{first, second, third} {
		suite.assertReady(request.Metadata().ID(), nil)
	}

	suite.Require().Equal([]string{
		"copy:images/first.raw:/library/object:",
		"copy:other/only.raw:/library/object:",
		"copy:images/second.raw:/library/object:",
	}, suite.client.recorded(), "a pool with queued work gets its turn before the previous pool drains its backlog")
}

func (suite *StoragePoolVolumeSuite) TestCopyWorkerBoundsAdmissionAndKeepsReconciliationResponsive() {
	suite.pool(true)

	second := storageres.NewStoragePoolStatus(storageres.NamespaceName, "other")
	second.TypedSpec().Phase = storageres.StoragePoolPhaseReady
	second.TypedSpec().TargetPath = "/var/mnt/other"
	second.TypedSpec().VolumeID = "u-other"
	suite.Create(second, state.WithCreateOwner("storage.StoragePoolController"))

	for i := range 6 {
		pool := []string{volumePool, "other"}[i%2]

		spec := suite.spec(pool, fmt.Sprintf("bounded-%d.raw", i), "raw", 0)
		current, err := safe.StateGetByID[*storageres.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), spec.Metadata().ID())
		suite.Require().NoError(err)

		current.TypedSpec().Copy = &storageres.StoragePoolVolumeCopySpec{Source: "/library/object"}
		suite.Require().NoError(suite.State().Update(suite.Ctx(), current, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController")))
	}

	suite.client.copyStarted = make(chan struct{}, 1)
	release := make(chan struct{})

	suite.client.copyRelease = release

	var unblock sync.Once
	defer unblock.Do(func() { close(release) })

	suite.start()

	select {
	case <-suite.client.copyStarted:
	case <-time.After(time.Second):
		suite.FailNow("copy worker did not start")
	}

	suite.spec(volumePool, "responsive.raw", "raw", 4096)
	suite.assertReady(storageres.StoragePoolVolumeID(volumePool, "responsive.raw"), nil)
	ctx, st := suite.Ctx(), suite.State()
	suite.Require().Eventually(func() bool {
		statuses, err := safe.StateListAll[*storageres.StoragePoolVolumeStatus](ctx, st)
		if err != nil {
			return false
		}

		counts := map[storageres.StoragePoolVolumeCopyPhase]int{}

		for status := range statuses.All() {
			if observation := status.TypedSpec().Copy; observation != nil {
				counts[observation.Phase]++
			}
		}

		return counts[storageres.StoragePoolVolumeCopyPhaseQueued] == 5 && counts[storageres.StoragePoolVolumeCopyPhaseRunning] == 1
	}, time.Second, time.Millisecond, "one running copy and five observable queued requests")

	copies := func() int {
		suite.client.mu.Lock()
		defer suite.client.mu.Unlock()

		return suite.client.copyPreparations
	}()

	suite.Require().Equal(1, copies, "only one copy operation may enter the worker")
	unblock.Do(func() { close(release) })

	for i := range 6 {
		pool := []string{volumePool, "other"}[i%2]

		suite.assertCopyPhase(storageres.StoragePoolVolumeID(pool, fmt.Sprintf("bounded-%d.raw", i)), storageres.StoragePoolVolumeCopyPhaseSucceeded)
	}
}

type observedCopyController struct {
	*storagectrl.StoragePoolVolumeController
	run func(context.Context, controller.Runtime, *zap.Logger) error
}

func (ctrl *observedCopyController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	return ctrl.run(ctx, r, logger)
}

func (suite *StoragePoolVolumeSuite) TestCopyWorkerShutdownJoinsCleanup() {
	suite.pool(true)
	spec := suite.spec(volumePool, "shutdown.raw", "raw", 0)
	current, err := safe.StateGetByID[*storageres.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), spec.Metadata().ID())
	suite.Require().NoError(err)

	current.TypedSpec().Copy = &storageres.StoragePoolVolumeCopySpec{Source: "/library/object"}
	suite.Require().NoError(suite.State().Update(suite.Ctx(), current, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController")))
	client := &drainingCopySession{
		VolumeClient: suite.client,
		started:      make(chan struct{}),
		canceled:     make(chan struct{}),
		release:      make(chan struct{}),
	}

	var release sync.Once
	defer release.Do(func() { close(client.release) })

	ctrl := &storagectrl.StoragePoolVolumeController{Open: func(context.Context) (libvirtstorage.VolumeClient, error) { return client, nil }}
	stop := make(chan context.CancelFunc, 1)
	exited := make(chan struct{})

	suite.Require().NoError(suite.Runtime().RegisterController(&observedCopyController{
		StoragePoolVolumeController: ctrl,
		run: func(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
			workerCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			stop <- cancel

			err := ctrl.Run(workerCtx, r, logger)

			close(exited)

			return err
		},
	}))

	select {
	case <-client.started:
	case <-time.After(time.Second):
		suite.FailNow("copy worker did not start")
	}

	cancel := <-stop
	cancel()

	select {
	case <-client.canceled:
	case <-time.After(time.Second):
		suite.FailNow("shutdown did not cancel copy worker")
	}

	select {
	case <-exited:
		suite.FailNow("Run returned before worker cleanup acknowledged cancellation")
	default:
	}

	release.Do(func() { close(client.release) })

	select {
	case <-exited:
	case <-time.After(time.Second):
		suite.FailNow("Run did not join the completed worker")
	}
}

func (suite *StoragePoolVolumeSuite) heldCopy(name string) (*storageres.StoragePoolVolumeSpec, context.Context, func()) {
	return suite.heldCopyWithSpec(name, false)
}

func (suite *StoragePoolVolumeSuite) heldCopyWithSpec(name string, image bool) (*storageres.StoragePoolVolumeSpec, context.Context, func()) {
	suite.pool(true)
	spec := suite.spec(volumePool, name, "raw", 0)
	current, err := safe.StateGetByID[*storageres.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), spec.Metadata().ID())
	suite.Require().NoError(err)

	current.TypedSpec().Copy = &storageres.StoragePoolVolumeCopySpec{Source: "/library/object"}
	if image {
		current.TypedSpec().Copy.Image = true
		current.TypedSpec().Capacity = 8
	}

	suite.Require().NoError(suite.State().Update(suite.Ctx(), current, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController")))
	suite.client.copyStarted = make(chan struct{}, 1)
	suite.client.copyContexts = make(chan context.Context, 1)
	release := make(chan struct{})
	suite.client.copyRelease = release

	var once sync.Once

	unblock := func() { once.Do(func() { close(release) }) }
	suite.T().Cleanup(unblock)
	suite.start()

	select {
	case copyCtx := <-suite.client.copyContexts:
		return spec, copyCtx, unblock
	case <-time.After(time.Second):
		suite.FailNow("copy worker did not start")
	}

	return nil, nil, nil
}

func (suite *StoragePoolVolumeSuite) TestCopyMetadataChangesDoNotRestartTransfer() {
	spec, copyCtx, release := suite.heldCopy("metadata.raw")
	current, err := safe.StateGetByID[*storageres.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), spec.Metadata().ID())
	suite.Require().NoError(err)
	current.Metadata().Labels().Set("test-metadata-change", "true")
	suite.Require().NoError(suite.State().Update(suite.Ctx(), current, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController")))
	suite.spec(volumePool, "metadata-observer.raw", "raw", 4096)
	suite.assertReady(storageres.StoragePoolVolumeID(volumePool, "metadata-observer.raw"), nil)
	suite.Require().NoError(copyCtx.Err(), "metadata-only changes must not cancel an unchanged copy intent")
	release()
	suite.assertReady(spec.Metadata().ID(), nil)
}

func (suite *StoragePoolVolumeSuite) TestCopySurvivesUnrelatedVolumeFailure() {
	spec, copyCtx, release := suite.heldCopy("uninterrupted.raw")
	suite.client.setErrors(nil, errors.New("unrelated blank-volume failure"), nil)
	failed := suite.spec(volumePool, "failing.raw", "raw", 4096)
	suite.assertError(failed.Metadata().ID(), "unrelated blank-volume failure")
	suite.Require().Never(func() bool { return copyCtx.Err() != nil }, 100*time.Millisecond, time.Millisecond, "unrelated per-spec failures must not cancel valid active copies")
	release()
	suite.assertReady(spec.Metadata().ID(), nil)
	suite.client.setErrors(nil, nil, nil)
	suite.assertReady(failed.Metadata().ID(), nil)
}

func (suite *StoragePoolVolumeSuite) TestCopyIntentChangesStillCancelOldTransfer() {
	spec, copyCtx, release := suite.heldCopy("changed.raw")
	current, err := safe.StateGetByID[*storageres.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), spec.Metadata().ID())
	suite.Require().NoError(err)

	current.TypedSpec().Copy.Source = "/library/new-object"
	suite.Require().NoError(suite.State().Update(suite.Ctx(), current, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController")))

	select {
	case <-copyCtx.Done():
	case <-time.After(time.Second):
		suite.FailNow("copy intent change did not cancel obsolete transfer")
	}

	release()
	suite.assertReady(spec.Metadata().ID(), nil)
	suite.Require().Contains(suite.client.recorded(), "copy:"+volumePool+"/changed.raw:/library/new-object:")
	suite.Require().NotContains(suite.client.recorded(), "copy:"+volumePool+"/changed.raw:/library/object:")
}

func (suite *StoragePoolVolumeSuite) copySpec(name string) *storageres.StoragePoolVolumeSpec {
	spec := storageres.NewStoragePoolVolumeSpec(storageres.NamespaceName, storageres.StoragePoolVolumeID(volumePool, name))
	*spec.TypedSpec() = storageres.StoragePoolVolumeSpecSpec{
		Pool: volumePool, Name: name, Format: "raw",
		Copy: &storageres.StoragePoolVolumeCopySpec{Source: "/library/object"},
	}
	suite.Create(spec, state.WithCreateOwner("hypervisor.VirtualMachineDiskController"))

	return spec
}

func (suite *StoragePoolVolumeSuite) assertCopyPhase(id string, phase storageres.StoragePoolVolumeCopyPhase) {
	suite.Require().EventuallyWithT(func(t *assert.CollectT) {
		asrt := assert.New(t)

		status, err := safe.StateGetByID[*storageres.StoragePoolVolumeStatus](suite.Ctx(), suite.State(), id)
		if !asrt.NoError(err) || !asrt.NotNil(status.TypedSpec().Copy) {
			return
		}

		asrt.Equal(phase, status.TypedSpec().Copy.Phase)

		current, err := safe.StateGetByID[*storageres.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), id)
		if !asrt.NoError(err) {
			return
		}

		pool, err := safe.StateGetByID[*storageres.StoragePoolStatus](suite.Ctx(), suite.State(), current.TypedSpec().Pool)
		if asrt.NoError(err) {
			asrt.Equal(storageres.StoragePoolVolumeCopyInitializationIntent(current, pool), status.TypedSpec().Copy.Intent)
		}
	}, 5*time.Second, 10*time.Millisecond)
}

func (suite *StoragePoolVolumeSuite) TestCopyCancellationKeepsSlotAndNeverReportsFailureBeforeDrain() {
	suite.pool(true)
	first := suite.copySpec("a-cancel.raw")
	client := &drainingCopySession{
		VolumeClient: suite.client, started: make(chan struct{}),
		canceled: make(chan struct{}), release: make(chan struct{}),
	}

	var release sync.Once
	defer release.Do(func() { close(client.release) })

	suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.StoragePoolVolumeController{
		Open: func(context.Context) (libvirtstorage.VolumeClient, error) { return client, nil },
	}))

	select {
	case <-client.started:
	case <-time.After(time.Second):
		suite.FailNow("worker did not start")
	}

	suite.assertCopyPhase(first.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseRunning)
	suite.changeCopySpec(first, func(current *storageres.StoragePoolVolumeSpec) {
		current.TypedSpec().Copy.Source = "/library/replacement"
	})

	select {
	case <-client.canceled:
	case <-time.After(time.Second):
		suite.FailNow("worker did not cancel")
	}

	second := suite.copySpec("b-wait.raw")
	suite.assertCopyPhase(first.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseCancelling)
	suite.assertCopyPhase(second.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseQueued)
	suite.client.mu.Lock()
	count := suite.client.copyPreparations
	suite.client.mu.Unlock()
	suite.Equal(1, count, "cancellation keeps admission slot until cleanup")
	// Withdraw both before releasing the fake worker, which models only one attempt.
	_, err := suite.State().Teardown(suite.Ctx(), first.Metadata(), state.WithTeardownOwner("hypervisor.VirtualMachineDiskController"))
	suite.Require().NoError(err)
	_, err = suite.State().Teardown(suite.Ctx(), second.Metadata(), state.WithTeardownOwner("hypervisor.VirtualMachineDiskController"))
	suite.Require().NoError(err)
	release.Do(func() { close(client.release) })
	suite.assertPoolHold(false)
}

type retryCopyClient struct {
	*volumeClient
	attempts chan string
	failure  error
}

func (client *retryCopyClient) FinishCopy(ctx context.Context, pool libvirtstorage.Pool, op *libvirtstorage.CopyOperation, validate func() error) (libvirtstorage.Volume, error) {
	client.mu.Lock()
	name := client.copyName
	failure := client.failure
	client.mu.Unlock()

	select {
	case client.attempts <- name:
	default:
	}

	if name == "a-retry.raw" && failure != nil {
		return libvirtstorage.Volume{}, failure
	}

	return client.volumeClient.FinishCopy(ctx, pool, op, validate)
}

func (suite *StoragePoolVolumeSuite) TestCopyFailedAttemptRetriesWithoutStarvingOtherRequests() {
	suite.pool(true)
	first := suite.copySpec("a-retry.raw")
	second := suite.copySpec("b-success.raw")
	invalid := suite.copySpec("c-invalid.raw")
	suite.changeCopySpec(invalid, func(current *storageres.StoragePoolVolumeSpec) { current.TypedSpec().Capacity = 1 })
	client := &retryCopyClient{volumeClient: suite.client, attempts: make(chan string, 20), failure: errors.New("temporary daemon failure")}
	suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.StoragePoolVolumeController{
		Open: func(context.Context) (libvirtstorage.VolumeClient, error) { return client, nil },
	}))
	suite.assertCopyPhase(first.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseFailed)
	suite.assertCopyPhase(second.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseSucceeded)
	suite.assertCopyPhase(invalid.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseFailed)

	for _, expected := range []string{"a-retry.raw", "b-success.raw", "a-retry.raw"} {
		select {
		case name := <-client.attempts:
			suite.Equal(expected, name)
		case <-time.After(3 * time.Second):
			suite.FailNow("completion and retry must wake without input events")
		}
	}

	client.mu.Lock()
	client.failure = nil
	client.mu.Unlock()
	suite.assertCopyPhase(first.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseSucceeded)
}

func (suite *StoragePoolVolumeSuite) TestCopyImportExitBacksOffAndRecoversAfterSpaceReturns() {
	suite.pool(true)
	request := suite.copySpec("a-retry.raw")
	suite.changeCopySpec(request, func(current *storageres.StoragePoolVolumeSpec) {
		current.TypedSpec().Capacity = 8
		current.TypedSpec().Copy.Image = true
	})
	client := &retryCopyClient{
		volumeClient: suite.client,
		attempts:     make(chan string, 10),
		failure: &qemuimg.ExecError{
			ExitCode: 1,
			Stderr:   []byte("No space left on device"),
		},
	}
	suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.StoragePoolVolumeController{
		Open: func(context.Context) (libvirtstorage.VolumeClient, error) {
			return client, nil
		},
	}))

	for range 2 {
		select {
		case name := <-client.attempts:
			suite.Equal("a-retry.raw", name)
		case <-time.After(3 * time.Second):
			suite.FailNow("retryable copy never retried")
		}
	}

	suite.assertCopyPhase(request.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseFailed)
	suite.assertError(request.Metadata().ID(), "No space left on device")

	select {
	case <-client.attempts:
		suite.FailNow("second failed import must not retry after one second")
	case <-time.After(1200 * time.Millisecond):
	}

	status, err := safe.StateGetByID[*storageres.StoragePoolVolumeStatus](suite.Ctx(), suite.State(), request.Metadata().ID())
	suite.Require().NoError(err)
	suite.Equal(storageres.StoragePoolVolumeCopyPhaseFailed, status.TypedSpec().Copy.Phase, "status must not flap during cooldown")
	client.mu.Lock()
	client.failure = nil
	client.mu.Unlock()
	suite.assertCopyPhase(request.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseSucceeded)
}

func (suite *StoragePoolVolumeSuite) changeCopySpec(spec *storageres.StoragePoolVolumeSpec, change func(*storageres.StoragePoolVolumeSpec)) {
	ctest.UpdateWithConflicts(suite, spec, func(current *storageres.StoragePoolVolumeSpec) error {
		change(current)

		return nil
	}, state.WithUpdateOwner("hypervisor.VirtualMachineDiskController"))
}

// forceDestroy destroys a resource behind the controller's back. A controller
// retrying through an outage can re-add its finalizer between the removal and
// the destroy, so both are repeated until the destroy wins.
func (suite *StoragePoolVolumeSuite) forceDestroy(ptr resource.Pointer, owner string) {
	suite.Require().EventuallyWithT(func(t *assert.CollectT) {
		if !assert.NoError(t, suite.State().RemoveFinalizer(suite.Ctx(), ptr, volumeFinalizer)) {
			return
		}

		assert.NoError(t, suite.State().Destroy(suite.Ctx(), ptr, state.WithDestroyOwner(owner)))
	}, 2*time.Second, 10*time.Millisecond)
}

func (suite *StoragePoolVolumeSuite) TestCopyBackingChangeRejectsOldCompletion() {
	spec, copyCtx, release := suite.heldCopy("backing.raw")
	pool, err := safe.StateGetByID[*storageres.StoragePoolStatus](suite.Ctx(), suite.State(), volumePool)
	suite.Require().NoError(err)
	ctest.UpdateWithConflicts(suite, pool, func(current *storageres.StoragePoolStatus) error {
		current.TypedSpec().VolumeID = "replacement"
		current.TypedSpec().TargetPath = "/replacement"

		return nil
	}, state.WithUpdateOwner("storage.StoragePoolController"))

	select {
	case <-copyCtx.Done():
	case <-time.After(time.Second):
		suite.FailNow("old backing transfer not canceled")
	}

	release()
	suite.assertCopyPhase(spec.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseSucceeded)
	suite.client.mu.Lock()
	attempts := suite.client.copyPreparations
	suite.client.mu.Unlock()
	suite.Equal(2, attempts, "new backing must get a new attempt")
}

func (suite *StoragePoolVolumeSuite) TestCopyIncarnationChangeRejectsOldCompletion() {
	spec, copyCtx, release := suite.heldCopy("incarnation.raw")
	current, err := safe.StateGetByID[*storageres.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), spec.Metadata().ID())
	suite.Require().NoError(err)
	// Deliberately bypass the producer's normal drain protocol to exercise the
	// independent incarnation fence against a stale worker.
	suite.Require().NoError(suite.State().RemoveFinalizer(suite.Ctx(), current.Metadata(), volumeFinalizer))
	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), current.Metadata(), state.WithDestroyOwner("hypervisor.VirtualMachineDiskController")))
	replacement := storageres.NewStoragePoolVolumeSpec(storageres.NamespaceName, spec.Metadata().ID())
	*replacement.TypedSpec() = *current.TypedSpec()
	suite.Create(replacement, state.WithCreateOwner("hypervisor.VirtualMachineDiskController"))
	suite.NotEqual(current.Metadata().Created(), replacement.Metadata().Created())

	select {
	case <-copyCtx.Done():
	case <-time.After(time.Second):
		suite.FailNow("old incarnation transfer not canceled")
	}

	release()
	suite.assertCopyPhase(spec.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseSucceeded)
}

// publicationRecoveryClient models published bytes followed by a lost daemon
// observation. Recovery must adopt those bytes, never start another transfer.
type publicationRecoveryClient struct {
	*volumeClient
	failAdoption bool
	finished     chan struct{}
}

func (client *publicationRecoveryClient) FinishCopy(ctx context.Context, pool libvirtstorage.Pool, op *libvirtstorage.CopyOperation, validate func() error) (libvirtstorage.Volume, error) {
	volume, err := client.volumeClient.FinishCopy(ctx, pool, op, validate)
	client.mu.Lock()
	client.failAdoption = true
	client.mu.Unlock()
	close(client.finished)

	return volume, err
}

func (client *publicationRecoveryClient) AdoptCopy(pool libvirtstorage.Pool, name string, cache *libvirtstorage.CopyCache) (libvirtstorage.Volume, bool, error) {
	client.mu.Lock()
	fail := client.failAdoption
	client.mu.Unlock()

	if fail {
		return libvirtstorage.Volume{}, false, libvirtstorage.ErrVolumeObservationUnavailable
	}

	return client.volumeClient.AdoptCopy(pool, name, cache)
}

func (suite *StoragePoolVolumeSuite) TestCopyPublicationRecoveryDoesNotReportFailureOrRecopy() {
	suite.pool(true)
	spec := suite.copySpec("recover.raw")
	client := &publicationRecoveryClient{volumeClient: suite.client, finished: make(chan struct{})}
	suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.StoragePoolVolumeController{
		Open: func(context.Context) (libvirtstorage.VolumeClient, error) { return client, nil },
	}))

	select {
	case <-client.finished:
	case <-time.After(time.Second):
		suite.FailNow("copy did not finish")
	}

	suite.assertCopyPhase(spec.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseWaiting)
	client.mu.Lock()
	client.failAdoption = false
	client.mu.Unlock()
	// No resource event: the scheduled retry must recover the observation.
	suite.assertCopyPhase(spec.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseSucceeded)
	client.mu.Lock()
	attempts := client.copyPreparations
	client.mu.Unlock()
	suite.Equal(1, attempts)
}

func (suite *StoragePoolVolumeSuite) TestCopyRestartAdoptsBeforeRequiringSource() {
	suite.pool(true)
	spec := suite.copySpec("retained.raw")
	suite.changeCopySpec(spec, func(current *storageres.StoragePoolVolumeSpec) { current.TypedSpec().Copy.Source = "" })
	suite.client.setVolume("retained.raw", libvirtstorage.Volume{Name: "retained.raw", Path: "/pool/retained.raw", Format: "raw", Capacity: 4096})

	stale := storageres.NewStoragePoolVolumeStatus(storageres.NamespaceName, spec.Metadata().ID())
	stale.TypedSpec().Pool, stale.TypedSpec().Name = volumePool, "retained.raw"
	stale.TypedSpec().Copy = &storageres.StoragePoolVolumeCopyStatusSpec{Phase: storageres.StoragePoolVolumeCopyPhaseRunning, Intent: "old-process"}
	suite.Create(stale, state.WithCreateOwner(volumeFinalizer))
	suite.start()
	suite.assertCopyPhase(spec.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseSucceeded)
	suite.client.mu.Lock()
	attempts := suite.client.copyPreparations
	suite.client.mu.Unlock()
	suite.Zero(attempts)
}

func (suite *StoragePoolVolumeSuite) TestWritableImageQualificationPreservesDeferredGrowthReason() {
	suite.pool(true)
	suite.client.setVolume("guest.img", libvirtstorage.Volume{
		Name: "guest.img", Path: "/var/mnt/u-vms/images/guest.img", Format: "raw", Capacity: 8,
	})
	suite.openDisk("guest.img")
	request := suite.spec(volumePool, "guest.img", "raw", 16)
	suite.changeCopySpec(request, func(current *storageres.StoragePoolVolumeSpec) {
		current.TypedSpec().Copy = &storageres.StoragePoolVolumeCopySpec{Image: true}
		current.Metadata().Labels().Set(libvirtstorage.CopyBindingLabel, "missing-disk-intent")
	})
	suite.start()
	ctest.AssertResource(suite, request.Metadata().ID(), func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseNotReady, status.TypedSpec().Phase)
		asrt.EqualValues(16, status.TypedSpec().PendingCapacity)
		asrt.Contains(status.TypedSpec().Error, "detach all consumers to apply growth")
		asrt.Contains(status.TypedSpec().Error, "awaiting current disk binding intent")
	})
	suite.Require().Empty(suite.client.recorded())
}

func (suite *StoragePoolVolumeSuite) TestWritableImageGrowthDoesNotLogInitialization() {
	suite.pool(true)
	suite.client.setVolume("guest.img", libvirtstorage.Volume{
		Name: "guest.img", Path: "/var/mnt/u-vms/images/guest.img", Format: "raw", Capacity: 8,
	})
	request := suite.spec(volumePool, "guest.img", "raw", 8)
	suite.changeCopySpec(request, func(current *storageres.StoragePoolVolumeSpec) {
		current.TypedSpec().Copy = &storageres.StoragePoolVolumeCopySpec{Image: true}
	})
	suite.start()
	suite.assertReady(request.Metadata().ID(), nil)
	suite.Require().Eventually(func() bool {
		return suite.logs.FilterMessage("copy provisioning succeeded").Len() == 1
	}, time.Second, time.Millisecond)
	suite.changeCopySpec(request, func(current *storageres.StoragePoolVolumeSpec) { current.TypedSpec().Capacity = 16 })
	suite.assertReady(request.Metadata().ID(), func(status *storageres.StoragePoolVolumeStatusSpec, asrt *assert.Assertions) {
		asrt.EqualValues(16, status.Capacity)
	})
	logs := suite.logs
	suite.Require().Never(func() bool {
		return logs.FilterMessage("copy provisioning succeeded").Len() != 1
	}, 100*time.Millisecond, time.Millisecond, "growth did not initialize or copy a disk")
}

func (suite *StoragePoolVolumeSuite) TestWritableImportMissingToolDoesNotRetry() {
	input, err := os.CreateTemp(suite.T().TempDir(), "raw")
	suite.Require().NoError(err)

	defer input.Close() //nolint:errcheck

	_, err = input.WriteString("raw image")
	suite.Require().NoError(err)
	_, toolErr := qemuimg.New(filepath.Join(suite.T().TempDir(), "missing-qemu-img")).Info(suite.Ctx(), input)
	suite.Require().Error(toolErr)
	suite.testWritableImportFailure(toolErr, true)
}

func (suite *StoragePoolVolumeSuite) TestWritableImportChildExitRetriesAfterSpaceRecovered() {
	// Inject the library boundary's child error; the real controller owns worker
	// completion, retry deadlines, proof invalidation, and eventual publication.
	suite.testWritableImportFailure(&qemuimg.ExecError{ExitCode: 1, Stderr: []byte("No space left on device")}, false)
}

func (suite *StoragePoolVolumeSuite) TestWritableImportValidationDoesNotRetry() {
	input, err := os.CreateTemp(suite.T().TempDir(), "invalid-qcow2")
	suite.Require().NoError(err)

	defer input.Close() //nolint:errcheck

	_, err = input.WriteString("QFI\xfb")
	suite.Require().NoError(err)

	_, validationErr := qemuimg.Inspect(input)
	suite.Require().Error(validationErr)
	suite.testWritableImportFailure(validationErr, true)
}

func (suite *StoragePoolVolumeSuite) testWritableImportFailure(failure error, terminal bool) {
	suite.pool(true)
	request := suite.copySpec("a-retry.raw")
	suite.changeCopySpec(request, func(current *storageres.StoragePoolVolumeSpec) {
		current.TypedSpec().Capacity = 8
		current.TypedSpec().Copy.Image = true
	})
	client := &retryCopyClient{volumeClient: suite.client, attempts: make(chan string, 20), failure: failure}
	suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.StoragePoolVolumeController{
		Open: func(context.Context) (libvirtstorage.VolumeClient, error) { return client, nil },
	}))
	suite.assertCopyPhase(request.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseFailed)

	select {
	case <-client.attempts:
	case <-time.After(time.Second):
		suite.FailNow("worker did not report its first attempt")
	}

	if terminal {
		suite.Require().Never(func() bool { return len(client.attempts) != 0 }, 1300*time.Millisecond, 10*time.Millisecond,
			"missing tool must remain dormant until an input change")

		return
	}

	select {
	case <-client.attempts:
	case <-time.After(3 * time.Second):
		suite.FailNow("child exit must retry without an input event")
	}

	client.mu.Lock()
	client.failure = nil
	client.mu.Unlock()
	suite.assertCopyPhase(request.Metadata().ID(), storageres.StoragePoolVolumeCopyPhaseSucceeded)
}

func (suite *StoragePoolVolumeSuite) TestWritableCapacityEditCancelsInitializationWorker() {
	request, copyCtx, release := suite.heldCopyWithSpec("growing.img", true)
	pool, err := safe.StateGetByID[*storageres.StoragePoolStatus](suite.Ctx(), suite.State(), volumePool)
	suite.Require().NoError(err)
	before, err := safe.StateGetByID[*storageres.StoragePoolVolumeSpec](suite.Ctx(), suite.State(), request.Metadata().ID())
	suite.Require().NoError(err)

	initialization := storageres.StoragePoolVolumeCopyInitializationIntent(before, pool)

	suite.changeCopySpec(request, func(current *storageres.StoragePoolVolumeSpec) { current.TypedSpec().Capacity = 16 })

	select {
	case <-copyCtx.Done():
	case <-time.After(time.Second):
		suite.FailNow("desired capacity must still fence an active initialization worker")
	}

	release()
	suite.assertReady(request.Metadata().ID(), func(status *storageres.StoragePoolVolumeStatusSpec, asrt *assert.Assertions) {
		asrt.EqualValues(16, status.Capacity)

		if asrt.NotNil(status.Copy) {
			asrt.Equal(initialization, status.Copy.Intent, "growth leaves initialization identity unchanged")
		}
	})
}
