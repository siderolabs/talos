// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	"github.com/siderolabs/talos/internal/pkg/cloudinit"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

type CloudInitISOSuite struct{ ctest.DefaultSuite }

func TestCloudInitISOSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, &CloudInitISOSuite{ctest.DefaultSuite{
		Timeout: 30 * time.Second,
		AfterSetup: func(s *ctest.DefaultSuite) {
			s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.CloudInitISOController{}))
		},
	}})
}

func (s *CloudInitISOSuite) TestBlockedBackingDoesNotTakePoolHold() {
	for _, tc := range []struct {
		name      string
		mutating  bool
		wantError string
	}{
		{
			name:      "mutation-in-progress",
			mutating:  true,
			wantError: "backing is changing",
		},
		{
			name:      "missing-mount",
			wantError: "backing mount",
		},
	} {
		s.Run(tc.name, func() {
			dir := s.T().TempDir()
			pool := storage.NewStoragePoolStatus(storage.NamespaceName, tc.name)
			pool.TypedSpec().VolumeID = "backing-" + tc.name
			pool.TypedSpec().TargetPath = filepath.Join(dir, tc.name)
			pool.TypedSpec().Phase = storage.StoragePoolPhaseReady

			if tc.mutating {
				pool.Metadata().Finalizers().Add("storage.StoragePoolController/mutating/backing")
				mount := block.NewVolumeMountStatus(block.NamespaceName, pool.TypedSpec().VolumeID)
				mount.TypedSpec().Target = dir
				s.Create(mount)
			}

			s.Create(pool)
			before, err := safe.StateGet[*storage.StoragePoolStatus](s.Ctx(), s.State(), pool.Metadata())
			s.Require().NoError(err)

			seed := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "guest-"+tc.name)
			*seed.TypedSpec() = hypervisor.CloudInitSpecSpec{
				Pool:     tc.name,
				UserData: "#cloud-config\n",
			}
			s.Create(seed)
			id := hypervisor.CloudInitStatusID(seed.Metadata().ID(), *seed.TypedSpec())
			ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
				a.Equal(hypervisor.CloudInitPhaseNotReady, status.TypedSpec().Phase)
				a.Contains(status.TypedSpec().Error, tc.wantError)
			})
			after, err := safe.StateGet[*storage.StoragePoolStatus](s.Ctx(), s.State(), pool.Metadata())
			s.Require().NoError(err)
			s.Equal(before.Metadata().Version(), after.Metadata().Version(), "a known unusable backing must not acquire and release a pool hold")
			s.False(after.Metadata().Finalizers().Has("hypervisor.CloudInitISOController"))
		})
	}
}

func (s *CloudInitISOSuite) TestCreatesSeedDirectlyInPoolWithoutLibrary() {
	dir := s.T().TempDir()
	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "pool-a")
	pool.TypedSpec().VolumeID = "pool-backing"
	pool.TypedSpec().TargetPath = filepath.Join(dir, "pool-a")
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	s.Require().NoError(os.Mkdir(pool.TypedSpec().TargetPath, 0o700))

	mount := block.NewVolumeMountStatus(block.NamespaceName, "pool-backing")
	mount.TypedSpec().Target = dir
	s.Create(mount)
	s.Create(pool)

	seed := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "guest")
	*seed.TypedSpec() = hypervisor.CloudInitSpecSpec{Pool: "pool-a", UserData: "#cloud-config\n"}
	s.Create(seed)
	id := hypervisor.CloudInitStatusID("guest", *seed.TypedSpec())
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(hypervisor.CloudInitPhaseReady, status.TypedSpec().Phase)
		a.Equal(pool.TypedSpec().TargetPath, status.TypedSpec().Path)

		info, err := os.Lstat(filepath.Join(status.TypedSpec().Path, status.TypedSpec().Name))
		if a.NoError(err) {
			a.True(info.Mode().IsRegular())
		}
	})
	requests, err := safe.StateListAll[*storage.StoragePoolVolumeSpec](s.Ctx(), s.State())
	s.Require().NoError(err)
	s.Zero(requests.Len())
}

