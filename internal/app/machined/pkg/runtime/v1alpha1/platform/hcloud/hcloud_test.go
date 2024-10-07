// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hcloud_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/hcloud"
)

//go:embed testdata/metadata.yaml
var rawMetadata []byte

//go:embed testdata/expected.yaml
var expectedNetworkConfig string

func TestParseMetadata(t *testing.T) {
	h := &hcloud.Hcloud{}

	metadata := &hcloud.MetadataConfig{
		Hostname:         "talos.fqdn",
		PublicIPv4:       "1.2.3.4",
		InstanceID:       "0",
		Region:           "hel1",
		AvailabilityZone: "hel1-dc2",
	}

	var m hcloud.NetworkConfig

	require.NoError(t, yaml.Unmarshal(rawMetadata, &m))

	networkConfig, err := h.ParseMetadata(&m, metadata)
	require.NoError(t, err)

	marshaled, err := yaml.Marshal(networkConfig)
	require.NoError(t, err)

	assert.Equal(t, expectedNetworkConfig, string(marshaled))
}

//go:embed testdata/userdata-plain.yaml
var userdataPlain []byte

//go:embed testdata/userdata-base64.txt
var userdataBase64 []byte

func TestParseUserdata(t *testing.T) {
	decodedUserdataPlain := hcloud.MaybeBase64Decode(userdataPlain)
	decodedUserdataBase64 := hcloud.MaybeBase64Decode(userdataBase64)

	assert.Equal(t, decodedUserdataPlain, decodedUserdataBase64)
	assert.Equal(t, userdataPlain, decodedUserdataBase64)
}
