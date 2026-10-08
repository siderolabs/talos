// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

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
		asrt.Equal(resource.PhaseRunning, current.Metadata().Phase())
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

					break
				}
			}
		}()
	})

	return errors.New("transport disconnected after daemon accepted ensure")
}

func (c *retargetTransportClient) EnsureOperation(pool libvirtstorage.Pool, target string, prepare func() error) (libvirtstorage.OperationOutcome, error) {
	if target != c.newTarget {
		return c.poolClient.EnsureOperation(pool, target, prepare)
	}

	return libvirtstorage.Unknown, c.Ensure(pool, target, prepare)
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

type removalTransportClient struct {
	*poolClient
	entered  chan struct{}
	release  chan struct{}
	finished chan struct{}
	once     sync.Once
}

func (c *removalTransportClient) Remove(pool libvirtstorage.Pool) error {
	c.once.Do(func() {
		close(c.entered)

		go func() {
			defer close(c.finished)

			<-c.release
			c.mu.Lock()
			defer c.mu.Unlock()

			for i, existing := range c.pools {
				if existing.Name == pool.Name {
					c.pools = append(c.pools[:i], c.pools[i+1:]...)

					break
				}
			}

			delete(c.active, pool.Name)
		}()
	})

	return errors.New("transport disconnected after daemon accepted remove")
}

func (c *removalTransportClient) RemoveOperation(pool libvirtstorage.Pool) (libvirtstorage.OperationOutcome, error) {
	return libvirtstorage.Unknown, c.Remove(pool)
}

func TestPoolUncertainRemovalSurvivesRecreationAndLateCompletion(t *testing.T) {
	t.Parallel()

	fixture := &StoragePoolSuite{Timeout: 10 * time.Second}
	fixture.SetT(t)

	fixture.SetupTest()
	defer fixture.TearDownTest()

	fixture.mount("u-vms")
	client := &removalTransportClient{
		poolClient: fixture.client,
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
	fixture.Destroy(fixture.spec)

	select {
	case <-client.entered:
	case <-fixture.Ctx().Done():
		fixture.Require().FailNow("removal operation was not submitted")
	}

	ctest.AssertResource(fixture, "images", func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Has(poolExclusion))
	})

	recreated := storageres.NewStoragePoolSpec(storageres.NamespaceName, "images")
	recreated.TypedSpec().VolumeID = "u-vms"
	fixture.Create(recreated)
	fixture.assertError("uncertain prior mutation")
	fixture.assertHold("u-vms", true)
	before := fixture.client.completedEvents()

	unblock()
	<-client.finished
	fixture.volume("x-wakeup")

	fixture.mount("x-wakeup")
	fixture.assertError("uncertain prior mutation")
	fixture.Require().Equal(before, fixture.client.completedEvents(), "late removal must not admit a new definition")
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

// poolFinishedRetarget arms the publication fault only after a real Finished Ensure.
type poolFinishedRetarget struct {
	*poolClient
	target   string
	finished atomic.Bool
}

func (c *poolFinishedRetarget) EnsureOperation(pool libvirtstorage.Pool, target string, prepare func() error) (libvirtstorage.OperationOutcome, error) {
	outcome, err := c.poolClient.EnsureOperation(pool, target, prepare)
	if target == c.target && outcome == libvirtstorage.Finished && err == nil {
		c.finished.Store(true)
	}

	return outcome, err
}

type poolPublicationController struct {
	controller.Controller
	client  *poolFinishedRetarget
	entered chan struct{}
	release chan struct{}
	result  chan error
	once    sync.Once
}

type poolPublicationRuntime struct {
	controller.Runtime
	observer *poolPublicationController
}

func (r poolPublicationRuntime) Modify(ctx context.Context, empty resource.Resource, update func(resource.Resource) error, options ...controller.ModifyOption) error {
	if empty.Metadata().Type() == storageres.StoragePoolStatusType && empty.Metadata().ID() == "images" && r.observer.client.finished.Load() {
		intercepted := false

		r.observer.once.Do(func() {
			intercepted = true

			close(r.observer.entered)

			select {
			case <-r.observer.release:
			case <-ctx.Done():
			}
		})

		if intercepted {
			return errors.New("pool status publication rejected")
		}
	}

	return r.Runtime.Modify(ctx, empty, update, options...)
}

func (c *poolPublicationController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	err := c.Controller.Run(ctx, poolPublicationRuntime{Runtime: r, observer: c}, logger)
	c.result <- err

	return err
}

func TestPoolFinishedRetargetPublicationFailureRetainsExclusion(t *testing.T) {
	t.Parallel()

	fixture := &StoragePoolSuite{Timeout: 10 * time.Second}
	fixture.SetT(t)

	fixture.SetupTest()
	defer fixture.TearDownTest()

	fixture.mount("u-vms")
	fixture.volume("x-new")
	fixture.mount("x-new")
	target := filepath.Join(fixture.root, "x-new", "images")
	client := &poolFinishedRetarget{poolClient: fixture.client, target: target}
	entered, release := make(chan struct{}), make(chan struct{})
	result := make(chan error, 1)

	var releaseOnce sync.Once

	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()

	fixture.Require().NoError(fixture.Runtime().RegisterController(&poolPublicationController{
		Controller: &storagectrl.StoragePoolController{
			Open: func(context.Context) (libvirtstorage.Client, error) { return client, nil },
		},
		client:  client,
		entered: entered,
		release: release,
		result:  result,
	}))
	fixture.assertReady("u-vms")
	fixture.setVolumeID("x-new")

	select {
	case <-entered:
	case <-fixture.Ctx().Done():
		fixture.Require().FailNow("finished retarget did not reach status publication")
	}

	fixture.Require().True(client.finished.Load(), "daemon operation must finish before the publication fault")

	ctx, st := fixture.Ctx(), fixture.State()
	status, err := safe.StateGetByID[*storageres.StoragePoolStatus](ctx, st, "images")
	fixture.Require().NoError(err)
	fixture.Require().Equal(storageres.StoragePoolPhaseReady, status.TypedSpec().Phase, "old Ready observation remains until publication")
	fixture.Require().Equal("u-vms", status.TypedSpec().VolumeID)
	fixture.Require().True(status.Metadata().Finalizers().Has(poolExclusion), "exclusion must cover stale Ready observation")

	volumeClient := &volumeClient{checkHold: func(pool string) error {
		current, getErr := safe.StateGetByID[*storageres.StoragePoolStatus](ctx, st, pool)
		if getErr != nil {
			return getErr
		}

		if !current.Metadata().Finalizers().Has(volumeFinalizer) {
			return errors.New("volume operation without pool hold")
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
	ctest.AssertResource(fixture, request.Metadata().ID(), func(current *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.NotEqual(storageres.StoragePoolVolumePhaseReady, current.TypedSpec().Phase, "stale Ready pool must not admit a volume during failed publication")
	})
	fixture.Require().Empty(volumeClient.recorded(), "no volume may be created against stale Ready backing")
	unblock()

	select {
	case publicationErr := <-result:
		fixture.Require().ErrorContains(publicationErr, "pool status publication rejected")
	case <-ctx.Done():
		fixture.Require().FailNow("status publication fault was not returned")
	}

	ctest.AssertResource(fixture, "images", func(current *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(current.Metadata().Finalizers().Has(poolExclusion), "failed publication must retain exclusion")
	})
	fixture.Require().Empty(volumeClient.recorded(), "failed publication must not admit the stale Ready volume")
}

type poolUnknownNilEnsure struct {
	*poolClient
	target string
}

func (c *poolUnknownNilEnsure) EnsureOperation(pool libvirtstorage.Pool, target string, prepare func() error) (libvirtstorage.OperationOutcome, error) {
	if target == c.target {
		return libvirtstorage.Unknown, nil
	}

	return c.poolClient.EnsureOperation(pool, target, prepare)
}

// poolFirstFailure pauses restart so assertions observe the first pass, not a later retry.
type poolFirstFailure struct {
	controller.Controller
	failed  chan error
	release chan struct{}
	once    sync.Once
}

func (c *poolFirstFailure) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	err := c.Controller.Run(ctx, r, logger)
	c.once.Do(func() {
		c.failed <- err

		select {
		case <-c.release:
		case <-ctx.Done():
		}
	})

	return err
}

func TestPoolUnknownNilEnsureCannotPublishReady(t *testing.T) {
	t.Parallel()

	fixture := &StoragePoolSuite{Timeout: 10 * time.Second}
	fixture.SetT(t)

	fixture.SetupTest()
	defer fixture.TearDownTest()

	fixture.mount("u-vms")
	fixture.volume("x-new")
	fixture.mount("x-new")
	client := &poolUnknownNilEnsure{
		poolClient: fixture.client,
		target:     filepath.Join(fixture.root, "x-new", "images"),
	}

	gate := &poolFirstFailure{
		Controller: &storagectrl.StoragePoolController{
			Open: func(context.Context) (libvirtstorage.Client, error) { return client, nil },
		},
		failed:  make(chan error, 1),
		release: make(chan struct{}),
	}
	defer close(gate.release)

	fixture.Require().NoError(fixture.Runtime().RegisterController(gate))
	fixture.assertReady("u-vms")
	fixture.setVolumeID("x-new")

	select {
	case err := <-gate.failed:
		fixture.Require().ErrorContains(err, "outcome unknown")
	case <-fixture.Ctx().Done():
		fixture.Require().FailNow("unknown outcome did not fail the controller pass")
	}

	current, err := safe.StateGetByID[*storageres.StoragePoolStatus](fixture.Ctx(), fixture.State(), "images")
	fixture.Require().NoError(err)
	fixture.Require().NotEqual(storageres.StoragePoolPhaseReady, current.TypedSpec().Phase)
	fixture.Require().True(current.Metadata().Finalizers().Has(poolExclusion))
	fixture.Require().NotContains(fixture.client.completedEvents(), "ensure:"+client.target)
}

// poolCensusFailure injects a transient COSI read failure after a new exclusion,
// before any daemon operation is submitted.
type poolCensusFailure struct {
	controller.Controller
	enabled atomic.Bool
	armed   atomic.Bool
}

type poolCensusRuntime struct {
	controller.Runtime
	fault *poolCensusFailure
}

func (r poolCensusRuntime) AddFinalizer(ctx context.Context, pointer resource.Pointer, finalizers ...resource.Finalizer) error {
	if err := r.Runtime.AddFinalizer(ctx, pointer, finalizers...); err != nil {
		return err
	}

	if r.fault.enabled.Load() && pointer.Type() == storageres.StoragePoolStatusType && len(finalizers) == 1 && finalizers[0] == poolExclusion {
		r.fault.armed.Store(true)
	}

	return nil
}

func (r poolCensusRuntime) List(ctx context.Context, kind resource.Kind, options ...state.ListOption) (resource.List, error) {
	if kind.Type() == storageres.StoragePoolVolumeStatusType && r.fault.armed.CompareAndSwap(true, false) {
		return resource.List{}, errors.New("transient volume census failure")
	}

	return r.Runtime.List(ctx, kind, options...)
}

func (c *poolCensusFailure) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	return c.Controller.Run(ctx, poolCensusRuntime{Runtime: r, fault: c}, logger)
}

func TestPoolPreSubmissionCensusFailureReleasesOwnExclusionAndRetries(t *testing.T) {
	t.Parallel()

	fixture := &StoragePoolSuite{Timeout: 10 * time.Second}
	fixture.SetT(t)

	fixture.SetupTest()
	defer fixture.TearDownTest()

	fixture.mount("u-vms")
	fixture.volume("x-new")
	fixture.mount("x-new")
	fault := &poolCensusFailure{Controller: &storagectrl.StoragePoolController{Open: fixture.client.open}}
	gate := &poolFirstFailure{
		Controller: fault,
		failed:     make(chan error, 1),
		release:    make(chan struct{}),
	}

	var releaseOnce sync.Once

	unblock := func() { releaseOnce.Do(func() { close(gate.release) }) }
	defer unblock()

	fixture.Require().NoError(fixture.Runtime().RegisterController(gate))
	fixture.assertReady("u-vms")
	ctest.AssertResource(fixture, "images", func(status *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.False(status.Metadata().Finalizers().Has(poolExclusion))
	})
	fault.enabled.Store(true)
	fixture.setVolumeID("x-new")

	select {
	case err := <-gate.failed:
		fixture.Require().ErrorContains(err, "transient volume census failure")
	case <-fixture.Ctx().Done():
		fixture.Require().FailNow("post-exclusion census did not fail")
	}

	status, err := safe.StateGetByID[*storageres.StoragePoolStatus](fixture.Ctx(), fixture.State(), "images")
	fixture.Require().NoError(err)
	fixture.Require().False(status.Metadata().Finalizers().Has(poolExclusion), "no operation was submitted, so this pass must release its marker")
	fixture.Require().NotContains(fixture.client.completedEvents(), "ensure:"+filepath.Join(fixture.root, "x-new", "images"))
	fault.enabled.Store(false)
	unblock()
	fixture.assertReady("x-new")
	fixture.waitForEvent("ensure:" + filepath.Join(fixture.root, "x-new", "images"))
}

// poolMarkerBarrier pauses after publishing exclusion so a consumer can claim
// the old observation before the controller's final decision.
type poolMarkerBarrier struct {
	controller.Controller
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

type poolMarkerRuntime struct {
	controller.Runtime
	barrier *poolMarkerBarrier
}

func (r poolMarkerRuntime) AddFinalizer(ctx context.Context, pointer resource.Pointer, finalizers ...resource.Finalizer) error {
	if err := r.Runtime.AddFinalizer(ctx, pointer, finalizers...); err != nil {
		return err
	}

	if pointer.Type() == storageres.StoragePoolStatusType && len(finalizers) == 1 && finalizers[0] == poolExclusion {
		r.barrier.once.Do(func() {
			close(r.barrier.entered)

			select {
			case <-r.barrier.release:
			case <-ctx.Done():
			}
		})
	}

	return nil
}

func (b *poolMarkerBarrier) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	return b.Controller.Run(ctx, poolMarkerRuntime{Runtime: r, barrier: b}, logger)
}

func TestPoolInactiveSameBackingDefersActivationForLateConsumer(t *testing.T) {
	t.Parallel()

	fixture := &StoragePoolSuite{Timeout: 10 * time.Second}
	fixture.SetT(t)

	fixture.SetupTest()
	defer fixture.TearDownTest()

	mount := fixture.mount("u-vms")
	fixture.AddFinalizer(mount.Metadata(), poolFinalizer)
	target := filepath.Join(fixture.root, "u-vms", "images")
	status := storageres.NewStoragePoolStatus(storageres.NamespaceName, "images")
	status.TypedSpec().VolumeID = "u-vms"
	status.TypedSpec().TargetPath = target
	status.TypedSpec().Phase = storageres.StoragePoolPhaseReady
	fixture.Create(status, state.WithCreateOwner(poolFinalizer))
	fixture.client.pools = []libvirtstorage.Pool{{
		Name:       "images",
		UUID:       libvirtstorage.UUID(uuid.MustParse(poolMachineUUID), "images"),
		Target:     target,
		Type:       "dir",
		Persistent: true,
	}}
	barrier := &poolMarkerBarrier{
		Controller: &storagectrl.StoragePoolController{Open: fixture.client.open},
		entered:    make(chan struct{}),
		release:    make(chan struct{}),
	}

	var once sync.Once

	unblock := func() { once.Do(func() { close(barrier.release) }) }
	defer unblock()

	fixture.Require().NoError(fixture.Runtime().RegisterController(barrier))

	select {
	case <-barrier.entered:
	case <-fixture.Ctx().Done():
		fixture.Require().FailNow("inactive pool was not excluded before activation")
	}

	fixture.AddFinalizer(status.Metadata(), consumerFinalizer)
	unblock()

	select {
	case <-fixture.client.changed:
	case <-fixture.Ctx().Done():
		fixture.Require().FailNow("controller did not complete deferred activation")
	}

	current, err := safe.StateGetByID[*storageres.StoragePoolStatus](fixture.Ctx(), fixture.State(), "images")
	fixture.Require().NoError(err)
	fixture.Require().NotEqual(storageres.StoragePoolPhaseReady, current.TypedSpec().Phase, "inactive held pool must not be advertised Ready")
	fixture.Require().Contains(current.TypedSpec().Error, "held by a consumer")
	fixture.Require().NotContains(fixture.client.completedEvents(), "ensure:"+target)
	fixture.RemoveFinalizer(status.Metadata(), consumerFinalizer)
	fixture.assertReady("u-vms")
	fixture.waitForEvent("ensure:" + target)
}

type stoppedRemovalClient struct {
	*poolClient
	stopped chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *stoppedRemovalClient) RemoveOperation(pool libvirtstorage.Pool) (libvirtstorage.OperationOutcome, error) {
	first := false

	c.once.Do(func() { first = true })

	if !first {
		return c.poolClient.RemoveOperation(pool)
	}

	if err := c.poolClient.Stop(pool); err != nil {
		return libvirtstorage.NoMutationSubmitted, err
	}

	close(c.stopped)
	<-c.release

	return libvirtstorage.Finished, errors.New("terminal error after stopping pool")
}

type poolReleaseBarrier struct {
	controller.Controller
	entered chan struct{}
	release chan struct{}
	armed   atomic.Bool
	once    sync.Once
}

type poolReleaseRuntime struct {
	controller.Runtime
	barrier *poolReleaseBarrier
}

func (r poolReleaseRuntime) RemoveFinalizer(ctx context.Context, pointer resource.Pointer, finalizers ...resource.Finalizer) error {
	if r.barrier.armed.Load() && pointer.Type() == storageres.StoragePoolStatusType && len(finalizers) == 1 && finalizers[0] == poolExclusion {
		r.barrier.once.Do(func() {
			close(r.barrier.entered)

			select {
			case <-r.barrier.release:
			case <-ctx.Done():
			}
		})
	}

	return r.Runtime.RemoveFinalizer(ctx, pointer, finalizers...)
}

func (b *poolReleaseBarrier) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	return b.Controller.Run(ctx, poolReleaseRuntime{Runtime: r, barrier: b}, logger)
}

func TestPoolFinishedRemovalErrorPublishesNonReadyBeforeUnfencing(t *testing.T) {
	t.Parallel()

	fixture := &StoragePoolSuite{Timeout: 10 * time.Second}
	fixture.SetT(t)

	fixture.SetupTest()
	defer fixture.TearDownTest()

	fixture.mount("u-vms")
	client := &stoppedRemovalClient{
		poolClient: fixture.client,
		stopped:    make(chan struct{}),
		release:    make(chan struct{}),
	}
	barrier := &poolReleaseBarrier{
		Controller: &storagectrl.StoragePoolController{Open: func(context.Context) (libvirtstorage.Client, error) { return client, nil }},
		entered:    make(chan struct{}),
		release:    make(chan struct{}),
	}

	var clientOnce, markerOnce sync.Once

	defer clientOnce.Do(func() { close(client.release) })
	defer markerOnce.Do(func() { close(barrier.release) })

	fixture.Require().NoError(fixture.Runtime().RegisterController(barrier))
	fixture.assertReady("u-vms")
	ctest.AssertResource(fixture, "images", func(current *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.False(current.Metadata().Finalizers().Has(poolExclusion))
	})
	barrier.armed.Store(true)
	fixture.Destroy(fixture.spec)

	select {
	case <-client.stopped:
	case <-fixture.Ctx().Done():
		fixture.Require().FailNow("removal did not stop the pool")
	}

	ctest.AssertResource(fixture, "images", func(current *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.True(current.Metadata().Finalizers().Has(poolExclusion))
	})
	clientOnce.Do(func() { close(client.release) })

	select {
	case <-barrier.entered:
	case <-fixture.Ctx().Done():
		fixture.Require().FailNow("finished removal did not reach marker release")
	}

	current, err := safe.StateGetByID[*storageres.StoragePoolStatus](fixture.Ctx(), fixture.State(), "images")
	fixture.Require().NoError(err)
	fixture.Require().NotEqual(storageres.StoragePoolPhaseReady, current.TypedSpec().Phase, "stopped pool must be non-Ready before exclusion is released")
	fixture.Require().Contains(current.TypedSpec().Error, "terminal error after stopping pool")
	fixture.Require().True(current.Metadata().Finalizers().Has(poolExclusion))
	fixture.assertHold("u-vms", true)
	markerOnce.Do(func() { close(barrier.release) })
	fixture.waitForEvent("remove:images")
}

type stopFaultClient struct {
	*poolClient
	outcome  libvirtstorage.OperationOutcome
	nilError bool
	called   atomic.Bool
}

func (c *stopFaultClient) StopOperation(pool libvirtstorage.Pool) (libvirtstorage.OperationOutcome, error) {
	if c.called.CompareAndSwap(false, true) {
		if c.outcome == libvirtstorage.Finished {
			if err := c.poolClient.Stop(pool); err != nil {
				return libvirtstorage.NoMutationSubmitted, err
			}
		}

		if c.nilError {
			return c.outcome, nil
		}

		return c.outcome, errors.New("injected stop failure")
	}

	return c.poolClient.StopOperation(pool)
}

type stopPublicationFault struct {
	controller.Controller
	client *stopFaultClient
	fail   atomic.Bool
}

type stopPublicationRuntime struct {
	controller.Runtime
	fault *stopPublicationFault
}

func (r stopPublicationRuntime) Modify(ctx context.Context, empty resource.Resource, update func(resource.Resource) error, options ...controller.ModifyOption) error {
	if empty.Metadata().Type() == storageres.StoragePoolStatusType && r.fault.client.called.Load() && r.fault.fail.CompareAndSwap(true, false) {
		return errors.New("stop status publication rejected")
	}

	return r.Runtime.Modify(ctx, empty, update, options...)
}

func (c *stopPublicationFault) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	return c.Controller.Run(ctx, stopPublicationRuntime{Runtime: r, fault: c}, logger)
}

