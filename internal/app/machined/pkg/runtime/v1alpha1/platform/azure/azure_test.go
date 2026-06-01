// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package azure_test

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/azure"
)

//go:embed testdata/interfaces.json
var rawInterfaces []byte

//go:embed testdata/compute.json
var rawCompute []byte

//go:embed testdata/loadbalancer.json
var rawLoadBalancerMetadata []byte

//go:embed testdata/expected.yaml
var expectedNetworkConfig string

func TestParseMetadata(t *testing.T) {
	a := &azure.Azure{}

	var interfacesMetadata []azure.NetworkConfig

	require.NoError(t, json.Unmarshal(rawInterfaces, &interfacesMetadata))

	var computeMetadata azure.ComputeMetadata

	require.NoError(t, json.Unmarshal(rawCompute, &computeMetadata))

	networkConfig, err := a.ParseMetadata(&computeMetadata, interfacesMetadata, []byte("some.fqdn"))
	require.NoError(t, err)

	var lb azure.LoadBalancerMetadata

	require.NoError(t, json.Unmarshal(rawLoadBalancerMetadata, &lb))

	networkConfig.ExternalIPs, err = a.ParseLoadBalancerIP(lb, networkConfig.ExternalIPs)
	require.NoError(t, err)

	marshaled, err := yaml.Marshal(networkConfig)
	require.NoError(t, err)

	assert.Equal(t, expectedNetworkConfig, string(marshaled))
}