func (s *CloudInitISOSuite) TestDoesNotPublishIntoTearingDownMount() {
	dir := s.T().TempDir()
	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "pool-a")
	pool.TypedSpec().VolumeID = "pool-backing"
	pool.TypedSpec().TargetPath = filepath.Join(dir, "pool-a")
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	s.Require().NoError(os.Mkdir(pool.TypedSpec().TargetPath, 0o700))

	mount := block.NewVolumeMountStatus(block.NamespaceName, "pool-backing")
	mount.TypedSpec().Target = dir
	s.Create(mount)
	s.AddFinalizer(mount.Metadata(), "mounted")
	_, err := s.State().Teardown(s.Ctx(), mount.Metadata())
	s.Require().NoError(err)
	s.Create(pool)

	seed := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "guest")
	*seed.TypedSpec() = hypervisor.CloudInitSpecSpec{Pool: "pool-a", UserData: "#cloud-config\n"}
	s.Create(seed)
	ctest.AssertResource(s, hypervisor.CloudInitStatusID("guest", *seed.TypedSpec()), func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(hypervisor.CloudInitPhaseNotReady, status.TypedSpec().Phase)
		a.Empty(status.TypedSpec().Path)
	})
}

func (s *CloudInitISOSuite) pool() (*storage.StoragePoolStatus, *block.VolumeMountStatus) {
	s.T().Helper()
	dir := s.T().TempDir()
	pool := storage.NewStoragePoolStatus(storage.NamespaceName, "pool-a")
	pool.TypedSpec().VolumeID = "pool-backing"
	pool.TypedSpec().TargetPath = filepath.Join(dir, "pool-a")
	pool.TypedSpec().Phase = storage.StoragePoolPhaseReady
	s.Require().NoError(os.Mkdir(pool.TypedSpec().TargetPath, 0o700))

	mount := block.NewVolumeMountStatus(block.NamespaceName, "pool-backing")
	mount.TypedSpec().Target = dir
	s.Create(mount)
	s.Create(pool)

	return pool, mount
}

func seedName(id string) string { return "cloud-init-" + id[strings.LastIndex(id, "@")+1:] + ".iso" }

func (s *CloudInitISOSuite) TestChangedSeedRetiresOnlyAfterConsumerDetaches() {
	pool, _ := s.pool()
	seed := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "guest")
	*seed.TypedSpec() = hypervisor.CloudInitSpecSpec{Pool: "pool-a", UserData: "first"}
	s.Create(seed)
	firstID := hypervisor.CloudInitStatusID("guest", *seed.TypedSpec())
	ctest.AssertResource(s, firstID, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(hypervisor.CloudInitPhaseReady, status.TypedSpec().Phase)
	})
	first := hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, firstID)
	s.AddFinalizer(first.Metadata(), "consumer")
	ctest.UpdateWithConflicts(s, seed, func(current *hypervisor.CloudInitSpec) error {
		current.TypedSpec().UserData = "second"

		return nil
	})
	changed := *seed.TypedSpec()
	changed.UserData = "second"
	secondID := hypervisor.CloudInitStatusID("guest", changed)
	ctest.AssertResource(s, secondID, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(hypervisor.CloudInitPhaseReady, status.TypedSpec().Phase)
	})
	ctest.AssertResource(s, firstID, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(resource.PhaseTearingDown, status.Metadata().Phase())
	})
	firstPath := filepath.Join(pool.TypedSpec().TargetPath, seedName(firstID))
	s.FileExists(firstPath)
	s.RemoveFinalizer(first.Metadata(), "consumer")
	ctest.AssertNoResource[*hypervisor.CloudInitStatus](s, firstID)
	s.NoFileExists(firstPath)
	s.FileExists(filepath.Join(pool.TypedSpec().TargetPath, seedName(secondID)))
	s.Destroy(seed)
	ctest.AssertNoResource[*hypervisor.CloudInitStatus](s, secondID)
	s.NoFileExists(filepath.Join(pool.TypedSpec().TargetPath, seedName(secondID)))
	ctest.AssertResource(s, "pool-a", func(current *storage.StoragePoolStatus, a *assert.Assertions) {
		a.False(current.Metadata().Finalizers().Has("hypervisor.CloudInitISOController"))
	})
}

