// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

const poolExclusion = "storage.StoragePoolController/mutating/backing"

// A marker surviving a controller crash has no completion receipt. Neither a
// missing request/definition nor a Ready status matching inventory proves that
// the daemon has stopped an operation which outlived the client session.
func TestPoolOrphanExclusionRequiresQuiescedRecovery(t *testing.T) {
	t.Parallel()

	fixture := &StoragePoolSuite{Timeout: 10 * time.Second}
	fixture.SetT(t)
	fixture.SetupTest()

	defer fixture.TearDownTest()

	fixture.mount("u-vms")
	status := storageres.NewStoragePoolStatus(storageres.NamespaceName, "images")
	status.TypedSpec().VolumeID = "u-vms"
	status.TypedSpec().Phase = storageres.StoragePoolPhaseReady
	fixture.Create(status, state.WithCreateOwner(poolFinalizer))
	fixture.AddFinalizer(status.Metadata(), poolExclusion)
	fixture.Destroy(fixture.spec)
	fixture.start()

	ctest.AssertResource(fixture, "images", func(current *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseRunning, current.Metadata().Phase())
		asrt.True(current.Metadata().Finalizers().Has(poolExclusion))
	})

	recreated := storageres.NewStoragePoolSpec(storageres.NamespaceName, "images")
	recreated.TypedSpec().VolumeID = "u-vms"
	fixture.Create(recreated)
	fixture.assertError("uncertain prior mutation")
	fixture.Require().Empty(fixture.client.completedEvents())
}

func TestPoolReadyInventoryDoesNotClearOrphanExclusion(t *testing.T) {
	t.Parallel()

	fixture := &StoragePoolSuite{Timeout: 10 * time.Second}
	fixture.SetT(t)
	fixture.SetupTest()

	defer fixture.TearDownTest()

	fixture.mount("u-vms")
	fixture.start()
	fixture.assertReady("u-vms")
	fixture.waitForEvent("ensure:" + filepath.Join(fixture.root, "u-vms", "images"))
	ctest.AssertResource(fixture, "images", func(current *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.False(current.Metadata().Finalizers().Has(poolExclusion))
	})
	status, err := safe.StateGetByID[*storageres.StoragePoolStatus](fixture.Ctx(), fixture.State(), "images")
	fixture.Require().NoError(err)
	fixture.AddFinalizer(status.Metadata(), poolExclusion)

	fixture.volume("x-wakeup")
	fixture.mount("x-wakeup")
	fixture.assertError("uncertain prior mutation")
	ctest.AssertResource(fixture, "images", func(current *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(current.Metadata().Finalizers().Has(poolExclusion))
	})
}

func TestPoolOutagePreservesWithdrawnMutationExclusion(t *testing.T) {
	t.Parallel()

	fixture := &StoragePoolSuite{Timeout: 10 * time.Second}
	fixture.SetT(t)
	fixture.SetupTest()
	defer fixture.TearDownTest()

	fixture.mount("u-vms")
	status := storageres.NewStoragePoolStatus(storageres.NamespaceName, "images")
	status.TypedSpec().VolumeID = "u-vms"
	fixture.Create(status, state.WithCreateOwner(poolFinalizer))
	fixture.AddFinalizer(status.Metadata(), poolExclusion)
	fixture.Destroy(fixture.spec)

	other := storageres.NewStoragePoolSpec(storageres.NamespaceName, "other")
	other.TypedSpec().VolumeID = "u-vms"
	fixture.Create(other)
	fixture.client.setErrors(errors.New("daemon unavailable"), nil)
	fixture.start()
	ctest.AssertResource(fixture, "other", func(current *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.Contains(current.TypedSpec().Error, "daemon unavailable")
	})
	ctest.AssertResource(fixture, "images", func(current *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(current.Metadata().Finalizers().Has(poolExclusion))
	})
}

// retargetTransportClient accepts an operation while its reply is lost. The
// daemon-side work remains behind release, independent of the client session.
type retargetTransportClient struct {
	*poolClient
	newTarget string
	entered   chan struct{}
	release   chan struct{}
	finished  chan struct{}
	once      sync.Once
}

func (c *retargetTransportClient) Ensure(pool libvirtstorage.Pool, target string, prepare func() error) error {
	if target != c.newTarget {
		return c.poolClient.Ensure(pool, target, prepare)
	}

	c.once.Do(func() {
		close(c.entered)

		go func() {
			defer close(c.finished)
			<-c.release
			c.mu.Lock()
			defer c.mu.Unlock()

			pool.Target = target
			for i := range c.pools {
				if c.pools[i].Name == pool.Name {
					c.pools[i] = pool
				}
			}
		}()
	})

	return errors.New("transport disconnected after daemon accepted ensure")
}

