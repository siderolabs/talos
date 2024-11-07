// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package encoder

import (
	"cmp"
	"encoding"
	"reflect"
	"slices"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// Encoder implements config encoder.
type Encoder struct {
	value   any
	options *Options
}

// NewEncoder initializes and returns an `Encoder`.
func NewEncoder(value any, opts ...Option) *Encoder {
	return &Encoder{
		value:   value,
		options: newOptions(opts...),
	}
}

// Marshal converts value to YAML-serializable value (suitable for MarshalYAML).
func (e *Encoder) Marshal() (*yaml.Node, error) {
	node, err := toYamlNode(e.value, e.options)
	if err != nil {
		return nil, err
	}

	if e.options.Comments.enabled(CommentsDocs) {
		addComments(node, getDoc(e.value), HeadComment, LineComment)
	}

	return node, nil
}

// Encode converts value to yaml.
//
//nolint:gocyclo
func (e *Encoder) Encode() ([]byte, error) {
	if e.options.Comments == CommentsDisabled {
		return yaml.Marshal(e.value)
	}

	node, err := e.Marshal()
	if err != nil {
		return nil, err
	}

	// special handling for case when we get an empty output
	if node.Kind == yaml.MappingNode && len(node.Content) == 0 && node.FootComment != "" && e.options.Comments.enabled(CommentsExamples) {
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

func isEmpty(value reflect.Value) bool {
	if !value.IsValid() {
		return true
	}

	//nolint:exhaustive
	switch value.Kind() {
	case reflect.Ptr:
		return value.IsNil()
	case reflect.Map:
		return len(value.MapKeys()) == 0
	case reflect.Slice:
		return value.Len() == 0
	default:
		return value.IsZero()
	}
}

func isNil(value reflect.Value) bool {
	if !value.IsValid() {
		return true
	}

	//nolint:exhaustive
	switch value.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

//nolint:gocyclo,cyclop
func toYamlNode(in any, options *Options) (*yaml.Node, error) {
	node := &yaml.Node{}

	flags := options.Comments

	// do not wrap yaml.Node into yaml.Node
	if n, ok := in.(*yaml.Node); ok {
		return n, nil
	}

	// if input implements yaml.Marshaler we should use that marshaller instead
	// same way as regular yaml marshal does
	if m, ok := in.(yaml.Marshaler); ok && !isNil(reflect.ValueOf(in)) {
		res, err := m.MarshalYAML()
		if err != nil {
			return nil, err
		}

		if n, ok := res.(*yaml.Node); ok {
			return n, nil
		}

		in = res
	}

	if _, ok := in.(encoding.TextMarshaler); ok && !isNil(reflect.ValueOf(in)) {
		return node, node.Encode(in)
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

		var examples []string

		for i := range v.NumField() {
			// skip unexported fields
			if !v.Field(i).CanInterface() {
				continue
			}

			tag := t.Field(i).Tag.Get("yaml")
			parts := strings.Split(tag, ",")
			fieldName := parts[0]
			parts = parts[1:]

			tag = t.Field(i).Tag.Get("talos")
			if tag != "" {
				parts = append(parts, strings.Split(tag, ",")...)
			}

			if fieldName == "" {
				fieldName = strings.ToLower(t.Field(i).Name)
			}

			if fieldName == "-" {
				continue
			}

			var (
				empty = isEmpty(v.Field(i))
				null  = isNil(v.Field(i))

				skip   bool
				inline bool
				flow   bool
			)

			for _, part := range parts {
				if part == "omitempty" && empty && options.OmitEmpty {
					skip = true
				}

				if part == "omitonlyifnil" && !null {
					skip = false
				}

				if part == "inline" {
					inline = true
				}

				if part == "flow" {
					flow = true
				}
			}

			var value any
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

			// inlineExample is rendered after the value
			var inlineExample string

			if empty && flags.enabled(CommentsExamples) && fieldDoc != nil {
				if skip {
					// render example to be appended to the end of the rendered struct
					example := renderExample(fieldName, fieldDoc, options)

					if example != "" {
						examples = append(examples, example)
					}
				} else {
					// render example to be appended to the empty field
					fieldDocCopy := *fieldDoc
					fieldDocCopy.Comments = [3]string{}

					inlineExample = renderExample("", &fieldDocCopy, options)
				}
			}

			if skip {
				continue
			}

			var style yaml.Style
			if flow {
				style |= yaml.FlowStyle
			}

			if inline {
				child, err := toYamlNode(value, options)
				if err != nil {
					return nil, err
				}

				if child.Kind == yaml.MappingNode || child.Kind == yaml.SequenceNode {
					appendNodes(node, child.Content...)
				}
			} else if err := addToMap(node, fieldDoc, fieldName, value, style, options); err != nil {
				return nil, err
			}

			if inlineExample != "" {
				nodeToAttach := node.Content[len(node.Content)-1]

				if nodeToAttach.FootComment != "" {
					nodeToAttach.FootComment += "\n"
				}

				nodeToAttach.FootComment += inlineExample
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
		slices.SortFunc(keys, func(a, b reflect.Value) int { return cmp.Compare(a.String(), b.String()) })

		for _, k := range keys {
			element := v.MapIndex(k)
			value := element.Interface()

			if err := addToMap(node, nil, k.Interface(), value, 0, options); err != nil {
				return nil, err
			}
		}
	case reflect.Slice:
		node.Kind = yaml.SequenceNode
		nodes := make([]*yaml.Node, v.Len())

		for i := range v.Len() {
			element := v.Index(i)

			var err error

			nodes[i], err = toYamlNode(element.Interface(), options)
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

func addToMap(dest *yaml.Node, doc *Doc, fieldName, in any, style yaml.Style, options *Options) error {
	key, err := toYamlNode(fieldName, options)
	if err != nil {
		return err
	}

	value, err := toYamlNode(in, options)
	if err != nil {
		return err
	}

	value.Style = style

	if options.Comments.enabled(CommentsDocs) {
		addComments(key, doc, HeadComment, FootComment)
		addComments(value, doc, LineComment)
	}

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
