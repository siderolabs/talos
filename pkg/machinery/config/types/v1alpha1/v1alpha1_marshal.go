// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"encoding/base64"
)

// Base64Bytes implements YAML marshaling/unmarshaling via base64 encoding.
type Base64Bytes []byte

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (b *Base64Bytes) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var data string

	if err := unmarshal(&data); err != nil {
		return err
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return err
	}

	*b = decoded

	return nil
}

// MarshalYAML implements the yaml.Marshaler interface.
func (b Base64Bytes) MarshalYAML() (interface{}, error) {
	return base64.StdEncoding.EncodeToString(b), nil
}
