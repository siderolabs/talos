// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import "github.com/siderolabs/talos/pkg/machinery/config/encoder"

// Encoder provides the interface to encode configuration documents.
type Encoder interface {
	// Bytes returns source YAML representation (if available) or does default encoding.
	Bytes() ([]byte, error)

	// Encode configuration to YAML using the provided options.
	EncodeString(encoderOptions ...encoder.Option) (string, error)
	EncodeBytes(encoderOptions ...encoder.Option) ([]byte, error)
}
