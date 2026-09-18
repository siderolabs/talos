// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	storagecfg "github.com/siderolabs/talos/pkg/machinery/config/types/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

type StoragePoolSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *StoragePoolSpecSuite) TestStoragePoolSpec() {
	images := storagecfg.NewStoragePoolV1Alpha1()
	images.MetaName = "images"
	images.VolumeConfig.VolumeName = "data"
	archives := storagecfg.NewStoragePoolV1Alpha1()
	archives.MetaName = "archives"
	archives.VolumeConfig.VolumeName = "external"
	user := blockcfg.NewUserVolumeConfigV1Alpha1()
	user.MetaName = "data"
	external := blockcfg.NewExternalVolumeConfigV1Alpha1()
	external.MetaName = "external"
	cfg := applyMachineConfigDocs(&suite.DefaultSuite, user, external, images, archives)

	ctest.AssertResources(
		suite,
		[]string{"images", "archives"},
		func(spec *storageres.StoragePoolSpec, asrt *assert.Assertions) {
			asrt.Equal(map[string]string{"images": "u-data", "archives": "x-external"}[spec.Metadata().ID()], spec.TypedSpec().VolumeID)
		},
	)

	archives = archives.DeepCopy()
	archives.VolumeConfig.VolumeName = "data"
	existing := blockcfg.NewExistingVolumeConfigV1Alpha1()
	existing.MetaName = "data"
	ctr, err := container.New(existing, archives)
	suite.Require().NoError(err)

	updated := config.NewMachineConfig(ctr)
	updated.Metadata().SetVersion(cfg.Metadata().Version())
	suite.Update(updated)

	ctest.AssertNoResource[*storageres.StoragePoolSpec](suite, "images")
	ctest.AssertResource(suite, "archives", func(spec *storageres.StoragePoolSpec, asrt *assert.Assertions) {
		asrt.Equal("e-data", spec.TypedSpec().VolumeID)
	})

	suite.Destroy(updated)

	ctest.AssertNoResource[*storageres.StoragePoolSpec](suite, "archives")
}

func (suite *StoragePoolSpecSuite) TestLiteralNameAndInvalidReferences() {
	user := blockcfg.NewUserVolumeConfigV1Alpha1()
	user.MetaName = "u-images"
	pool := storagecfg.NewStoragePoolV1Alpha1()
	pool.MetaName = "images"
	pool.VolumeConfig.VolumeName = "u-images"
	cfg := applyMachineConfigDocs(&suite.DefaultSuite, user, pool)

	ctest.AssertResource(suite, "images", func(spec *storageres.StoragePoolSpec, asrt *assert.Assertions) {
		asrt.Equal("u-u-images", spec.TypedSpec().VolumeID)
	})

	// The runtime ID is not an alias for the document name.
	pool = pool.DeepCopy()
	pool.VolumeConfig.VolumeName = "u-u-images"
	ctr, err := container.New(user, pool)
	suite.Require().NoError(err)

	updated := config.NewMachineConfig(ctr)
	updated.Metadata().SetVersion(cfg.Metadata().Version())
	suite.Update(updated)
	ctest.AssertNoResource[*storageres.StoragePoolSpec](suite, "images")
}

func (suite *StoragePoolSpecSuite) TestUnusableBackingVolumes() {
	names := []string{"raw", "swap", "readonly-existing", "readonly-external", "unknown"}
	initial := make([]configcfg.Document, 0, 2*len(names))
	updatedDocs := make([]configcfg.Document, 0, 2*len(names))

	for _, name := range names {
		volume := blockcfg.NewUserVolumeConfigV1Alpha1()
		volume.MetaName = name
		pool := storagecfg.NewStoragePoolV1Alpha1()
		pool.MetaName = name
		pool.VolumeConfig.VolumeName = name
		initial = append(initial, volume, pool)
		updatedDocs = append(updatedDocs, pool)
	}

	cfg := applyMachineConfigDocs(&suite.DefaultSuite, initial...)
	ctest.AssertResources(suite, names, func(spec *storageres.StoragePoolSpec, asrt *assert.Assertions) {
		asrt.Equal("u-"+spec.Metadata().ID(), spec.TypedSpec().VolumeID)
	})

	raw := blockcfg.NewRawVolumeConfigV1Alpha1()
	raw.MetaName = "raw"
	swap := blockcfg.NewSwapVolumeConfigV1Alpha1()
	swap.MetaName = "swap"
	existing := blockcfg.NewExistingVolumeConfigV1Alpha1()
	existing.MetaName = "readonly-existing"
	existing.MountSpec.MountReadOnly = new(true)
	external := blockcfg.NewExternalVolumeConfigV1Alpha1()
	external.MetaName = "readonly-external"
	external.MountSpec.MountReadOnly = new(true)
	updatedDocs = append(updatedDocs, raw, swap, existing, external)
	ctr, err := container.New(updatedDocs...)
	suite.Require().NoError(err)

	updated := config.NewMachineConfig(ctr)
	updated.Metadata().SetVersion(cfg.Metadata().Version())
	suite.Update(updated)

	for _, name := range names {
		ctest.AssertNoResource[*storageres.StoragePoolSpec](suite, name)
	}
}

func TestStoragePoolSpecSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &StoragePoolSpecSuite{
		Timeout: 5 * time.Second,
		AfterSetup: func(s *ctest.DefaultSuite) {
			s.Require().NoError(s.Runtime().RegisterController(&storagectrl.StoragePoolSpecController{}))
		},
	})
}