func (s *CloudInitISOSuite) TestRefusesUnownedSeedPublicationAndCleanup() {
	pool, _ := s.pool()
	seed := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "guest")
	*seed.TypedSpec() = hypervisor.CloudInitSpecSpec{Pool: "pool-a", UserData: "seed"}
	id := hypervisor.CloudInitStatusID("guest", *seed.TypedSpec())
	path := filepath.Join(pool.TypedSpec().TargetPath, seedName(id))
	// Even identical valid ISO bytes do not prove ownership.
	s.Require().NoError(cloudinit.WriteISO(s.Ctx(), path, cloudinit.Seed{UserData: "seed"}))
	before, err := os.ReadFile(path)
	s.Require().NoError(err)
	s.Create(seed)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(hypervisor.CloudInitPhaseNotReady, status.TypedSpec().Phase)
		a.Contains(status.TypedSpec().Error, "refusing foreign cloud-init ISO collision")
	})
	s.Destroy(seed)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(resource.PhaseTearingDown, status.Metadata().Phase())
	})

	after, err := os.ReadFile(path)
	s.Require().NoError(err)
	s.Equal(before, after)
	s.Require().NoError(os.Remove(path))
	ctest.AssertNoResource[*hypervisor.CloudInitStatus](s, id)
}

func (s *CloudInitISOSuite) TestRetiresSeedBeforePoolRetarget() {
	pool, _ := s.pool()
	seed := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "guest")
	*seed.TypedSpec() = hypervisor.CloudInitSpecSpec{Pool: "pool-a", UserData: "seed"}
	id := hypervisor.CloudInitStatusID("guest", *seed.TypedSpec())
	s.Create(seed)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(hypervisor.CloudInitPhaseReady, status.TypedSpec().Phase)
	})
	status, err := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), id)
	s.Require().NoError(err)
	s.AddFinalizer(status.Metadata(), "test-consumer")

	oldPath := filepath.Join(pool.TypedSpec().TargetPath, seedName(id))
	ctest.UpdateWithConflicts(s, pool, func(current *storage.StoragePoolStatus) error {
		current.TypedSpec().Phase = storage.StoragePoolPhaseNotReady
		current.TypedSpec().Error = "waiting for consumers before replacing backing"

		return nil
	})
	ctest.AssertResource(s, id, func(current *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(resource.PhaseTearingDown, current.Metadata().Phase())
	})
	s.FileExists(oldPath)
	ctest.AssertResource(s, "pool-a", func(current *storage.StoragePoolStatus, a *assert.Assertions) {
		a.True(current.Metadata().Finalizers().Has("hypervisor.CloudInitISOController"))
	})
	s.RemoveFinalizer(status.Metadata(), "test-consumer")
	ctest.AssertResource(s, "pool-a", func(current *storage.StoragePoolStatus, a *assert.Assertions) {
		a.False(current.Metadata().Finalizers().Has("hypervisor.CloudInitISOController"))
	})
	s.NoFileExists(oldPath)

	mount := block.NewVolumeMountStatus(block.NamespaceName, "replacement-backing")
	mount.TypedSpec().Target = s.T().TempDir()
	s.Create(mount)
	path := filepath.Join(mount.TypedSpec().Target, "pool-a")
	s.Require().NoError(os.Mkdir(path, 0o700))
	ctest.UpdateWithConflicts(s, pool, func(current *storage.StoragePoolStatus) error {
		current.TypedSpec().VolumeID = mount.Metadata().ID()
		current.TypedSpec().TargetPath = path
		current.TypedSpec().Phase = storage.StoragePoolPhaseReady
		current.TypedSpec().Error = ""

		return nil
	})
	ctest.AssertResource(s, id, func(current *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(hypervisor.CloudInitPhaseReady, current.TypedSpec().Phase)
		a.Equal(mount.Metadata().ID(), current.TypedSpec().VolumeID)
		a.Equal(path, current.TypedSpec().Path)
	})
	s.FileExists(filepath.Join(path, seedName(id)))
}

