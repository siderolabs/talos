// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"fmt"
	"slices"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"github.com/spf13/pflag"
)

type choiceValue struct {
	value    string
	validate func(string) error
}

// Set implements pflag.Value interface.
func (v *choiceValue) Set(s string) error {
	err := v.validate(s)
	if err != nil {
		return err
	}

	v.value = s

	return nil
}

// Type implements pflag.Value interface.
func (v *choiceValue) Type() string { return "string" }

// String implements pflag.Value interface.
func (v *choiceValue) String() string { return v.value }

// StringChoice returns a [choiceValue] that validates the value against a set
// of choices. Only the last value will be used if multiple values are set.
func StringChoice(defaultValue string, otherChoices ...string) pflag.Value {
	return &choiceValue{
		value: defaultValue,
		validate: func(s string) error {
			choices := slices.Concat(otherChoices, []string{defaultValue})

			if slices.Contains(choices, s) {
				return nil
			}

			return fmt.Errorf("must be one of %v", choices)
		},
	}
}

type semverValue struct {
	value      semver.Version
	validators []SemverValidateFunc
}

// SemverValidateFunc allows setting restrictions on the version.
type SemverValidateFunc func(v semver.Version) error

// Set implements pflag.Value interface.
func (v *semverValue) Set(s string) error {
	vers, err := semver.ParseTolerant(s)
	if err != nil {
		return err
	}

	for _, validator := range v.validators {
		if err := validator(vers); err != nil {
			return err
		}
	}

	v.value = vers

	return nil
}

// Type implements pflag.Value interface.
func (v *semverValue) Type() string { return "semver" }

// String implements pflag.Value interface.
func (v *semverValue) String() string { return "v" + v.value.String() }

// Semver returns a pflag.Value that parses and stores a semantic version.
//
// Parsing is performed using semver.ParseTolerant. After parsing, any provided
// SemverValidateFunc validators are applied in order and may reject the version.
//
// The returned value is initialized with defaultValue, which is used until Set
// is called successfully.
func Semver(defaultValue string, validators ...SemverValidateFunc) pflag.Value {
	v, err := semver.ParseTolerant(defaultValue)
	if err != nil {
		panic(err)
	}

	return &semverValue{
		value:      v,
		validators: validators,
	}
}

type comparableStringer interface {
	~int32
	comparable
	fmt.Stringer
}

// PflagExtended extends pflag.Value with additional methods for retrieving the value as type T and getting valid string values.
type PflagExtended[T any] interface {
	pflag.Value

	Value() T
	Options() []string
}

type protoEnumValue[T comparableStringer] struct {
	value  T
	values map[string]int32
	names  map[int32]string
}

// Set implements pflag.Value interface.
func (v *protoEnumValue[T]) Set(s string) error {
	value, ok := v.values[strings.ToUpper(s)]
	if !ok {
		return fmt.Errorf("must be one of %v", v.Options())
	}

	v.value = T(value)

	return nil
}

// Type implements pflag.Value interface.
func (v *protoEnumValue[T]) Type() string { return "string" }

// String implements pflag.Value interface.
func (v *protoEnumValue[T]) String() string { return strings.ToLower(v.value.String()) }

// Value returns the enum value as the type T.
func (v *protoEnumValue[T]) Value() T {
	return v.value
}

// Options returns the valid string values for the enum.
func (v *protoEnumValue[T]) Options() []string {
	opts := xslices.Map(maps.Keys(v.values), strings.ToLower)

	slices.Sort(opts)

	return opts
}

// ProtoEnum returns a [protoEnumValue] that validates the value is correct.
// Only the last value will be used if multiple values are set.
func ProtoEnum[T comparableStringer](defaultValue T, values map[string]int32, names map[int32]string) PflagExtended[T] {
	return &protoEnumValue[T]{
		value:  defaultValue,
		values: values,
		names:  names,
	}
}
