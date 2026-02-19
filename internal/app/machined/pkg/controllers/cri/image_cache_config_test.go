// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	crictrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cri"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
	v1alpha1res "github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

func (suite *ImageCacheConfigSuite) TestReconcileNoConfig() {
	ctest.AssertResource(suite, cri.ImageCacheConfigID, func(r *cri.ImageCacheConfig, asrt *assert.Assertions) {
		asrt.Equal(cri.ImageCacheStatusDisabled, r.TypedSpec().Status)
		asrt.Equal(cri.ImageCacheCopyStatusSkipped, r.TypedSpec().CopyStatus)
	})
}

func (suite *ImageCacheConfigSuite) TestReconcileFeatureNotEnabled() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	ctest.AssertResource(suite, cri.ImageCacheConfigID, func(r *cri.ImageCacheConfig, asrt *assert.Assertions) {
		asrt.Equal(cri.ImageCacheStatusDisabled, r.TypedSpec().Status)
		asrt.Equal(cri.ImageCacheCopyStatusSkipped, r.TypedSpec().CopyStatus)
	})
}

func (suite *ImageCacheConfigSuite) TestReconcileFeatureEnabled() {
	ctrlName := (&crictrl.ImageCacheConfigController{}).Name()

	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineFeatures: &v1alpha1.FeaturesConfig{
				ImageCacheSupport: &v1alpha1.ImageCacheConfig{
					CacheLocalEnabled: new(true),
				},
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	ctest.AssertResource(suite, crictrl.VolumeImageCacheISO, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.Equal(`volume.name in ["iso9660", "vfat"] && volume.label.startsWith("TALOS_")`, r.TypedSpec().Locator.Match.String())
	})
	ctest.AssertResource(suite, crictrl.VolumeImageCacheDISK, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.Equal(`volume.partition_label == "IMAGECACHE"`, r.TypedSpec().Locator.Match.String())
	})

	ctest.AssertResource(suite, cri.ImageCacheConfigID, func(r *cri.ImageCacheConfig, asrt *assert.Assertions) {
		asrt.Equal(cri.ImageCacheStatusDisabled, r.TypedSpec().Status)
		asrt.Equal(cri.ImageCacheCopyStatusSkipped, r.TypedSpec().CopyStatus)
	})

	// create volume statuses to simulate the volume being ready
	vs1 := block.NewVolumeStatus(block.NamespaceName, crictrl.VolumeImageCacheISO)
	vs1.TypedSpec().Phase = block.VolumePhaseReady
	suite.Require().NoError(suite.State().Create(suite.Ctx(), vs1))

	vs2 := block.NewVolumeStatus(block.NamespaceName, crictrl.VolumeImageCacheDISK)
	vs2.TypedSpec().Phase = block.VolumePhaseWaiting
	suite.Require().NoError(suite.State().Create(suite.Ctx(), vs2))

	// controller should create mount requests
	ctest.AssertResources(suite,
		[]string{
			ctrlName + "-" + crictrl.VolumeImageCacheISO,
			ctrlName + "-" + crictrl.VolumeImageCacheDISK,
		},
		func(vmr *block.VolumeMountRequest, asrt *assert.Assertions) {
			asrt.Equal(vmr.TypedSpec().VolumeID == crictrl.VolumeImageCacheISO, vmr.TypedSpec().ReadOnly)
		},
	)

	// simulate ISO being mounted
	vms1 := block.NewVolumeMountStatus(block.NamespaceName, ctrlName+"-"+crictrl.VolumeImageCacheISO)
	vms1.TypedSpec().ReadOnly = true
	vms1.TypedSpec().Target = constants.ImageCacheISOMountPoint
	suite.Require().NoError(suite.State().Create(suite.Ctx(), vms1))

	// one volume is ready, but second one is not (yet)
	ctest.AssertResource(suite, cri.ImageCacheConfigID, func(r *cri.ImageCacheConfig, asrt *assert.Assertions) {
		asrt.Equal(cri.ImageCacheStatusPreparing, r.TypedSpec().Status)
		asrt.Equal(cri.ImageCacheCopyStatusPending, r.TypedSpec().CopyStatus)
		asrt.Equal([]string{filepath.Join(constants.ImageCacheISOMountPoint, "imagecache")}, r.TypedSpec().Roots)
	})

	// mark second as ready
	vs2.TypedSpec().Phase = block.VolumePhaseReady
	suite.Require().NoError(suite.State().Update(suite.Ctx(), vs2))

	// simulate disk being mounted
	vms2 := block.NewVolumeMountStatus(block.NamespaceName, ctrlName+"-"+crictrl.VolumeImageCacheDISK)
	vms2.TypedSpec().ReadOnly = false
	vms2.TypedSpec().Target = constants.ImageCacheDiskMountPoint
	suite.Require().NoError(suite.State().Create(suite.Ctx(), vms2))

	// now both volumes are ready, but service hasn't started yet
	ctest.AssertResource(suite, cri.ImageCacheConfigID, func(r *cri.ImageCacheConfig, asrt *assert.Assertions) {
		asrt.Equal(cri.ImageCacheStatusPreparing, r.TypedSpec().Status)
		asrt.Equal([]string{constants.ImageCacheDiskMountPoint, filepath.Join(constants.ImageCacheISOMountPoint, "imagecache")}, r.TypedSpec().Roots)
	})

	// simulate registryd being ready
	service := v1alpha1res.NewService(crictrl.RegistrydServiceID)
	service.TypedSpec().Healthy = true
	service.TypedSpec().Running = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), service))

	// now both volumes are ready, and service is ready, should be ready
	ctest.AssertResource(suite, cri.ImageCacheConfigID, func(r *cri.ImageCacheConfig, asrt *assert.Assertions) {
		asrt.Equal(cri.ImageCacheStatusReady, r.TypedSpec().Status)
		asrt.Equal(cri.ImageCacheCopyStatusReady, r.TypedSpec().CopyStatus)
		asrt.Equal([]string{constants.ImageCacheDiskMountPoint, filepath.Join(constants.ImageCacheISOMountPoint, "imagecache")}, r.TypedSpec().Roots)
	})
}

