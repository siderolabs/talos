// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/cloudinit"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
	"github.com/siderolabs/talos/pkg/machinery/storagehelpers"
)

const cloudInitOwners = ".cloud-init-owners"

// CloudInitISOController builds immutable NoCloud seeds directly in storage pools.
type CloudInitISOController struct{}

// Name implements controller.Controller.
func (ctrl *CloudInitISOController) Name() string {
	return "hypervisor.CloudInitISOController"
}

// Inputs implements controller.Controller.
func (ctrl *CloudInitISOController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.CloudInitSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.CloudInitStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller.
func (ctrl *CloudInitISOController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hypervisor.CloudInitStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller.
func (ctrl *CloudInitISOController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r); err != nil {
			logger.Error("failed to reconcile cloud-init ISOs", zap.Error(err))

			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *CloudInitISOController) reconcile(ctx context.Context, r controller.ReaderWriter) error {
	specs, err := safe.ReaderListAll[*hypervisor.CloudInitSpec](ctx, r)
	if err != nil {
		return err
	}

	pools, err := safe.ReaderListAll[*storage.StoragePoolStatus](ctx, r)
	if err != nil {
		return err
	}

	byPool := map[string]*storage.StoragePoolStatus{}
	for pool := range pools.All() {
		byPool[pool.Metadata().ID()] = pool
	}

	wanted := map[string]struct{}{}

	var errs []error

	for spec := range specs.All() {
		if spec.Metadata().Phase() != resource.PhaseRunning {
			continue
		}

		id := hypervisor.CloudInitStatusID(spec.Metadata().ID(), *spec.TypedSpec())

		wanted[id] = struct{}{}
		if err := ctrl.produce(ctx, r, spec, id, byPool[spec.TypedSpec().Pool]); err != nil {
			errs = append(errs, fmt.Errorf("cloud-init %q: %w", id, err))
			errs = append(errs, ctrl.seedError(ctx, r, spec, id, err.Error()))
		}
	}

	errs = append(errs, ctrl.retireUnusedSeeds(ctx, r, wanted, byPool))

	return errors.Join(errs...)
}

// retireUnusedSeeds withdraws obsolete incarnations, then releases holds on pools with no remaining seeds.
func (ctrl *CloudInitISOController) retireUnusedSeeds(ctx context.Context, r controller.ReaderWriter, wanted map[string]struct{}, byPool map[string]*storage.StoragePoolStatus) error {
	statuses, err := safe.ReaderListAll[*hypervisor.CloudInitStatus](ctx, r)
	if err != nil {
		return err
	}

	var errs []error

	for status := range statuses.All() {
		if _, ok := wanted[status.Metadata().ID()]; ok && status.Metadata().Phase() == resource.PhaseRunning {
			continue
		}

		errs = append(errs, ctrl.retire(ctx, r, status, byPool[status.TypedSpec().Pool]))
	}

	errs = append(errs, ctrl.releaseUnusedPoolHolds(ctx, r, byPool))

	return errors.Join(errs...)
}

// releaseUnusedPoolHolds refreshes statuses after retirement so held or failed publications keep their pools mounted.
func (ctrl *CloudInitISOController) releaseUnusedPoolHolds(ctx context.Context, r controller.ReaderWriter, byPool map[string]*storage.StoragePoolStatus) error {
	remaining, err := safe.ReaderListAll[*hypervisor.CloudInitStatus](ctx, r)
	if err != nil {
		return err
	}

	inUse := map[string]bool{}

	for status := range remaining.All() {
		if status.TypedSpec().Path != "" {
			inUse[status.TypedSpec().Pool] = true
		}
	}

	var errs []error

	for id, pool := range byPool {
		if !inUse[id] && pool.Metadata().Finalizers().Has(ctrl.Name()) {
			errs = append(errs, r.RemoveFinalizer(ctx, pool.Metadata(), ctrl.Name()))
		}
	}

	return errors.Join(errs...)
}

func (ctrl *CloudInitISOController) seedError(ctx context.Context, r controller.ReaderWriter, spec *hypervisor.CloudInitSpec, id, reason string) error {
	return safe.WriterModify(ctx, r, hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, id), func(status *hypervisor.CloudInitStatus) error {
		if status.Metadata().Phase() != resource.PhaseRunning {
			return nil
		}

		if status.TypedSpec().Path == "" {
			*status.TypedSpec() = hypervisor.CloudInitStatusSpec{
				VirtualMachine:     spec.Metadata().ID(),
				Pool:               spec.TypedSpec().Pool,
				InputDigest:        spec.TypedSpec().InputDigest(),
				ObservedGeneration: spec.Metadata().Version().String(),
			}
		}

		status.TypedSpec().Phase = hypervisor.CloudInitPhaseNotReady
		status.TypedSpec().Error = reason

		return nil
	})
}

func cloudInitPoolName(id resource.ID) string {
	return "cloud-init-" + id[strings.LastIndex(id, "@")+1:] + ".iso"
}

// seedPoolMount observes the pinned mount independently of storage daemon availability.
// A mount being withdrawn can still be used for cleanup under the existing pool hold.
func seedPoolMount(ctx context.Context, r controller.Reader, pool *storage.StoragePoolStatus, path, volumeID string) (*block.VolumeMountStatus, error) {
	if pool == nil || volumeID == "" || pool.TypedSpec().VolumeID != volumeID || pool.TypedSpec().TargetPath != path {
		return nil, nil
	}

	mount, err := readSeedPoolMount(ctx, r, volumeID)
	if err != nil || mount == nil {
		return nil, err
	}

	if filepath.Join(mount.TypedSpec().Target, pool.Metadata().ID()) != path {
		return nil, nil
	}

	return mount, nil
}

// readSeedPoolMount observes a writable, attached backing mount with an absolute target.
func readSeedPoolMount(ctx context.Context, r controller.Reader, volumeID string) (*block.VolumeMountStatus, error) {
	mount, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, volumeID)
	if state.IsNotFoundError(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	if mount.TypedSpec().ReadOnly || mount.TypedSpec().Detached || !filepath.IsAbs(mount.TypedSpec().Target) {
		return nil, nil
	}

	return mount, nil
}

func (ctrl *CloudInitISOController) produce(ctx context.Context, r controller.ReaderWriter, spec *hypervisor.CloudInitSpec, id string, pool *storage.StoragePoolStatus) error {
	seed := spec.TypedSpec()
	if err := validateSeedInput(seed); err != nil {
		return err
	}

	old, err := safe.ReaderGetByID[*hypervisor.CloudInitStatus](ctx, r, id)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if old != nil && old.Metadata().Phase() != resource.PhaseRunning {
		return nil
	}

	if seedBackingWithdrawn(old, pool) {
		// A held old backing is deliberately reported unready during retarget.
		// Withdraw admission so consumers detach before retirement releases its pool hold.
		_, err := r.Teardown(ctx, old.Metadata())

		return err
	}

	retained, err := retainSeedDuringPoolOutage(ctx, r, old, pool)
	if err != nil || retained {
		return err
	}

	pool, err = ctrl.prepareSeedBacking(ctx, r, pool, seed.Pool)
	if err != nil {
		return err
	}

	return ctrl.produceSeed(ctx, r, spec, id, old, pool)
}

func validateSeedInput(seed *hypervisor.CloudInitSpecSpec) error {
	if err := storagehelpers.ValidateStoragePoolName(seed.Pool); err != nil {
		return err
	}

	if len(seed.MetaData)+len(seed.UserData)+len(seed.NetworkConfig) > cloudinit.MaxSeedBytes {
		return errors.New("cloud-init seed exceeds size limit")
	}

	return nil
}

func seedBackingWithdrawn(old *hypervisor.CloudInitStatus, pool *storage.StoragePoolStatus) bool {
	return old != nil && old.TypedSpec().Path != "" &&
		(pool == nil || pool.Metadata().Phase() != resource.PhaseRunning || pool.TypedSpec().Phase == storage.StoragePoolPhaseNotReady)
}

// retainSeedDuringPoolOutage preserves only a complete witnessed seed on the same live backing.
func retainSeedDuringPoolOutage(ctx context.Context, r controller.Reader, old *hypervisor.CloudInitStatus, pool *storage.StoragePoolStatus) (bool, error) {
	if old == nil || old.TypedSpec().Digest == "" || pool == nil {
		return false, nil
	}

	if pool.TypedSpec().Phase != storage.StoragePoolPhaseObservationUnavailable || !cloudInitPoolIdentityMatches(pool, old.TypedSpec()) {
		return false, nil
	}

	mount, err := seedPoolMount(ctx, r, pool, old.TypedSpec().Path, old.TypedSpec().VolumeID)
	if err != nil {
		return false, err
	}

	return mount != nil && mount.Metadata().Phase() == resource.PhaseRunning && checkOwnedISO(old.TypedSpec()) == nil, nil
}

// holdSeedPool fences backing mutation before publication can inspect or create files.
func (ctrl *CloudInitISOController) holdSeedPool(ctx context.Context, r controller.ReaderWriter, pool *storage.StoragePoolStatus, poolID string) (*storage.StoragePoolStatus, error) {
	if pool == nil || pool.Metadata().Phase() != resource.PhaseRunning || pool.TypedSpec().Phase != storage.StoragePoolPhaseReady {
		return nil, errors.New("waiting for storage pool to become ready")
	}

	if !pool.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := r.AddFinalizer(ctx, pool.Metadata(), ctrl.Name()); err != nil {
			return nil, err
		}
	}

	// Re-read after taking the hold: the pool mutator publishes its marker before its hold census.
	return safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, r, poolID)
}

