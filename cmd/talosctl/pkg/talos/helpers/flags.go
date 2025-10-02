// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// choiceValue implements the [pflag.Value] interface.
type choiceValue struct {
	value    string
	validate func(string) error
}

// Set sets the value of the choice.
func (f *choiceValue) Set(s string) error {
	err := f.validate(s)
	if err != nil {
		return err
	}

	f.value = s

	return nil
}

// Type returns the type of the choice, which must be "string" for [pflag.FlagSet.GetString].
func (f *choiceValue) Type() string { return "string" }

// String returns the current value of the choice.
func (f *choiceValue) String() string { return f.value }

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

// ChainCobraPositionalArgs chains multiple cobra.PositionalArgs validators together.
func ChainCobraPositionalArgs(validators ...cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		for _, validator := range validators {
			if err := validator(cmd, args); err != nil {
				return err
			}
		}

		return nil
	}
}
