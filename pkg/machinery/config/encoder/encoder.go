// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package encoder

import (
	"reflect"
	"sort"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// Encoder implements config encoder.
type Encoder struct {
	value interface{}
}

// NewEncoder initializes and returns an `Encoder`.
func NewEncoder(value interface{}) *Encoder {
	return &Encoder{
		value: value,
	}
}

// Marshal converts value to YAML-serializable value (suitable for MarshalYAML).
func (e *Encoder) Marshal() (*yaml.Node, error) {
	node, err := toYamlNode(e.value)
	if err != nil {
		return nil, err
	}

	addComments(node, getDoc(e.value), HeadComment, LineComment)

	return node, nil
}

// Encode converts value to yaml.
func (e *Encoder) Encode() ([]byte, error) {
	node, err := e.Marshal()
	if err != nil {
		return nil, err
	}

	// special handling for case when we get an empty output
	if node.Kind == yaml.MappingNode && len(node.Content) == 0 && node.FootComment != "" {
		res := ""

		if node.HeadComment != "" {
			res += node.HeadComment + "\n"
		}

		lines := strings.Split(res+node.FootComment, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
				continue
			}

			lines[i] = "# " + line
		}

		return []byte(strings.Join(lines, "\n")), nil
	}

	return yaml.Marshal(node)
}

func isSet(value reflect.Value) bool {
	if !value.IsValid() {
		return false
	}

	//nolint:exhaustive
	switch value.Kind() {
	case reflect.Ptr:
		return !value.IsNil()
	case reflect.Map:
		return len(value.MapKeys()) != 0
	case reflect.Slice:
		return value.Len() > 0
	default:
		return !value.IsZero()
	}
}

//nolint:gocyclo
func toYamlNode(in interface{}) (*yaml.Node, error) {
	node := &yaml.Node{}

	// do not wrap yaml.Node into yaml.Node
	if n, ok := in.(*yaml.Node); ok {
		return n, nil
	}

	// if input implements yaml.Marshaler we should use that marshaller instead
	// same way as regular yaml marshal does
	if m, ok := in.(yaml.Marshaler); ok {
		res, err := m.MarshalYAML()
		if err != nil {
			return nil, err
		}

		if n, ok := res.(*yaml.Node); ok {
			return n, nil
		}

		in = res
	}

	v := reflect.ValueOf(in)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	doc := getDoc(in)

	//nolint:exhaustive
	switch v.Kind() {
	case reflect.Struct:
		node.Kind = yaml.MappingNode

		t := v.Type()

		examples := []string{}

		for i := 0; i < v.NumField(); i++ {
			// skip unexported fields
			if !v.Field(i).CanInterface() {
				continue
			}

			tag := t.Field(i).Tag.Get("yaml")
			parts := strings.Split(tag, ",")
			fieldName := parts[0]

			if fieldName == "" {
				fieldName = strings.ToLower(t.Field(i).Name)
			}

			if fieldName == "-" {
				continue
			}

			var (
				defined bool = isSet(v.Field(i))

				skip   bool
				inline bool
				flow   bool
			)

			for i, part := range parts {
				// always skip the first argument
				if i == 0 {
					continue
				}

				if part == "omitempty" && !defined {
					skip = true
				}

				if part == "inline" {
					inline = true
				}

				if part == "flow" {
					flow = true
				}
			}

			var value interface{}
			if v.Field(i).CanInterface() {
				value = v.Field(i).Interface()
			}

			// get documentation data either from field, or from type
			var fieldDoc *Doc

			if doc != nil {
				fieldDoc = mergeDoc(getDoc(value), doc.Field(i))
			} else {
				fieldDoc = getDoc(value)
			}

			if !defined {
				example := renderExample(fieldName, fieldDoc)

				if example != "" {
					examples = append(examples, example)
					skip = true
				}
			}

			var style yaml.Style
			if flow {
				style |= yaml.FlowStyle
			}

			if !skip {
				if inline {
					child, err := toYamlNode(value)
					if err != nil {
						return nil, err
					}

					if child.Kind == yaml.MappingNode || child.Kind == yaml.SequenceNode {
						appendNodes(node, child.Content...)
					}
				} else if err := addToMap(node, fieldDoc, fieldName, value, style); err != nil {
					return nil, err
				}
			}
		}

		if len(examples) > 0 {
			comment := strings.Join(examples, "\n")
			// add rendered example to the foot comment of the last node
			// or to the foot comment of parent node
			if len(node.Content) > 0 {
				node.Content[len(node.Content)-2].FootComment += "\n" + comment
			} else {
				node.FootComment += comment
			}
		}
	case reflect.Map:
		node.Kind = yaml.MappingNode
		keys := v.MapKeys()
		// always interate keys in alphabetical order to preserve the same output for maps
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})

		for _, k := range keys {
			element := v.MapIndex(k)
			value := element.Interface()

			if err := addToMap(node, nil, k.Interface(), value, 0); err != nil {
				return nil, err
			}
		}
	case reflect.Slice:
		node.Kind = yaml.SequenceNode
		nodes := make([]*yaml.Node, v.Len())

		for i := 0; i < v.Len(); i++ {
			element := v.Index(i)

			var err error

			nodes[i], err = toYamlNode(element.Interface())
			if err != nil {
				return nil, err
			}
		}
		appendNodes(node, nodes...)
	default:
		if err := node.Encode(in); err != nil {
			return nil, err
		}
	}

	return node, nil
}

func appendNodes(dest *yaml.Node, nodes ...*yaml.Node) {
	if dest.Content == nil {
		dest.Content = []*yaml.Node{}
	}

	dest.Content = append(dest.Content, nodes...)
}

func addToMap(dest *yaml.Node, doc *Doc, fieldName, in interface{}, style yaml.Style) error {
	key, err := toYamlNode(fieldName)
	if err != nil {
		return err
	}

	value, err := toYamlNode(in)
	if err != nil {
		return err
	}

	value.Style = style

	addComments(key, doc, HeadComment, FootComment)
	addComments(value, doc, LineComment)

	// override head comment with line comment for non-scalar nodes
	if value.Kind != yaml.ScalarNode {
		if key.HeadComment == "" {
			key.HeadComment = value.LineComment
		}

		value.LineComment = ""
	}

	appendNodes(dest, key, value)

	return nil
}
