// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	kubespanadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/fipsmode"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
)

func TestIdentityGenerateKey(t *testing.T) {
	if fipsmode.Strict() {
		t.Skip("skipping test in strict FIPS mode")
	}

	var spec kubespan.IdentitySpec

	assert.NoError(t, kubespanadapter.IdentitySpec(&spec).GenerateKey())
}

func TestIdentityUpdateAddress(t *testing.T) {
	var spec kubespan.IdentitySpec

	mac, err := net.ParseMAC("2e:1a:b6:53:81:69")
	require.NoError(t, err)

	assert.NoError(t, kubespanadapter.IdentitySpec(&spec).UpdateAddress("8XuV9TZHW08DOk3bVxQjH9ih_TBKjnh-j44tsCLSBzo=", mac))

	assert.Equal(t, "fd7f:175a:b97c:5602:2c1a:b6ff:fe53:8169/128", spec.Address.String())
	assert.Equal(t, "fd7f:175a:b97c:5602::/64", spec.Subnet.String())
}
