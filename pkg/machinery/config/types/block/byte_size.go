// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"encoding"
	"fmt"
	"slices"
	"strconv"

	"github.com/dustin/go-humanize"
	"github.com/siderolabs/go-pointer"
	"go.yaml.in/yaml/v4"
)

// Check interfaces.
var (
	_ encoding.TextMarshaler   = ByteSize{}
	_ encoding.TextUnmarshaler = (*ByteSize)(nil)
	_ yaml.IsZeroer            = ByteSize{}
)

// ByteSize is a byte size which can be convienintly represented as a human readable string
// with IEC sizes, e.g. 100MB.
type ByteSize struct {
	value *uint64
	raw   []byte
}

// MustByteSize returns a new ByteSize with the given value.
//
// It panics if the value is invalid.
func MustByteSize(value string) ByteSize {
	var bs ByteSize

	if err := bs.UnmarshalText([]byte(value)); err != nil {
		panic(err)
	}

	return bs
}

// Value returns the value.
func (bs ByteSize) Value() uint64 {
	return pointer.SafeDeref(bs.value)
}

// MarshalText implements encoding.TextMarshaler.
func (bs ByteSize) MarshalText() ([]byte, error) {
	if bs.raw != nil {
		return bs.raw, nil
	}

	if bs.value != nil {
		return []byte(strconv.FormatUint(*bs.value, 10)), nil
	}

	return nil, nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (bs *ByteSize) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return nil
	}

	value, err := humanize.ParseBytes(string(text))
	if err != nil {
		return err
	}

	bs.value = pointer.To(value)
	bs.raw = slices.Clone(text)

	return nil
}

// IsZero implements yaml.IsZeroer.
func (bs ByteSize) IsZero() bool {
	return bs.value == nil && bs.raw == nil
}

// Merge implements merger interface.
func (bs *ByteSize) Merge(other any) error {
	otherBS, ok := other.(ByteSize)
	if !ok {
		return fmt.Errorf("cannot merge %T with %T", bs, other)
	}

	bs.raw = otherBS.raw
	bs.value = otherBS.value

	return nil
}
