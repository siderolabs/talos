// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"bytes"
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

// ByteSize is a byte size which can be conveniently represented as a human readable string
// with IEC sizes, e.g. 100MB.
type ByteSize struct {
	value    *uint64
	raw      []byte
	negative bool
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

	negative := ""
	if bs.negative {
		negative = "-"
	}

	if bs.value != nil {
		return []byte(negative + strconv.FormatUint(*bs.value, 10)), nil
	}

	return nil, nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (bs *ByteSize) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return nil
	}

	raw := slices.Clone(text)

	if v, ok := bytes.CutPrefix(text, []byte("-")); ok {
		text = v
		bs.negative = true
	}

	value, err := humanize.ParseBytes(string(text))
	if err != nil {
		return err
	}

	bs.value = new(value)
	bs.raw = raw

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

// IsNegative returns true if the value is negative.
func (bs ByteSize) IsNegative() bool {
	return bs.negative
}
