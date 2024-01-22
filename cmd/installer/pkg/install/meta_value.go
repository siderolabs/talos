// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"strings"

	"github.com/spf13/pflag"

	"github.com/siderolabs/talos/pkg/machinery/meta"
)

// MetaValues is a list of MetaValue.
type MetaValues struct {
	values  meta.Values
	changed bool
}

// Interface check.
var (
	_ pflag.Value      = &MetaValues{}
	_ pflag.SliceValue = &MetaValues{}
)

// FromMeta returns a new MetaValues from a meta.Values.
func FromMeta(values meta.Values) MetaValues {
	return MetaValues{values: values}
}

// Set implements pflag.Value.
func (s *MetaValues) Set(val string) error {
	var v meta.Value

	if err := v.Parse(val); err != nil {
		return err
	}

	if !s.changed {
		s.values = meta.Values{v}
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
	var v meta.Value

	if err := v.Parse(val); err != nil {
		return err
	}

	s.values = append(s.values, v)

	return nil
}

// Replace implements pflag.SliceValue.
func (s *MetaValues) Replace(val []string) error {
	out := make(meta.Values, len(val))

	for i, pair := range val {
		var v meta.Value

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

// Encode returns the encoded values.
func (s *MetaValues) Encode() string {
	return s.values.Encode(false)
}

// Decode the values from the given string.
func (s *MetaValues) Decode(val string) error {
	values, err := meta.DecodeValues(val)
	if err != nil {
		return err
	}

	s.values = values

	return nil
}

// GetMetaValues returns the wrapped meta.Values.
func (s *MetaValues) GetMetaValues() meta.Values {
	return s.values
}
