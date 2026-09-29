// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta

import (
	"bytes"
	"encoding"
	"fmt"
	"slices"
	"strconv"

	"github.com/siderolabs/go-pointer"
	"go.yaml.in/yaml/v4"
)

// Check interfaces.
var (
	_ encoding.TextMarshaler   = PercentageSize{}
	_ encoding.TextUnmarshaler = (*PercentageSize)(nil)
	_ yaml.IsZeroer            = PercentageSize{}
)

// PercentageSize is a size in percents.
//
//docgen:nodoc
type PercentageSize struct {
	value    *uint64
	raw      []byte
	negative bool
}

// Value returns the value.
func (ps PercentageSize) Value() uint64 {
	return pointer.SafeDeref(ps.value)
}

// MarshalText implements encoding.TextMarshaler.
func (ps PercentageSize) MarshalText() ([]byte, error) {
	if ps.raw != nil {
		// the internal buffer is never handed out, as the caller might mutate it.
		return slices.Clone(ps.raw), nil
	}

	if ps.value != nil {
		return []byte(strconv.FormatUint(*ps.value, 10)), nil
	}

	return nil, nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (ps *PercentageSize) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		ps.value = nil
		ps.raw = nil

		return nil
	}

	if !bytes.HasSuffix(text, []byte("%")) {
		return fmt.Errorf("percentage must end with '%%'")
	}

	raw := slices.Clone(text)

	if v, ok := bytes.CutPrefix(text, []byte("-")); ok {
		text = v
		ps.negative = true
	}

	numStr := string(text[:len(text)-1])

	value, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return fmt.Errorf("invalid percentage value: %w", err)
	}

	if value < 0 || value > 100 {
		return fmt.Errorf("percentage must be between 0 and 100, got %v", value)
	}

	ps.value = new(uint64(value))
	ps.raw = raw

	return nil
}

// IsZero implements yaml.IsZeroer.
func (ps PercentageSize) IsZero() bool {
	return ps.value == nil && ps.raw == nil
}

// IsNegative returns true if the value is negative.
func (ps PercentageSize) IsNegative() bool {
	return ps.negative
}

// Merge implements merger interface.
//
// A zero value is skipped: the generic merge consults this interface before its own
// zero check, so a patch which omits the field must not erase the base value.
func (ps *PercentageSize) Merge(other any) error {
	otherPS, ok := other.(PercentageSize)
	if !ok {
		return fmt.Errorf("cannot merge %T with %T", ps, other)
	}

	if otherPS.IsZero() {
		return nil
	}

	*ps = otherPS.DeepCopy()

	return nil
}

// DeepCopy generates a deep copy of PercentageSize.
//
// The unexported fields are not reachable from other packages, so the deep copy generator
// cannot copy them on its own: it reuses this method instead.
func (ps PercentageSize) DeepCopy() PercentageSize {
	cp := ps

	if ps.value != nil {
		cp.value = new(*ps.value)
	}

	cp.raw = slices.Clone(ps.raw)

	return cp
}