func TestPoolStopFailureOutcomeAndPublicationMatrix(t *testing.T) {
	for _, tc := range []struct {
		name            string
		outcome         libvirtstorage.OperationOutcome
		nilError        bool
		failPublication bool
		marker          bool
	}{
		{name: "finished publication rejected", outcome: libvirtstorage.Finished, failPublication: true, marker: true},
		{name: "unknown", outcome: libvirtstorage.Unknown, marker: true},
		{name: "unknown without error", outcome: libvirtstorage.Unknown, nilError: true, marker: true},
		{name: "not submitted publication rejected", outcome: libvirtstorage.NoMutationSubmitted, failPublication: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := &StoragePoolSuite{Timeout: 10 * time.Second}
			fixture.SetT(t)

			fixture.SetupTest()
			defer fixture.TearDownTest()

			oldMount := fixture.mount("u-vms")
			fixture.volume("x-new")
			newMount := fixture.mount("x-new")
			fixture.AddFinalizer(oldMount.Metadata(), poolFinalizer)
			fixture.AddFinalizer(newMount.Metadata(), poolFinalizer)
			fixture.setVolumeID("x-new")

			target := filepath.Join(fixture.root, "u-vms", "images")
			status := storageres.NewStoragePoolStatus(storageres.NamespaceName, "images")
			status.TypedSpec().VolumeID = "u-vms"
			status.TypedSpec().TargetPath = target
			status.TypedSpec().Phase = storageres.StoragePoolPhaseReady
			fixture.Create(status, state.WithCreateOwner(poolFinalizer))
			fixture.client.pools = []libvirtstorage.Pool{{
				Name:       "images",
				UUID:       libvirtstorage.UUID(uuid.MustParse(poolMachineUUID), "images"),
				Target:     target,
				Type:       "dir",
				Active:     true,
				Persistent: true,
			}}
			fixture.client.active = map[string]struct{}{"images": {}}

			client := &stopFaultClient{poolClient: fixture.client, outcome: tc.outcome, nilError: tc.nilError}
			fault := &stopPublicationFault{
				Controller: &storagectrl.StoragePoolController{
					Open: func(context.Context) (libvirtstorage.Client, error) { return client, nil },
				},
				client: client,
			}
			fault.fail.Store(tc.failPublication)
			gate := &poolFirstFailure{
				Controller: fault,
				failed:     make(chan error, 1),
				release:    make(chan struct{}),
			}

			var once sync.Once

			unblock := func() { once.Do(func() { close(gate.release) }) }
			defer unblock()

			fixture.Require().NoError(fixture.Runtime().RegisterController(gate))

			select {
			case err := <-gate.failed:
				if tc.nilError {
					fixture.Require().ErrorContains(err, "stop outcome unknown")
				} else {
					fixture.Require().ErrorContains(err, "injected stop failure")
				}

				if tc.failPublication {
					fixture.Require().ErrorContains(err, "stop status publication rejected")
				}
			case <-fixture.Ctx().Done():
				fixture.Require().FailNow("stop failure did not return")
			}

			current, err := safe.StateGetByID[*storageres.StoragePoolStatus](fixture.Ctx(), fixture.State(), "images")
			fixture.Require().NoError(err)
			fixture.Require().Equal(tc.marker, current.Metadata().Finalizers().Has(poolExclusion))

			if tc.failPublication {
				fixture.Require().Equal(storageres.StoragePoolPhaseReady, current.TypedSpec().Phase, "failed publication leaves old observation behind the exclusion")
			} else {
				fixture.Require().Equal(storageres.StoragePoolPhaseNotReady, current.TypedSpec().Phase)

				if tc.nilError {
					fixture.Require().Contains(current.TypedSpec().Error, "stop outcome unknown")
				} else {
					fixture.Require().Contains(current.TypedSpec().Error, "injected stop failure")
				}
			}

			fixture.assertHold("u-vms", true)
			fixture.assertHold("x-new", true)
			fixture.Require().NotContains(fixture.client.completedEvents(), "ensure:"+filepath.Join(fixture.root, "x-new", "images"))
			unblock()

			if tc.outcome == libvirtstorage.NoMutationSubmitted {
				fixture.assertReady("x-new")
			} else {
				fixture.assertError("uncertain prior mutation")
			}
		})
	}
}