func (suite *ImageCacheConfigSuite) TestReconcileJustDiskVolume() {
	ctrlName := (&crictrl.ImageCacheConfigController{}).Name()

	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineFeatures: &v1alpha1.FeaturesConfig{
				ImageCacheSupport: &v1alpha1.ImageCacheConfig{
					CacheLocalEnabled: new(true),
				},
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	ctest.AssertResource(suite, cri.ImageCacheConfigID, func(r *cri.ImageCacheConfig, asrt *assert.Assertions) {
		asrt.Equal(cri.ImageCacheStatusDisabled, r.TypedSpec().Status)
		asrt.Equal(cri.ImageCacheCopyStatusSkipped, r.TypedSpec().CopyStatus)
	})

	// create volume statuses to simulate the volume being ready/not
	vs1 := block.NewVolumeStatus(block.NamespaceName, crictrl.VolumeImageCacheISO)
	vs1.TypedSpec().Phase = block.VolumePhaseMissing
	suite.Require().NoError(suite.State().Create(suite.Ctx(), vs1))

	vs2 := block.NewVolumeStatus(block.NamespaceName, crictrl.VolumeImageCacheDISK)
	vs2.TypedSpec().Phase = block.VolumePhaseWaiting
	suite.Require().NoError(suite.State().Create(suite.Ctx(), vs2))

	// ISO is missing, but disk volume is not ready yet
	ctest.AssertResource(suite, cri.ImageCacheConfigID, func(r *cri.ImageCacheConfig, asrt *assert.Assertions) {
		asrt.Equal(cri.ImageCacheStatusDisabled, r.TypedSpec().Status)
		asrt.Equal(cri.ImageCacheCopyStatusSkipped, r.TypedSpec().CopyStatus)
		asrt.Empty(r.TypedSpec().Roots)
	})

	// make disk image cache ready
	vs2.TypedSpec().Phase = block.VolumePhaseReady
	suite.Update(vs2)

	// simulate disk being mounted
	vms2 := block.NewVolumeMountStatus(block.NamespaceName, ctrlName+"-"+crictrl.VolumeImageCacheDISK)
	vms2.TypedSpec().ReadOnly = false
	vms2.TypedSpec().Target = constants.ImageCacheDiskMountPoint
	suite.Require().NoError(suite.State().Create(suite.Ctx(), vms2))

	ctest.AssertResource(suite, cri.ImageCacheConfigID, func(r *cri.ImageCacheConfig, asrt *assert.Assertions) {
		asrt.Equal(cri.ImageCacheStatusPreparing, r.TypedSpec().Status)
		asrt.Equal(cri.ImageCacheCopyStatusSkipped, r.TypedSpec().CopyStatus)
		asrt.Equal([]string{constants.ImageCacheDiskMountPoint}, r.TypedSpec().Roots)
	})

	// simulate registryd being ready
	service := v1alpha1res.NewService(crictrl.RegistrydServiceID)
	service.TypedSpec().Healthy = true
	service.TypedSpec().Running = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), service))

	ctest.AssertResource(suite, cri.ImageCacheConfigID, func(r *cri.ImageCacheConfig, asrt *assert.Assertions) {
		asrt.Equal(cri.ImageCacheStatusReady, r.TypedSpec().Status)
		asrt.Equal(cri.ImageCacheCopyStatusSkipped, r.TypedSpec().CopyStatus)
		asrt.Equal([]string{constants.ImageCacheDiskMountPoint}, r.TypedSpec().Roots)
	})

	// volume mount status should have a finalizer
	ctest.AssertResource(suite,
		ctrlName+"-"+crictrl.VolumeImageCacheDISK,
		func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
			asrt.True(vms.Metadata().Finalizers().Has(ctrlName))
		},
	)

	// now, simulate reboot sequence:
	// * missing ISO volume is destroyed
	// * volume mount status is being torn down
	suite.Destroy(vs1)

	_, err := suite.State().Teardown(suite.Ctx(), block.NewVolumeMountStatus(block.NamespaceName, ctrlName+"-"+crictrl.VolumeImageCacheDISK).Metadata())
	suite.Require().NoError(err)

	// controller should remove its finalizer
	ctest.AssertResource(suite,
		ctrlName+"-"+crictrl.VolumeImageCacheDISK,
		func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
			asrt.True(vms.Metadata().Finalizers().Empty())
		},
	)
}

