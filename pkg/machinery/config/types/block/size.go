// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"encoding"
	"strings"

	"gopkg.in/yaml.v3"
)

// Check interfaces.
var (
	_ encoding.TextMarshaler   = Size{}
	_ encoding.TextUnmarshaler = (*PercentageSize)(nil)
	_ yaml.IsZeroer            = Size{}
)

// Size is either a PercentageSize or ByteSize.
type Size struct {
	PercentageSize *PercentageSize
	ByteSize       *ByteSize
}

// MustSize returns a new Size with the given value.
//
// It panics if the value is invalid.
func MustSize(value string) Size {
	var s Size

	if err := s.UnmarshalText([]byte(value)); err != nil {
		panic(err)
	}

	return s
}

// MustByteSize returns a new Size with the given ByteSize value.
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
func (s Size) Value() uint64 {
	if s.ByteSize != nil {
		return s.ByteSize.Value()
	}

	return 0
}

// RelativeValue returns the relative value.
func (s Size) RelativeValue() (uint64, bool) {
	if s.PercentageSize != nil {
		return s.PercentageSize.Value(), true
	}

	return 0, false
}

// MarshalText implements encoding.TextMarshaler.
func (s Size) MarshalText() ([]byte, error) {
	if s.ByteSize != nil {
		return s.ByteSize.MarshalText()
	}

	if s.PercentageSize != nil {
		return s.PercentageSize.MarshalText()
	}

	return nil, nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (s *Size) UnmarshalText(text []byte) error {
	if string(text) == "" {
		return nil
	}

	if strings.Contains(string(text), "%") {
		var ps PercentageSize
		if err := ps.UnmarshalText(text); err != nil {
			return err
		}

		s.PercentageSize = &ps
	} else {
		var bs ByteSize
		if err := bs.UnmarshalText(text); err != nil {
			return err
		}

		s.ByteSize = &bs
	}

	return nil
}

// IsZero implements yaml.IsZeroer.
func (s Size) IsZero() bool {
	return (s.PercentageSize == nil || s.PercentageSize.IsZero()) && (s.ByteSize == nil || s.ByteSize.IsZero())
}

// IsRelative returns if the Size is a relative size.
func (s Size) IsRelative() bool {
	return (s.PercentageSize != nil && !s.PercentageSize.IsZero())
}

// IsNegative returns true if the value is negative.
func (s Size) IsNegative() bool {
	if s.ByteSize != nil {
		return s.ByteSize.IsNegative()
	}

	if s.PercentageSize != nil {
		return s.PercentageSize.IsNegative()
	}

	return false
}
