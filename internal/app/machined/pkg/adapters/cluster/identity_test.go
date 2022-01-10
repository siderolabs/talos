// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clusteradapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/cluster"
	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
)

func TestIdentityGenerate(t *testing.T) {
	var spec1, spec2 cluster.IdentitySpec

	require.NoError(t, clusteradapter.IdentitySpec(&spec1).Generate())
	require.NoError(t, clusteradapter.IdentitySpec(&spec2).Generate())

	assert.NotEqual(t, spec1, spec2)

	length := len(spec1.NodeID)

	assert.GreaterOrEqual(t, length, 43)
	assert.LessOrEqual(t, length, 44)
}

func TestIdentityConvertMachineID(t *testing.T) {
	spec := cluster.IdentitySpec{
		NodeID: "sou7yy34ykX3n373Zw1DXKb8zD7UnyKT6HT3QDsGH6L",
	}

	machineID, err := clusteradapter.IdentitySpec(&spec).ConvertMachineID()
	require.NoError(t, err)

	assert.Equal(t, "be871ac0d0dd31fa4caca753b0f3f1b2", string(machineID))
}
