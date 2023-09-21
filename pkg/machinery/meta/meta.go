// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package meta provides interfaces for encoding and decoding META values.
package meta

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/siderolabs/gen/xslices"
)

// Value represents a key/value pair for META.
type Value struct {
	Key   uint8
	Value string
}

func (v Value) String() string {
	return fmt.Sprintf("0x%x=%s", v.Key, v.Value)
}

// Parse k=v expression.
func (v *Value) Parse(s string) error {
	k, vv, ok := strings.Cut(s, "=")
	if !ok {
		return fmt.Errorf("invalid value %q", s)
	}

	key, err := strconv.ParseUint(k, 0, 8)
	if err != nil {
		return fmt.Errorf("invalid key %q", k)
	}

	v.Key = uint8(key)
	v.Value = vv

	return nil
}

// Values is a collection of Value.
type Values []Value

// Encode returns a string representation of Values for the environment variable.
//
// Each Value is encoded a k=v, split by ';' character.
// The result is base64 encoded.
func (v Values) Encode() string {
	return base64.StdEncoding.EncodeToString([]byte(strings.Join(xslices.Map(v, Value.String), ";")))
}

// DecodeValues parses a string representation of Values for the environment variable.
//
// See Encode for the details of the encoding.
func DecodeValues(s string) (Values, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}

	if len(b) == 0 {
		return nil, nil
	}

	parts := strings.Split(string(b), ";")

	result := make(Values, 0, len(parts))

	for _, v := range parts {
		var vv Value

		if err := vv.Parse(v); err != nil {
			return nil, err
		}

		result = append(result, vv)
	}

	return result, nil
}