// terminalStopClient stops the first pool normally, then returns an acknowledged
// terminal error for the second without leaving a daemon operation in flight.
type terminalStopClient struct {
	*poolClient
	fault atomic.Bool
}

func (c *terminalStopClient) StopOperation(pool libvirtstorage.Pool) (libvirtstorage.OperationOutcome, error) {
	if pool.Name == "other" && c.fault.CompareAndSwap(true, false) {
		if err := c.poolClient.Stop(pool); err != nil {
			return libvirtstorage.NoMutationSubmitted, err
		}

		return libvirtstorage.Finished, errors.New("terminal stop error")
	}

	return c.poolClient.StopOperation(pool)
}

func TestPoolFinishedStopErrorPublishesBeforeUnfencingAndRetries(t *testing.T) {
	t.Parallel()

	fixture := &StoragePoolSuite{Timeout: 10 * time.Second}
	fixture.SetT(t)

	fixture.SetupTest()
	defer fixture.TearDownTest()

	oldMount := fixture.mount("u-vms")
	fixture.volume("x-new")
	fixture.mount("x-new")
	fixture.AddFinalizer(oldMount.Metadata(), poolFinalizer)

	other := storageres.NewStoragePoolSpec(storageres.NamespaceName, "other")
	other.TypedSpec().VolumeID = "x-new"
	fixture.Create(other)
	fixture.setVolumeID("x-new")

	for _, name := range []string{"images", "other"} {
		target := filepath.Join(fixture.root, "u-vms", name)
		status := storageres.NewStoragePoolStatus(storageres.NamespaceName, name)
		status.TypedSpec().VolumeID = "u-vms"
		status.TypedSpec().TargetPath = target
		status.TypedSpec().Phase = storageres.StoragePoolPhaseReady
		fixture.Create(status, state.WithCreateOwner(poolFinalizer))
		fixture.client.pools = append(fixture.client.pools, libvirtstorage.Pool{
			Name:       name,
			UUID:       libvirtstorage.UUID(uuid.MustParse(poolMachineUUID), name),
			Target:     target,
			Type:       "dir",
			Active:     true,
			Persistent: true,
		})
	}

	fixture.client.active = map[string]struct{}{"images": {}, "other": {}}
	client := &terminalStopClient{poolClient: fixture.client}
	client.fault.Store(true)

	gate := &poolFirstFailure{
		Controller: &storagectrl.StoragePoolController{
			Open: func(context.Context) (libvirtstorage.Client, error) { return client, nil },
		},
		failed:  make(chan error, 1),
		release: make(chan struct{}),
	}

	var once sync.Once

	unblock := func() { once.Do(func() { close(gate.release) }) }
	defer unblock()

	fixture.Require().NoError(fixture.Runtime().RegisterController(gate))

	select {
	case err := <-gate.failed:
		fixture.Require().ErrorContains(err, "terminal stop error")
	case <-fixture.Ctx().Done():
		fixture.Require().FailNow("terminal stop error did not return")
	}

	for _, name := range []string{"images", "other"} {
		status, err := safe.StateGetByID[*storageres.StoragePoolStatus](fixture.Ctx(), fixture.State(), name)
		fixture.Require().NoError(err)
		fixture.Require().NotEqual(storageres.StoragePoolPhaseReady, status.TypedSpec().Phase, name)
		fixture.Require().False(status.Metadata().Finalizers().Has(poolExclusion), "finished stop %q must retire its own marker after publication", name)
	}

	status, err := safe.StateGetByID[*storageres.StoragePoolStatus](fixture.Ctx(), fixture.State(), "other")
	fixture.Require().NoError(err)
	fixture.Require().Contains(status.TypedSpec().Error, "terminal stop error")
	fixture.assertHold("u-vms", true)
	fixture.Require().NotContains(fixture.client.completedEvents(), "ensure:"+filepath.Join(fixture.root, "x-new", "other"))

	unblock()
	fixture.assertReady("x-new")
	ctest.AssertResource(fixture, "other", func(current *storageres.StoragePoolStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolPhaseReady, current.TypedSpec().Phase, current.TypedSpec().Error)
		asrt.False(current.Metadata().Finalizers().Has(poolExclusion))
	})
	fixture.assertHold("u-vms", false)
}
