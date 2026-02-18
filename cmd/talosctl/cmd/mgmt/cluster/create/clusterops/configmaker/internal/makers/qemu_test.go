// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker/internal/makers"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/flags"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/provision"
)

func TestQemuMaker_MachineConfig(t *testing.T) {
	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()

	m, err := makers.NewQemu(makers.MakerOptions[clusterops.Qemu]{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: testProvisioner{}, // use test provisioner to simplify the test case.
	})
	require.NoError(t, err)

	desiredExtraGenOps := []generate.Option{}

	assertConfigDefaultness(t, cOps, *m.Maker, desiredExtraGenOps...)
}

func TestQemuMaker_Disks(t *testing.T) {
	cOps := clusterops.GetCommon()
	qOps := clusterops.GetQemu()

	disks := flags.Disks{}
	err := disks.Set("virtio:10GiB,nvme:20GiB,virtio:30GiB")
	require.NoError(t, err)

	qOps.Disks = disks
	cOps.Controlplanes = 1
	cOps.Workers = 1

	m, err := makers.NewQemu(makers.MakerOptions[clusterops.Qemu]{
		ExtraOps:    qOps,
		CommonOps:   cOps,
		Provisioner: testProvisioner{}, // use test provisioner to simplify the test case.
	})
	require.NoError(t, err)

	req, err := m.GetClusterConfigs()
	require.NoError(t, err)

	controlplaneDisks := req.ClusterRequest.Nodes[0].Disks
	workerDisks := req.ClusterRequest.Nodes[1].Disks

	assert.Equal(t, 1, len(controlplaneDisks))
	assert.Equal(t, 3, len(workerDisks))

	assert.Equal(t, []*provision.Disk{
		{
			Size:            disks.Requests()[0].Size.Bytes(),
			SkipPreallocate: !qOps.PreallocateDisks,
			Driver:          "virtio",
			BlockSize:       qOps.DiskBlockSize,
			Serial:          "",
		},
	}, controlplaneDisks)

	assert.Equal(t, []*provision.Disk{
		{
			Size:            disks.Requests()[0].Size.Bytes(),
			SkipPreallocate: !qOps.PreallocateDisks,
			Driver:          "virtio",
			BlockSize:       qOps.DiskBlockSize,
			Serial:          "",
		},
		{
			Size:            disks.Requests()[1].Size.Bytes(),
			SkipPreallocate: !qOps.PreallocateDisks,
			Driver:          "nvme",
			BlockSize:       qOps.DiskBlockSize,
			Serial:          "",
		},
		{
			Size:            disks.Requests()[2].Size.Bytes(),
			SkipPreallocate: !qOps.PreallocateDisks,
			Driver:          "virtio",
			BlockSize:       qOps.DiskBlockSize,
			Serial:          "",
		},
	}, workerDisks)
}
