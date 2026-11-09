// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

func (suite *StoragePoolVolumeSuite) TestPrivateCleanupWaitsForAuthoritativeConfiguration() {
	key := fmt.Sprintf("%x", sha256.Sum256([]byte("retired-copy")))
	marker, binding := suite.privateCopyMetadata(key)
	suite.startPrivateCleanupSentinel()
	suite.Require().FileExists(marker)
	suite.Require().FileExists(binding)

	ctr, err := container.New()
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(ctr))
	suite.assertPrivateMetadataRemoved(marker, binding)
}

func (suite *StoragePoolVolumeSuite) TestPrivateCleanupProtectsCanonicalIntentBeforeRuntimeReconstruction() {
	key := fmt.Sprintf("%x", sha256.Sum256([]byte("guest\x00disk\x00"+volumePool+"\x00raw\x00missing-source\x00missing.img\x00")))
	paths := suite.privateImmutableMetadata(key)
	vm := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	vm.MetaName = "guest"
	vm.DisksConfig = hypervisorcfg.VirtualMachineDiskList{
		{
			DiskName: "disk",
			DiskPool: volumePool,
			DiskType: hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
					ImageLibrary: "missing-source",
					ImageFile:    "missing.img",
				},
			},
		},
	}
	ctr, err := container.New(vm)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)
	suite.startPrivateCleanupSentinel()

	for _, path := range paths {
		suite.Require().FileExists(path)
	}

	empty, err := container.New()
	suite.Require().NoError(err)

	updated := config.NewMachineConfig(empty)
	updated.Metadata().SetVersion(cfg.Metadata().Version())
	suite.Update(updated)
	suite.assertPrivateMetadataRemoved(paths...)
}

func (suite *StoragePoolVolumeSuite) TestPrivateCleanupWaitsForConsumerDrain() {
	key := fmt.Sprintf("%x", sha256.Sum256([]byte("retired-consumer-copy")))
	paths := suite.privateImmutableMetadata(key)
	ctr, err := container.New()
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	status := storageres.NewStoragePoolVolumeStatus(storageres.NamespaceName, storageres.StoragePoolVolumeID(volumePool, "retired.img"))
	status.TypedSpec().Pool = volumePool
	status.TypedSpec().Name = "retired.img"
	status.TypedSpec().Phase = storageres.StoragePoolVolumePhaseReady
	suite.Create(status, state.WithCreateOwner(volumeFinalizer))
	suite.AddFinalizer(status.Metadata(), "test.GuestConsumer")
	suite.startPrivateCleanupSentinel()
	ctest.AssertResource(suite, status.Metadata().ID(), func(current *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, current.Metadata().Phase())
	})

	for _, path := range paths {
		suite.Require().FileExists(path)
	}

	suite.RemoveFinalizer(status.Metadata(), "test.GuestConsumer")
	ctest.AssertNoResource[*storageres.StoragePoolVolumeStatus](suite, status.Metadata().ID())
	// A canonical revision makes retirement immediately eligible; the independent
	// deadline regression covers periodic collection without a configuration event.
	updated := config.NewMachineConfig(ctr)
	updated.Metadata().SetVersion(cfg.Metadata().Version())
	suite.Update(updated)
	suite.assertPrivateMetadataRemoved(paths...)
}

func (suite *StoragePoolVolumeSuite) privateCopyMetadata(key string) (string, string) {
	target := suite.T().TempDir()
	private := filepath.Join(target, ".copy-on-use")
	suite.Require().NoError(os.Mkdir(private, 0o700))

	name := "retired.img"
	marker := filepath.Join(private, fmt.Sprintf("image-%x", sha256.Sum256([]byte(name))))
	binding := filepath.Join(private, "binding-"+key)

	suite.Require().NoError(os.WriteFile(marker, []byte("image-owner:raw:"+uuid.NewString()+"\n"), 0o600))
	suite.Require().NoError(os.WriteFile(binding, []byte(name), 0o600))
	pool := suite.pool(true)
	ctest.UpdateWithConflicts(suite, pool, func(current *storageres.StoragePoolStatus) error {
		current.TypedSpec().TargetPath = target

		return nil
	}, state.WithUpdateOwner("storage.StoragePoolController"))

	return marker, binding
}

func (suite *StoragePoolVolumeSuite) privateImmutableMetadata(key string) []string {
	marker, binding := suite.privateCopyMetadata(key)
	suite.Require().NoError(os.Remove(marker))
	private := filepath.Dir(marker)
	name := "retired.img"
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
	witness := filepath.Join(private, digest)
	owner := filepath.Join(private, "owner-"+digest)

	suite.Require().NoError(os.WriteFile(witness, []byte("immutable recovery payload"), 0o600))
	info, err := os.Stat(witness)
	suite.Require().NoError(err)

	identity := info.Sys().(*syscall.Stat_t)
	contents := fmt.Sprintf("%s\n%s\n%d\n", key, name, identity.Ino)
	suite.Require().NoError(os.WriteFile(owner, []byte(contents), 0o600))

	return []string{witness, owner, binding}
}

func (suite *StoragePoolVolumeSuite) startPrivateCleanupSentinel() {
	spec := suite.spec(volumePool, "sentinel.raw", "raw", 8)
	suite.start()
	ctest.AssertResource(suite, spec.Metadata().ID(), func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseReady, status.TypedSpec().Phase)
	})

	// Ready is published before private maintenance. A later request cannot be
	// reconciled until that first maintenance pass has finished.
	next := suite.spec(volumePool, "after-maintenance.raw", "raw", 8)
	ctest.AssertResource(suite, next.Metadata().ID(), func(status *storageres.StoragePoolVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.StoragePoolVolumePhaseReady, status.TypedSpec().Phase)
	})
}

func (suite *StoragePoolVolumeSuite) assertPrivateMetadataRemoved(paths ...string) {
	suite.Require().Eventually(func() bool {
		for _, path := range paths {
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				return false
			}
		}

		return true
	}, 5*time.Second, 10*time.Millisecond, "retired private ownership metadata was not collected")
}