// prepareSeedBacking checks before taking a hold, then rechecks under the hold.
func (ctrl *CloudInitISOController) prepareSeedBacking(ctx context.Context, r controller.ReaderWriter, pool *storage.StoragePoolStatus, poolID string) (*storage.StoragePoolStatus, error) {
	if err := checkSeedPoolBacking(ctx, r, pool); err != nil {
		return nil, err
	}

	pool, err := ctrl.holdSeedPool(ctx, r, pool, poolID)
	if err != nil {
		return nil, err
	}

	if err := checkSeedPoolBacking(ctx, r, pool); err != nil {
		return nil, err
	}

	return pool, nil
}

func checkSeedPoolBacking(ctx context.Context, r controller.Reader, pool *storage.StoragePoolStatus) error {
	if pool == nil || pool.Metadata().Phase() != resource.PhaseRunning || pool.TypedSpec().Phase != storage.StoragePoolPhaseReady {
		return errors.New("waiting for storage pool to become ready")
	}

	if pool.Metadata().Finalizers().Has("storage.StoragePoolController/mutating/backing") {
		return errors.New("storage pool backing is changing")
	}

	mount, err := seedPoolMount(ctx, r, pool, pool.TypedSpec().TargetPath, pool.TypedSpec().VolumeID)
	if err != nil {
		return err
	}

	if mount == nil || mount.Metadata().Phase() != resource.PhaseRunning {
		return errors.New("waiting for storage pool backing mount")
	}

	return nil
}

