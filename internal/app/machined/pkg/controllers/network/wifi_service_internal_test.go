// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestRenderWpaSupplicantConfig(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		`ctrl_interface=/run/wpa_supplicant
country=NL

network={
	ssid=486f6d654e6574776f726b
	priority=2
	key_mgmt=WPA-PSK SAE
	ieee80211w=1
	psk="pass\"phrase\\"
	sae_password="pass\"phrase\\"
}

network={
	ssid=4f70656e4e6574776f726b
	scan_ssid=1
	priority=1
	key_mgmt=NONE
}
`,
		renderWpaSupplicantConfig(&network.WifiSpecSpec{
			CountryCode: "NL",
			Networks: []network.WifiNetwork{
				{
					SSID: "HomeNetwork",
					PSK:  `pass"phrase\`,
				},
				{
					SSID:   "OpenNetwork",
					Hidden: true,
				},
			},
		}),
	)
}
