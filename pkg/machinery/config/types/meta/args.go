// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta

import (
	"fmt"
	"maps"

	"go.yaml.in/yaml/v4"
)

// Args represents a map of argument names to their values.
type Args map[string]ArgValue

// ToMap converts Args to a map of string slices.
func (a Args) ToMap() map[string][]string {
	result := make(map[string][]string)

	for key, argValue := range a {
		// technically this shouldn't happen due to validation during unmarshalling
		// but just in case, we handle case when both are set
		value := make([]string, 0)

		if argValue.strValue != "" {
			value = append(value, argValue.strValue)
		}

		if argValue.listValue != nil {
			value = append(value, argValue.listValue...)
		}

		result[key] = value
	}

	return result
}

// Merge with another Args.
func (a *Args) Merge(other any) error {
	otherArgs, ok := other.(Args)
	if !ok {
		return fmt.Errorf("cannot merge Args with %T", other)
	}

	if len(otherArgs) == 0 {
		return nil
	}

	if *a == nil {
		*a = make(Args)
	}

	maps.Copy(*a, otherArgs)

	return nil
}

// ArgValue represents a value for an argument, which can be either a single string or a list of strings.
// docgen:nodoc
type ArgValue struct {
	listValue []string
	strValue  string
}

// NewArgValue creates a new ArgValue from either a string or a list of strings.
func NewArgValue(s string, l []string) ArgValue {
	return ArgValue{
		listValue: l,
		strValue:  s,
	}
}

// MarshalYAML is a custom marshaller for `ArgValue`.
func (a ArgValue) MarshalYAML() (any, error) {
	if a.listValue != nil {
		return &yaml.Node{
			Kind: yaml.SequenceNode,
			Tag:  "!!seq",
			Content: func() []*yaml.Node {
				nodes := make([]*yaml.Node, 0, len(a.listValue))

				for _, item := range a.listValue {
					nodes = append(nodes, &yaml.Node{
						Kind:  yaml.ScalarNode,
						Tag:   "!!str",
						Value: item,
					})
				}

				return nodes
			}(),
		}, nil
	}

	if a.strValue != "" {
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: a.strValue,
		}, nil
	}

	return nil, nil
}

// UnmarshalYAML is a custom unmarshaller for `ArgValue`.
func (a *ArgValue) UnmarshalYAML(unmarshal func(any) error) error {
	// Try scalar string first
	var s string
	if err := unmarshal(&s); err == nil {
		a.strValue = s
		a.listValue = nil

		return nil
	}

	// Then try list of strings
	var l []string
	if err := unmarshal(&l); err == nil {
		a.listValue = l
		a.strValue = ""

		return nil
	}

	return fmt.Errorf("arg value must be a string or list of strings")
}