// produceSeed validates a reserved incarnation or builds it before granting admission.
func (ctrl *CloudInitISOController) produceSeed(
	ctx context.Context,
	r controller.ReaderWriter,
	spec *hypervisor.CloudInitSpec,
	id string,
	old *hypervisor.CloudInitStatus,
	pool *storage.StoragePoolStatus,
) error {
	if old != nil && old.TypedSpec().Path != "" {
		if old.TypedSpec().Path != pool.TypedSpec().TargetPath || old.TypedSpec().VolumeID != pool.TypedSpec().VolumeID {
			// A held status cannot be repointed to another mount. Retire its identity
			// only once consumers detach, then reserve on the replacement in the next pass.
			return ctrl.retireReplacedSeed(ctx, r, old)
		}

		if old.TypedSpec().Digest != "" {
			if err := checkOwnedISO(old.TypedSpec()); err != nil {
				// Withdraw admission before retiring a missing or changed incarnation.
				if _, teardownErr := r.Teardown(ctx, old.Metadata()); teardownErr != nil {
					return teardownErr
				}

				return err
			}

			return ctrl.ready(ctx, r, spec, id, old.TypedSpec().Digest, old.TypedSpec().SizeBytes)
		}
	}

	return ctrl.reserveAndBuildSeed(ctx, r, spec, id, old, pool)
}

