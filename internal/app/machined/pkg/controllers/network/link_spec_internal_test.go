// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"testing"

	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	networkres "github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// TestRawLinkData verifies that link-specific attributes (IFLA_INFO_DATA) come back as raw bytes.
//
// rtnetlink decodes IFLA_INFO_DATA into a typed driver struct (e.g. *driver.Bond) instead of
// *rtnetlink.LinkData for every kind registered in its global driver registry, and that registry is
// populated by the init() of github.com/jsimonetti/rtnetlink/v2/driver. Talos encodes and decodes
// these attributes itself (see the adapters in pkg/adapters/network), so importing that package
// anywhere in the machined dependency graph would make rawLinkData return nil for bonds, bridges,
// VLANs, macvlans, veths and VXLANs, breaking every link kind that carries settings.
func TestRawLinkData(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{
		networkres.LinkKindBond,
		networkres.LinkKindBridge,
		networkres.LinkKindVLAN,
		networkres.LinkKindMacVLAN,
		networkres.LinkKindVeth,
		networkres.LinkKindVXLAN,
	} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			data := []byte{0x08, 0x00, 0x01, 0x00, 0x04, 0x00, 0x00, 0x00}

			encoded, err := (&rtnetlink.LinkMessage{
				Attributes: &rtnetlink.LinkAttributes{
					Name: "link0",
					Info: &rtnetlink.LinkInfo{
						Kind: kind,
						Data: &rtnetlink.LinkData{Name: kind, Data: data},
					},
				},
			}).MarshalBinary()
			require.NoError(t, err)

			var decoded rtnetlink.LinkMessage

			require.NoError(t, decoded.UnmarshalBinary(encoded))

			assert.Equal(t, data, rawLinkData(&decoded))
		})
	}
}
