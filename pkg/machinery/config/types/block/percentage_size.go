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

	"github.com/siderolabs/go-pointer"
	"gopkg.in/yaml.v3"
)

// Check interfaces.
var (
	_ encoding.TextMarshaler   = PercentageSize{}
	_ encoding.TextUnmarshaler = (*PercentageSize)(nil)
	_ yaml.IsZeroer            = PercentageSize{}
)

// PercentageSize is a size in percents.
type PercentageSize struct {
	value *uint64
	raw   []byte
}

// Value returns the value.
func (ps PercentageSize) Value() uint64 {
	return pointer.SafeDeref(ps.value)
}

// MarshalText implements encoding.TextMarshaler.
func (ps PercentageSize) MarshalText() ([]byte, error) {
	if ps.raw != nil {
		return ps.raw, nil
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

	numStr := string(text[:len(text)-1])

	value, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return fmt.Errorf("invalid percentage value: %w", err)
	}

	if value < 0 || value > 100 {
		return fmt.Errorf("percentage must be between 0 and 100, got %v", value)
	}

	ps.value = pointer.To(uint64(value))
	ps.raw = slices.Clone(text)

	return nil
}

// IsZero implements yaml.IsZeroer.
func (ps PercentageSize) IsZero() bool {
	return ps.value == nil && ps.raw == nil
}