// reserveAndBuildSeed records the backing identity before publishing and hashing the immutable seed.
func (ctrl *CloudInitISOController) reserveAndBuildSeed(
	ctx context.Context,
	r controller.ReaderWriter,
	spec *hypervisor.CloudInitSpec,
	id string,
	old *hypervisor.CloudInitStatus,
	pool *storage.StoragePoolStatus,
) error {
	seed := spec.TypedSpec()
	name := cloudInitPoolName(id)

	if old == nil || old.TypedSpec().Path == "" {
		// Reserve identity before any durable file, including a failed or interrupted publication.
		if err := safe.WriterModify(ctx, r, hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, id), func(status *hypervisor.CloudInitStatus) error {
			*status.TypedSpec() = hypervisor.CloudInitStatusSpec{
				VirtualMachine:     spec.Metadata().ID(),
				Pool:               seed.Pool,
				Name:               name,
				Path:               pool.TypedSpec().TargetPath,
				VolumeID:           pool.TypedSpec().VolumeID,
				InputDigest:        seed.InputDigest(),
				ObservedGeneration: spec.Metadata().Version().String(),
				Phase:              hypervisor.CloudInitPhaseNotReady,
				Error:              "building cloud-init ISO",
			}

			return nil
		}); err != nil {
			return err
		}
	}

	root, err := os.OpenRoot(pool.TypedSpec().TargetPath)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck

	if err := publishSeed(ctx, root, pool.TypedSpec().TargetPath, name, seed); err != nil {
		return err
	}

	file, err := root.Open(name)
	if err != nil {
		return err
	}

	digest, size, hashErr := hashISO(file)
	if err := errors.Join(hashErr, file.Close()); err != nil {
		return err
	}

	return ctrl.ready(ctx, r, spec, id, digest, size)
}

func (ctrl *CloudInitISOController) ready(ctx context.Context, r controller.ReaderWriter, spec *hypervisor.CloudInitSpec, id, digest string, size uint64) error {
	return safe.WriterModify(ctx, r, hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, id), func(status *hypervisor.CloudInitStatus) error {
		status.TypedSpec().Digest = digest
		status.TypedSpec().SizeBytes = size
		status.TypedSpec().ObservedGeneration = spec.Metadata().Version().String()
		status.TypedSpec().Phase = hypervisor.CloudInitPhaseReady
		status.TypedSpec().Error = ""

		return nil
	})
}

