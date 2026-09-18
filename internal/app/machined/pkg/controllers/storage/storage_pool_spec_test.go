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
	"github.com/siderolabs/talos/pkg/machinery/config/container"
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
	images.VolumeConfig.VolumeName = "u-data"
	archives := storagecfg.NewStoragePoolV1Alpha1()
	archives.MetaName = "archives"
	archives.VolumeConfig.VolumeName = "x-data"
	cfg := applyMachineConfigDocs(&suite.DefaultSuite, images, archives)

	ctest.AssertResources(
		suite,
		[]string{"images", "archives"},
		func(spec *storageres.StoragePoolSpec, asrt *assert.Assertions) {
			asrt.Equal(map[string]string{"images": "u-data", "archives": "x-data"}[spec.Metadata().ID()], spec.TypedSpec().VolumeID)
		},
	)

	archives = archives.DeepCopy()
	archives.VolumeConfig.VolumeName = "e-data"
	ctr, err := container.New(archives)
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

func TestStoragePoolSpecSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &StoragePoolSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&storagectrl.StoragePoolSpecController{}))
			},
		},
	})
}
