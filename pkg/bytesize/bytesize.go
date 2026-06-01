// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bytesize adds logic to help parse byte sizes in various forms such as gb, mb, GiB, etc.
package bytesize

import (
	"fmt"
	"unicode"

	"github.com/dustin/go-humanize"
)

// ByteSize implements pflag.Value interface and is meant to be used with flags specifying a size in bytes.
// A value can be set before assigning to a flag to function as a default value.
type ByteSize struct {
	valBytes    uint64
	val         string
	defaultUnit string
}

// WithDefaultUnit creates a New ByteSize with a default unit.
func WithDefaultUnit(unit string) *ByteSize {
	return &ByteSize{defaultUnit: unit}
}

// New creates a New ByteSize without a default unit.
func New() *ByteSize {
	return &ByteSize{}
}

// Bytes returns the value in bytes.
func (bs *ByteSize) Bytes() uint64 {
	return bs.valBytes
}

// Megabytes returns the value in megabytes.
func (bs *ByteSize) Megabytes() uint64 {
	return bs.valBytes / (1000 * 1000)
}

// Gigabytes returns the value in gigabytes.
func (bs *ByteSize) Gigabytes() uint64 {
	return bs.valBytes / (1000 * 1000 * 1000)
}

// Mebibytetes returns the value in binary megabytes.
func (bs *ByteSize) Mebibytetes() uint64 {
	return bs.valBytes / (1024 * 1024)
}

// Gibibytes returns the value in binary gigabytes.
func (bs *ByteSize) Gibibytes() uint64 {
	return bs.valBytes / (1024 * 1024 * 1024)
}

// String returns the string representation of the value with the default unit if set.
func (bs *ByteSize) String() string {
	return bs.val
}

// SetDefaultUnit sets the default unit to use in case one wasn't specifies.
func (bs *ByteSize) SetDefaultUnit(unit string) {
	bs.defaultUnit = unit
}

// Set implements pflag.Value interface.
func (bs *ByteSize) Set(in string) error {
	if in == "" || in == "0" {
		bs.val = "0"
		bs.valBytes = 0

		return nil
	}

	// if no unit is specified
	if unicode.IsDigit(rune(in[len(in)-1])) {
		if bs.defaultUnit == "" {
			return fmt.Errorf("no unit specified for %q", in)
		}

		in += bs.defaultUnit
	}

	valBytes, err := humanize.ParseBytes(in)
	if err != nil {
		return err
	}

	bs.val = in
	bs.valBytes = valBytes

	return nil
}

// Type implements pflag.Value interface (this will show up as the type next to the flag description).
func (bs *ByteSize) Type() string { return "string(mb,gb)" }