// publishSeed uses a private hard link to prove ownership across failed publication and restarts.
// A matching digest alone never authorizes adoption of an existing file.
func publishSeed(ctx context.Context, root *os.Root, path, name string, seed *hypervisor.CloudInitSpecSpec) error {
	if err := root.Mkdir(cloudInitOwners, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	witness := filepath.Join(cloudInitOwners, name)

	owned, err := sameInode(root, witness, name)
	if err != nil {
		return err
	}

	if owned {
		// A prior attempt may have linked both names but failed a directory sync.
		// Repeat both durability barriers before admitting the recovered publication.
		if err := syncDirectory(root, cloudInitOwners); err != nil {
			return err
		}

		return syncDirectory(root, ".")
	}

	if _, err := root.Lstat(name); err == nil {
		return fmt.Errorf("refusing foreign cloud-init ISO collision %q: %w", name, fs.ErrExist)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	stage, err := prepareSeedWitness(ctx, root, path, witness, seed)
	if stage != "" {
		defer root.Remove(stage) //nolint:errcheck // An unpublished private stage is never adopted.
	}

	if err != nil {
		return err
	}

	return linkSeedPublication(ctx, root, witness, name)
}

// linkSeedPublication makes the private witness durable before linking and syncing the final name.
func linkSeedPublication(ctx context.Context, root *os.Root, witness, name string) error {
	if err := syncDirectory(root, cloudInitOwners); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := root.Link(witness, name); err != nil {
		return err
	}

	return syncDirectory(root, ".")
}

// prepareSeedWitness reuses a regular private witness or links a completely written staged ISO.
// The caller keeps the stage until publication finishes, including failure cleanup.
func prepareSeedWitness(ctx context.Context, root *os.Root, path, witness string, seed *hypervisor.CloudInitSpecSpec) (string, error) {
	if info, err := root.Lstat(witness); err == nil {
		if !info.Mode().IsRegular() {
			return "", errors.New("invalid cloud-init ownership witness")
		}

		return "", nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}

	stage := ".cloud-init-" + rand.Text() + ".staged"
	if err := cloudinit.WriteISO(ctx, filepath.Join(path, stage), cloudinit.Seed{
		MetaData:      seed.MetaData,
		UserData:      seed.UserData,
		NetworkConfig: seed.NetworkConfig,
	}); err != nil {
		return "", err
	}

	if err := ctx.Err(); err != nil {
		return stage, err
	}

	return stage, root.Link(stage, witness)
}

func (ctrl *CloudInitISOController) retire(ctx context.Context, r controller.ReaderWriter, status *hypervisor.CloudInitStatus, pool *storage.StoragePoolStatus) error {
	ready, err := r.Teardown(ctx, status.Metadata())
	if err != nil {
		return err
	}

	if !ready {
		return nil
	}

	spec := status.TypedSpec()
	if spec.Path != "" {
		mount, err := seedPoolMount(ctx, r, pool, spec.Path, spec.VolumeID)
		if err != nil {
			return err
		}

		if mount == nil {
			if pool != nil && pool.TypedSpec().TargetPath == spec.Path && pool.TypedSpec().VolumeID == spec.VolumeID {
				// An unavailable mount is not a retired backing. Keep the identity
				// and pool hold so cleanup can finish when that same mount returns.
				return errors.New("waiting for storage pool backing mount to retire cloud-init ISO")
			}

			// A replacement backing cannot authorize writes to the old path.
			// Preserve its files and withdraw the detached incarnation instead.
			return r.Destroy(ctx, status.Metadata())
		}

		if err := removeRetiredISO(spec); err != nil {
			return err
		}
	}

	return r.Destroy(ctx, status.Metadata())
}

// retireReplacedSeed withdraws admission first and preserves files on the unproven old mount.
func (ctrl *CloudInitISOController) retireReplacedSeed(ctx context.Context, r controller.ReaderWriter, old *hypervisor.CloudInitStatus) error {
	ready, err := r.Teardown(ctx, old.Metadata())
	if err != nil || !ready {
		return err
	}

	return r.Destroy(ctx, old.Metadata())
}

func checkOwnedISO(spec *hypervisor.CloudInitStatusSpec) error {
	root, err := os.OpenRoot(spec.Path)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck

	owned, err := sameInode(root, filepath.Join(cloudInitOwners, spec.Name), spec.Name)
	if err != nil {
		return err
	}

	if !owned {
		return errors.New("cloud-init ISO ownership changed")
	}

	file, err := root.Open(spec.Name)
	if err != nil {
		return err
	}

	digest, size, hashErr := hashISO(file)
	if err := errors.Join(hashErr, file.Close()); err != nil {
		return err
	}

	if digest != spec.Digest || size != spec.SizeBytes {
		return errors.New("cloud-init ISO bytes changed")
	}

	return nil
}

func removeRetiredISO(spec *hypervisor.CloudInitStatusSpec) error {
	root, err := os.OpenRoot(spec.Path)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck

	witness := filepath.Join(cloudInitOwners, spec.Name)

	owned, err := sameInode(root, witness, spec.Name)
	if err != nil {
		return err
	}

	if err := removeRetiredSeedName(root, spec.Name, owned); err != nil {
		return err
	}

	if err := root.Remove(witness); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if err := syncDirectory(root, cloudInitOwners); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return nil
}

// removeRetiredSeedName refuses foreign files and syncs removal of an owned publication before witness cleanup.
func removeRetiredSeedName(root *os.Root, name string, owned bool) error {
	if !owned {
		if _, err := root.Lstat(name); err == nil {
			return fmt.Errorf("refusing to delete foreign ISO %q", name)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	} else {
		if err := root.Remove(name); err != nil {
			return err
		}
	}

	// A previous attempt may have unlinked the publication but failed its sync.
	// Re-establish that barrier before discarding the durable ownership witness.
	return syncDirectory(root, ".")
}

func sameInode(root *os.Root, witness, name string) (bool, error) {
	proof, err := root.Lstat(witness)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	asset, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return proof.Mode().IsRegular() && asset.Mode().IsRegular() && os.SameFile(proof, asset), nil
}

func hashISO(file *os.File) (string, uint64, error) {
	hash := sha256.New()
	size, err := io.Copy(hash, file)

	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), uint64(size), err
}

func syncDirectory(root *os.Root, name string) error {
	dir, err := root.Open(name)
	if err != nil {
		return err
	}

	return errors.Join(dir.Sync(), dir.Close())
}
