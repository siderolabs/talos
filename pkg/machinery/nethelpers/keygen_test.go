// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers_test

import (
	"encoding/base64"
	"testing"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
)

func TestWireguardKeyGeneration(t *testing.T) {
	privateKey, err := nethelpers.GenerateWireguardPrivateKey()
	if err != nil {
		t.Errorf("failed to generate wireguard key: %w", err)

		return
	}

	keyString := base64.StdEncoding.EncodeToString(privateKey)

	if err = nethelpers.CheckWireguardKey(keyString); err != nil {
		t.Errorf("wireguard key (%s) validation failed: %w", keyString, err)

		return
	}
}
