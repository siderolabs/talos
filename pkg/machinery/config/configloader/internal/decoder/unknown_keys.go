// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package decoder

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

func checkUnknownKeys(target any, spec *yaml.Node) error {
	unknown, err := internalCheckUnknownKeys(reflect.TypeOf(target), spec)
	if err != nil {
		return err
	}

	if unknown != nil {
		var data []byte

		if data, err = yaml.Marshal(unknown); err != nil {
			return fmt.Errorf("failed to marshal error summary %w", err)
		}

		return fmt.Errorf("unknown keys found during decoding:\n%s", string(data))
	}

	return nil
}

// structKeys builds a set of known YAML fields by name and their indexes in the struct.
//
//nolint:gocyclo
func structKeys(typ reflect.Type) (map[string][]int, reflect.Type) {
	fields := reflect.VisibleFields(typ)

	availableKeys := make(map[string][]int, len(fields))

	for _, field := range fields {
		if tag := field.Tag.Get("yaml"); tag != "" {
			if tag == "-" {
				continue
			}

			idx := strings.IndexByte(tag, ',')

			inlined := false

			if idx >= 0 {
				options := strings.Split(tag[idx+1:], ",")

				for _, opt := range options {
					if opt == "inline" {
						inlined = true
					}
				}
			}

			// handle inlined `map` objects, inlining structs in general is not supported yet
			if inlined {
				inlinedTyp := field.Type

				if inlinedTyp.Kind() == reflect.Map {
					return nil, inlinedTyp
				}
			}

			if idx == -1 {
				availableKeys[tag] = field.Index
			} else if idx > 0 {
				availableKeys[tag[:idx]] = field.Index
			}
		} else {
			availableKeys[strings.ToLower(field.Name)] = field.Index
		}
	}

	return availableKeys, typ
}

var typeOfInterfaceAny = reflect.TypeOf((*any)(nil)).Elem()

//nolint:gocyclo,cyclop
func internalCheckUnknownKeys(typ reflect.Type, spec *yaml.Node) (unknown any, err error) {
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// anything can be unmarshaled into `interface{}`
	if typ == typeOfInterfaceAny {
		return nil, nil
	}

	switch spec.Kind { //nolint:exhaustive // not checking for scalar types
	case yaml.MappingNode:
		var availableKeys map[string][]int

		switch typ.Kind() { //nolint:exhaustive
		case reflect.Map:
			// any key is fine in the map
		case reflect.Struct:
			availableKeys, typ = structKeys(typ)
		default:
			return unknown, fmt.Errorf("unexpected type for yaml mapping: %s", typ)
		}

		for i := 0; i < len(spec.Content); i += 2 {
			keyNode := spec.Content[i]

			if keyNode.Kind != yaml.ScalarNode {
				return unknown, errors.New("unexpected mapping key type")
			}

			key := keyNode.Value

			var elemType reflect.Type

			switch typ.Kind() { //nolint:exhaustive
			case reflect.Struct:
				fieldIndex, ok := availableKeys[key]
				if !ok {
					if unknown == nil {
						unknown = map[string]any{}
					}

					unknown.(map[string]any)[key] = spec.Content[i+1]

					continue
				}

				elemType = typ.FieldByIndex(fieldIndex).Type
			case reflect.Map:
				elemType = typ.Elem()
			}

			// validate nested values
			innerUnknown, err := internalCheckUnknownKeys(elemType, spec.Content[i+1])
			if err != nil {
				return unknown, err
			}

			if innerUnknown != nil {
				if unknown == nil {
					unknown = map[string]any{}
				}

				unknown.(map[string]any)[key] = innerUnknown
			}
		}
	case yaml.SequenceNode:
		if typ.Kind() != reflect.Slice {
			return unknown, fmt.Errorf("unexpected type for yaml sequence: %s", typ)
		}

		for i := range len(spec.Content) {
			innerUnknown, err := internalCheckUnknownKeys(typ.Elem(), spec.Content[i])
			if err != nil {
				return unknown, err
			}

			if innerUnknown != nil {
				if unknown == nil {
					unknown = []any{}
				}

				unknown = append(unknown.([]any), innerUnknown)
			}
		}
	}

	return unknown, nil
}