func (s *CloudInitISOSuite) TestRetiresOrphanWithoutWritingUnprovenPath() {
	// Finish an initial reconciliation before introducing the orphan, so startup
	// cannot mask a missing status-event subscription.
	s.pool()

	seed := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "live-guest")
	*seed.TypedSpec() = hypervisor.CloudInitSpecSpec{Pool: "pool-a", UserData: "seed"}
	s.Create(seed)
	ctest.AssertResource(s, hypervisor.CloudInitStatusID("live-guest", *seed.TypedSpec()), func(current *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(hypervisor.CloudInitPhaseReady, current.TypedSpec().Phase)
	})

	path := s.T().TempDir()
	file := filepath.Join(path, "seed.iso")
	s.Require().NoError(os.WriteFile(file, []byte("preserve me"), 0o600))

	status := hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, "orphan")
	*status.TypedSpec() = hypervisor.CloudInitStatusSpec{
		VirtualMachine: "guest",
		Pool:           "missing-pool",
		Name:           "seed.iso",
		Path:           path,
		VolumeID:       "old-backing",
		Phase:          hypervisor.CloudInitPhaseReady,
	}
	s.Create(status, state.WithCreateOwner("hypervisor.CloudInitISOController"))
	ctest.AssertNoResource[*hypervisor.CloudInitStatus](s, "orphan")

	data, err := os.ReadFile(file)

	s.Require().NoError(err)
	s.Equal("preserve me", string(data))
}

func (s *CloudInitISOSuite) TestRetirementWaitsForMountButNotStorageDaemon() {
	pool, mount := s.pool()
	seed := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "guest")
	*seed.TypedSpec() = hypervisor.CloudInitSpecSpec{Pool: "pool-a", UserData: "seed"}
	id := hypervisor.CloudInitStatusID("guest", *seed.TypedSpec())
	s.Create(seed)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(hypervisor.CloudInitPhaseReady, status.TypedSpec().Phase)
	})
	s.Destroy(mount)
	s.Destroy(seed)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(resource.PhaseTearingDown, status.Metadata().Phase())
	})
	path := filepath.Join(pool.TypedSpec().TargetPath, seedName(id))
	s.FileExists(path)
	ctest.AssertResource(s, "pool-a", func(current *storage.StoragePoolStatus, a *assert.Assertions) {
		a.True(current.Metadata().Finalizers().Has("hypervisor.CloudInitISOController"))
	})
	ctest.UpdateWithConflicts(s, pool, func(current *storage.StoragePoolStatus) error {
		current.TypedSpec().Phase = storage.StoragePoolPhaseObservationUnavailable

		return nil
	})
	mount.Metadata().SetVersion(resource.VersionUndefined)
	s.Create(mount)
	ctest.AssertNoResource[*hypervisor.CloudInitStatus](s, id)
	s.NoFileExists(path)
}

func (s *CloudInitISOSuite) TestRecoversInterruptedOwnedPublication() {
	pool, _ := s.pool()
	seed := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "guest")
	*seed.TypedSpec() = hypervisor.CloudInitSpecSpec{Pool: "pool-a", UserData: "seed"}
	id := hypervisor.CloudInitStatusID("guest", *seed.TypedSpec())
	name := seedName(id)
	ownerDir := filepath.Join(pool.TypedSpec().TargetPath, ".cloud-init-owners")
	s.Require().NoError(os.Mkdir(ownerDir, 0o700))
	s.Require().NoError(cloudinit.WriteISO(s.Ctx(), filepath.Join(ownerDir, name), cloudinit.Seed{UserData: "seed"}))

	status := hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, id)
	*status.TypedSpec() = hypervisor.CloudInitStatusSpec{
		VirtualMachine: "guest", Pool: "pool-a", Name: name, Path: pool.TypedSpec().TargetPath,
		VolumeID: pool.TypedSpec().VolumeID, InputDigest: seed.TypedSpec().InputDigest(), Phase: hypervisor.CloudInitPhaseNotReady,
	}
	s.Create(status, state.WithCreateOwner("hypervisor.CloudInitISOController"))
	s.Create(seed)
	ctest.AssertResource(s, id, func(current *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(hypervisor.CloudInitPhaseReady, current.TypedSpec().Phase)
		a.NotEmpty(current.TypedSpec().Digest)
	})

	owner, err := os.Stat(filepath.Join(ownerDir, name))
	s.Require().NoError(err)
	published, err := os.Stat(filepath.Join(pool.TypedSpec().TargetPath, name))
	s.Require().NoError(err)
	s.True(os.SameFile(owner, published))
}
