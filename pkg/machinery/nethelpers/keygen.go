// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateWireguardPrivateKey generates a random Wireguard key suitable as a private key.
func GenerateWireguardPrivateKey() ([]byte, error) {
	privateKey, err := GenerateWireguardKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}

	// Modify random bytes using algorithm described at:
	// https://cr.yp.to/ecdh.html.
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	return privateKey, nil
}

// GenerateWireguardKey generates a random Wireguard key.
func GenerateWireguardKey() ([]byte, error) {
	const WireguardKeyLen = 32

	// NB:  procedure stolen from wgctrl-go to avoid importing entire package:
	// https://github.com/WireGuard/wgctrl-go/blob/92e472f520a5/wgtypes/types.go#L89.
	k := make([]byte, WireguardKeyLen)
	if _, err := rand.Read(k); err != nil {
		return nil, fmt.Errorf("failed to read random bytes to generate wireguard key: %v", err)
	}

	return k, nil
}

// CheckWireguardKey is implemented to avoid pulling in wgctrl code to keep machinery dependencies slim.
func CheckWireguardKey(key string) error {
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return err
	}

	if len(raw) != 32 {
		return fmt.Errorf("wrong key %q length: %d", key, len(raw))
	}

	return nil
}