func TestPoolUncertainRetargetSurvivesRevertAndLateDaemonCompletion(t *testing.T) {
	t.Parallel()

	fixture := &StoragePoolSuite{Timeout: 10 * time.Second}
	fixture.SetT(t)
	fixture.SetupTest()

	defer fixture.TearDownTest()

	fixture.mount("u-vms")
	fixture.volume("x-new")
	fixture.mount("x-new")
	client := &retargetTransportClient{
		poolClient: fixture.client,
		newTarget:  filepath.Join(fixture.root, "x-new", "images"),
		entered:    make(chan struct{}),
		release:    make(chan struct{}),
		finished:   make(chan struct{}),
	}
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(client.release) }) }
	t.Cleanup(func() {
		unblock()
		select {
		case <-client.entered:
			<-client.finished
		default:
		}
	})
	fixture.Require().NoError(fixture.Runtime().RegisterController(&storagectrl.StoragePoolController{
		Open: func(_ context.Context) (libvirtstorage.Client, error) { return client, nil },
	}))
	fixture.assertReady("u-vms")
	fixture.setVolumeID("x-new")
	select {
	case <-client.entered:
	case <-fixture.Ctx().Done():
		fixture.Require().FailNow("retarget operation was not submitted")
	}
	fixture.assertError("transport disconnected")
	fixture.setVolumeID("u-vms")
	fixture.assertError("uncertain prior mutation")
	ctest.AssertResource(fixture, "images", func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Has(poolExclusion))
	})
	before := fixture.client.completedEvents()
	unblock()
	<-client.finished
	fixture.volume("wakeup")
	fixture.mount("wakeup")
	fixture.assertError("uncertain prior mutation")
	fixture.Require().Equal(before, fixture.client.completedEvents(), "no operation may be submitted after an uncertain mutation")
}

// A Ready volume was produced under a pool hold. During retarget the pool
// becomes pending, and the volume controller may release that hold while its
// old Ready volume observation has not yet been updated. A VM can pin that
// volume in this interval: pool retarget must census the volume itself.
func TestPoolRetargetDefersForConsumerVolumeWithoutPoolHold(t *testing.T) {
	t.Parallel()

	fixture := &StoragePoolSuite{Timeout: 10 * time.Second}
	fixture.SetT(t)
	fixture.SetupTest()

	defer fixture.TearDownTest()

	fixture.mount("u-vms")
	fixture.volume("x-new")
	fixture.mount("x-new")
	oldTarget := filepath.Join(fixture.root, "u-vms", "images")
	fixture.client.pools = []libvirtstorage.Pool{{
		Name:   "images",
		UUID:   libvirtstorage.UUID(uuid.MustParse(poolMachineUUID), "images"),
		Target: oldTarget,
	}}
	status := storageres.NewStoragePoolStatus(storageres.NamespaceName, "images")
	status.TypedSpec().VolumeID = "u-vms"
	status.TypedSpec().TargetPath = oldTarget
	status.TypedSpec().Phase = storageres.StoragePoolPhaseReady
	fixture.Create(status, state.WithCreateOwner(poolFinalizer))
	fixture.AddFinalizer(status.Metadata(), consumerFinalizer)

	volume := storageres.NewStoragePoolVolumeStatus(storageres.NamespaceName, "images/attached.raw")
	volume.TypedSpec().Pool = "images"
	volume.TypedSpec().Name = "attached.raw"
	volume.TypedSpec().Phase = storageres.StoragePoolVolumePhaseReady
	fixture.Create(volume, state.WithCreateOwner(consumerFinalizer))
	fixture.setVolumeID("x-new")
	ctest.UpdateWithConflicts(fixture, status, func(current *storageres.StoragePoolStatus) error {
		current.TypedSpec().Phase = storageres.StoragePoolPhaseNotReady

		return nil
	}, state.WithUpdateOwner(poolFinalizer))
	fixture.RemoveFinalizer(status.Metadata(), consumerFinalizer)
	fixture.AddFinalizer(volume.Metadata(), "hypervisor.VirtualMachineController/vm/system")
	fixture.start()
	fixture.assertError("held by a consumer")
	fixture.Require().NotContains(fixture.client.completedEvents(), "ensure:"+filepath.Join(fixture.root, "x-new", "images"))
	fixture.RemoveFinalizer(volume.Metadata(), "hypervisor.VirtualMachineController/vm/system")
	fixture.assertReady("x-new")
}
