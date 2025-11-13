// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package makers_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker/internal/makers"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/flags"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
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

func TestQemuMaker_ValidateQEMUConfig(t *testing.T) {
	tests := []struct {
		name        string
		disks       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "virtio-scsi-single rejected",
			disks:       "virtio-scsi-single:10GiB",
			expectError: true,
			errorMsg:    "virtio-scsi-single disk controller detected",
		},
		{
			name:        "regular virtio-scsi allowed",
			disks:       "virtio:10GiB",
			expectError: false,
		},
		{
			name:        "multiple disks with virtio-scsi-single",
			disks:       "virtio:10GiB,virtio-scsi-single:20GiB",
			expectError: true,
			errorMsg:    "virtio-scsi-single disk controller detected",
		},
		{
			name:        "virtio-scsi-single with trailing whitespace",
			disks:       "virtio-scsi-single :10GiB",
			expectError: true,
			errorMsg:    "virtio-scsi-single disk controller detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cOps := clusterops.GetCommon()
			qOps := clusterops.GetQemu()
			qOps.Disks = flags.Disks{}
			cli.Should(qOps.Disks.Set(tt.disks))

			_, err := makers.NewQemu(makers.MakerOptions[clusterops.Qemu]{
				ExtraOps:    qOps,
				CommonOps:   cOps,
				Provisioner: testProvisioner{},
			})

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