func (suite *ImageCacheConfigSuite) TestReconcileWithImageCacheVolume() {
	ctrlName := (&crictrl.ImageCacheConfigController{}).Name()

	v1alpha1Cfg := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineFeatures: &v1alpha1.FeaturesConfig{
				ImageCacheSupport: &v1alpha1.ImageCacheConfig{
					CacheLocalEnabled: new(true),
				},
			},
		},
	}

	volumeConfig := blockcfg.NewVolumeConfigV1Alpha1()
	volumeConfig.MetaName = constants.ImageCachePartitionLabel
	volumeConfig.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("10GiB")

	container, err := container.New(v1alpha1Cfg, volumeConfig)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(container)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	ctest.AssertResource(suite, crictrl.VolumeImageCacheDISK, func(r *block.VolumeConfig, asrt *assert.Assertions) {
		asrt.Equal(`volume.partition_label == "IMAGECACHE"`, r.TypedSpec().Locator.Match.String())
		asrt.Equal(`system_disk`, r.TypedSpec().Provisioning.DiskSelector.Match.String())
		asrt.False(r.TypedSpec().Provisioning.PartitionSpec.Grow)
		asrt.EqualValues(crictrl.MinImageCacheSize, r.TypedSpec().Provisioning.PartitionSpec.MinSize)
		asrt.EqualValues(10*1024*1024*1024, r.TypedSpec().Provisioning.PartitionSpec.MaxSize)
	})

	ctest.AssertResource(suite, cri.ImageCacheConfigID, func(r *cri.ImageCacheConfig, asrt *assert.Assertions) {
		asrt.Equal(cri.ImageCacheStatusDisabled, r.TypedSpec().Status)
		asrt.Equal(cri.ImageCacheCopyStatusSkipped, r.TypedSpec().CopyStatus)
	})

	// create volume statuses to simulate the volume being ready & missing
	vs1 := block.NewVolumeStatus(block.NamespaceName, crictrl.VolumeImageCacheISO)
	vs1.TypedSpec().Phase = block.VolumePhaseMissing
	suite.Require().NoError(suite.State().Create(suite.Ctx(), vs1))

	vs2 := block.NewVolumeStatus(block.NamespaceName, crictrl.VolumeImageCacheDISK)
	vs2.TypedSpec().Phase = block.VolumePhaseReady
	suite.Require().NoError(suite.State().Create(suite.Ctx(), vs2))

	// simulate disk being mounted
	vms := block.NewVolumeMountStatus(block.NamespaceName, ctrlName+"-"+crictrl.VolumeImageCacheDISK)
	vms.TypedSpec().ReadOnly = false
	vms.TypedSpec().Target = constants.ImageCacheDiskMountPoint
	suite.Require().NoError(suite.State().Create(suite.Ctx(), vms))

	// simulate registryd being ready
	service := v1alpha1res.NewService(crictrl.RegistrydServiceID)
	service.TypedSpec().Healthy = true
	service.TypedSpec().Running = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), service))

	// now both volumes are ready, and service is ready, should be ready
	ctest.AssertResource(suite, cri.ImageCacheConfigID, func(r *cri.ImageCacheConfig, asrt *assert.Assertions) {
		asrt.Equal(cri.ImageCacheStatusReady, r.TypedSpec().Status)
		asrt.Equal(cri.ImageCacheCopyStatusSkipped, r.TypedSpec().CopyStatus)
		asrt.Equal([]string{constants.ImageCacheDiskMountPoint}, r.TypedSpec().Roots)
	})
}

func TestImageCacheConfigSuite(t *testing.T) {
	s := &ImageCacheConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
		},
	}

	s.AfterSetup = func(suite *ctest.DefaultSuite) {
		suite.Require().NoError(suite.Runtime().RegisterController(&crictrl.ImageCacheConfigController{
			V1Alpha1ServiceManager: &mockServiceRunner{},
			DisableCacheCopy:       true,
		}))
	}

	suite.Run(t, s)
}

type ImageCacheConfigSuite struct {
	ctest.DefaultSuite
}

type mockServiceRunner struct{}

func (mock *mockServiceRunner) IsRunning(id string) (system.Service, bool, error) {
	return nil, true, nil
}

func (mock *mockServiceRunner) Load(services ...system.Service) []string {
	return nil
}

func (mock *mockServiceRunner) Start(serviceIDs ...string) error {
	return nil
}
