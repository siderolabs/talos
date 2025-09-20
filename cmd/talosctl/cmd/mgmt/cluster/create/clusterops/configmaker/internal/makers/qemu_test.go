// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makers_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker/internal/makers"
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
