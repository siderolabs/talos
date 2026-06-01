// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	storagecfg "github.com/siderolabs/talos/pkg/machinery/config/types/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// createDisk inserts a block.Disk plus a matching whole-disk
// block.DiscoveredVolume so the selector controller can evaluate disk-level CEL
// expressions against it.
func createDisk(suite *ctest.DefaultSuite, id, devPath, transport string) {
	d := block.NewDisk(block.NamespaceName, id)
	d.TypedSpec().DevPath = devPath
	d.TypedSpec().Transport = transport

	suite.Create(d)

	dv := block.NewDiscoveredVolume(block.NamespaceName, id)
	dv.TypedSpec().DevPath = devPath
	dv.TypedSpec().Type = "disk"

	suite.Create(dv)
}

// createPartition inserts a partition block.DiscoveredVolume with the given
// parent device path and partition label.
//
//nolint:unparam
func createPartition(suite *ctest.DefaultSuite, id, devPath, parentDevPath, partitionLabel string) {
	dv := block.NewDiscoveredVolume(block.NamespaceName, id)
	dv.TypedSpec().DevPath = devPath
	dv.TypedSpec().ParentDevPath = parentDevPath
	dv.TypedSpec().PartitionLabel = partitionLabel
	dv.TypedSpec().Type = "partition"

	suite.Create(dv)
}

// newVGDoc builds a minimal v1alpha1 LVMVolumeGroupConfig doc with the given
// name and CEL match expression for the physical-volume selector.
func newVGDoc(name, match string) *storagecfg.LVMVolumeGroupConfigV1Alpha1 {
	doc := storagecfg.NewLVMVolumeGroupConfigV1Alpha1()
	doc.MetaName = name

	if err := doc.PhysicalVolumes.VolumeSelector.Match.UnmarshalText([]byte(match)); err != nil {
		panic(err)
	}

	return doc
}

// applyMachineConfig creates a MachineConfig resource carrying the given
// v1alpha1 LVMVolumeGroupConfig docs and returns it so tests can later
// destroy it.
func applyMachineConfig(suite *ctest.DefaultSuite, docs ...*storagecfg.LVMVolumeGroupConfigV1Alpha1) *config.MachineConfig {
	cfgDocs := make([]configconfig.Document, 0, len(docs))
	for _, d := range docs {
		cfgDocs = append(cfgDocs, d)
	}

	ctr, err := container.New(cfgDocs...)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	return cfg
}
