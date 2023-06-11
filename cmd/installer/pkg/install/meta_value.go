// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
)

// MetaValues is a list of MetaValue.
type MetaValues struct {
	values  []MetaValue
	changed bool
}

// MetaValue represents a key/value pair for META.
type MetaValue struct {
	Key   uint8
	Value string
}

func (v *MetaValue) String() string {
	return fmt.Sprintf("0x%x=%s", v.Key, v.Value)
}

// Parse k=v expression.
func (v *MetaValue) Parse(s string) error {
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

// Interface check.
var (
	_ pflag.Value      = &MetaValues{}
	_ pflag.SliceValue = &MetaValues{}
)

// Set implements pflag.Value.
func (s *MetaValues) Set(val string) error {
	var v MetaValue

	if err := v.Parse(val); err != nil {
		return err
	}

	if !s.changed {
		s.values = []MetaValue{v}
	} else {
		s.values = append(s.values, v)
	}

	s.changed = true

	return nil
}

// Type implements pflag.Value.
func (s *MetaValues) Type() string {
	return "metaValueSlice"
}

// String implements pflag.Value.
func (s *MetaValues) String() string {
	return "[" + strings.Join(s.GetSlice(), ",") + "]"
}

// Append implements pflag.SliceValue.
func (s *MetaValues) Append(val string) error {
	var v MetaValue

	if err := v.Parse(val); err != nil {
		return err
	}

	s.values = append(s.values, v)

	return nil
}

// Replace implements pflag.SliceValue.
func (s *MetaValues) Replace(val []string) error {
	out := make([]MetaValue, len(val))

	for i, pair := range val {
		var v MetaValue

		if err := v.Parse(pair); err != nil {
			return err
		}

		out[i] = v
	}

	s.values = out

	return nil
}

// GetSlice implements pflag.SliceValue.
func (s *MetaValues) GetSlice() []string {
	out := make([]string, len(s.values))

	for i, v := range s.values {
		out[i] = v.String()
	}

	return out
}
