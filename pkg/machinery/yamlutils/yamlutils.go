// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package yamlutils provides utility types to work with YAML marshaling and unmarshaling.
package yamlutils

import "bytes"

// StringBytes is a type that represents a byte slice as a string when marshaled to YAML.
type StringBytes []byte

// MarshalYAML implements yaml.Marshaller interface for StringBytes.
func (s StringBytes) MarshalYAML() (any, error) {
	if bytes.Equal(bytes.ToValidUTF8(s, nil), s) {
		// If the byte slice is valid UTF-8, return it as a string.
		return string(s), nil
	}

	return s.Bytes(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface for StringBytes.
func (s *StringBytes) UnmarshalYAML(unmarshal func(any) error) error {
	var str string

	if err := unmarshal(&str); err == nil {
		*s = []byte(str)

		return nil
	}

	var data []byte

	if err := unmarshal(&data); err != nil {
		return err
	}

	*s = data

	return nil
}

// Bytes returns the byte slice representation of StringBytes.
func (s StringBytes) Bytes() []byte {
	return []byte(s)
}
