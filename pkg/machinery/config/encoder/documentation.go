// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package encoder

import (
	"reflect"
	"regexp"
	"strings"
	"sync"

	yaml "gopkg.in/yaml.v3"
)

const (
	// HeadComment populates `yaml.Node` `HeadComment`.
	HeadComment = iota
	// LineComment populates `yaml.Node` `LineComment`.
	LineComment
	// FootComment populates `yaml.Node` `FootComment`.
	FootComment
)

// Doc represents a struct documentation rendered from comments by docgen.
type Doc struct {
	// Comments stores foot, line and head comments.
	Comments [3]string
	// Fields contains fields documentation if related item is a struct.
	Fields []Doc
	// Examples list of example values for the item.
	Examples []*Example
	// Values is only used to render valid values list in the documentation.
	Values []string
	// Description represents the full description for the item.
	Description string
	// Name represents struct name or field name.
	Name string
	// Type represents struct name or field type.
	Type string
	// Note is rendered as a note for the example in markdown file.
	Note string
	// AppearsIn describes back references for the type.
	AppearsIn []Appearance
}

// AddExample adds a new example snippet to the doc.
func (d *Doc) AddExample(name string, value interface{}) {
	if d.Examples == nil {
		d.Examples = []*Example{}
	}

	d.Examples = append(d.Examples, &Example{
		Name:  name,
		value: value,
	})
}

// Describe returns a field description.
func (d *Doc) Describe(field string, short bool) string {
	desc := ""

	for _, f := range d.Fields {
		if f.Name == field {
			desc = f.Description
		}
	}

	if short {
		desc = strings.Split(desc, "\n")[0]
	}

	return desc
}

// Example represents one example snippet for a type.
type Example struct {
	populate sync.Once
	Name     string

	valueMutex sync.RWMutex
	value      interface{}
}

// Populate populates example value.
func (e *Example) Populate(index int) {
	e.populate.Do(func() {
		if reflect.TypeOf(e.value).Kind() != reflect.Ptr {
			return
		}

		v := reflect.ValueOf(e.value).Elem()

		defaultValue := getExample(v, getDoc(e.value), index)

		e.valueMutex.Lock()
		defer e.valueMutex.Unlock()

		if defaultValue != nil {
			v.Set(defaultValue.Convert(v.Type()))
		}

		populateNestedExamples(v, index)
	})
}

// GetValue returns example value.
func (e *Example) GetValue() interface{} {
	e.valueMutex.RLock()
	defer func() {
		e.valueMutex.RUnlock()
	}()

	return e.value
}

// Field gets field from the list of fields.
func (d *Doc) Field(i int) *Doc {
	if i < len(d.Fields) {
		return &d.Fields[i]
	}

	return nil
}

// Appearance of a type in a different type.
type Appearance struct {
	TypeName  string
	FieldName string
}

// Documented is used to check if struct has any documentation defined for it.
type Documented interface {
	// Doc requests documentation object.
	Doc() *Doc
}

func mergeDoc(a, b *Doc) *Doc {
	var res Doc
	if a != nil {
		res = *a
	}

	if b == nil {
		return &res
	}

	for i, comment := range b.Comments {
		if comment != "" {
			res.Comments[i] = comment
		}
	}

	if len(res.Examples) == 0 {
		res.Examples = b.Examples
	}

	return &res
}

func getDoc(in interface{}) *Doc {
	v := reflect.ValueOf(in)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		in = reflect.New(v.Type().Elem()).Interface()
	}

	if d, ok := in.(Documented); ok {
		return d.Doc()
	}

	return nil
}

func addComments(node *yaml.Node, doc *Doc, comments ...int) {
	if doc != nil {
		dest := []*string{
			&node.HeadComment,
			&node.LineComment,
			&node.FootComment,
		}

		if len(comments) == 0 {
			comments = []int{
				HeadComment,
				LineComment,
				FootComment,
			}
		}

		for _, i := range comments {
			if doc.Comments[i] != "" {
				*dest[i] = doc.Comments[i]
			}
		}
	}
}

//nolint:gocyclo
func renderExample(key string, doc *Doc, flags CommentsFlags) string {
	if doc == nil {
		return ""
	}

	examples := []string{}

	for i, e := range doc.Examples {
		v := reflect.ValueOf(e.GetValue())

		if isEmpty(v) {
			continue
		}

		if v.Kind() != reflect.Ptr {
			v = reflect.Indirect(v)
		}

		defaultValue := v.Interface()

		e.Populate(i)

		node, err := toYamlNode(defaultValue, flags)
		if err != nil {
			continue
		}

		if key != "" {
			node, err = toYamlNode(map[string]*yaml.Node{
				key: node,
			}, flags)
			if err != nil {
				continue
			}
		}

		if i == 0 && flags.enabled(CommentsDocs) {
			addComments(node, doc, HeadComment, LineComment)
		}

		// replace head comment with line comment
		if node.HeadComment == "" {
			node.HeadComment = node.LineComment
		}

		node.LineComment = ""
		if e.Name != "" {
			if node.HeadComment != "" {
				node.HeadComment += "\n\n"
			}

			node.HeadComment = node.HeadComment + e.Name + "\n"
		}

		data, err := yaml.Marshal(node)
		if err != nil {
			continue
		}

		if key == "" {
			// re-indent
			data = regexp.MustCompile(`(?m)^(.)`).ReplaceAll(data, []byte("  $1"))
		} else {
			// don't collapse comment
			data = regexp.MustCompile(`(?m)^#`).ReplaceAll(data, []byte("# #"))
		}

		examples = append(examples, string(data))
	}

	return strings.Join(examples, "")
}

func getExample(v reflect.Value, doc *Doc, index int) *reflect.Value {
	if doc == nil || len(doc.Examples) == 0 {
		return nil
	}

	numExamples := len(doc.Examples)
	if index >= numExamples {
		index = numExamples - 1
	}

	defaultValue := reflect.ValueOf(doc.Examples[index].GetValue())
	if !isEmpty(defaultValue) {
		if v.Kind() != reflect.Ptr && defaultValue.Kind() == reflect.Ptr {
			defaultValue = defaultValue.Elem()
		}
	}

	return &defaultValue
}

//nolint:gocyclo
func populateNestedExamples(v reflect.Value, index int) {
	//nolint:exhaustive
	switch v.Kind() {
	case reflect.Struct:
		doc := getDoc(v.Interface())

		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if !field.CanInterface() {
				continue
			}

			if doc != nil && i < len(doc.Fields) {
				defaultValue := getExample(field, doc.Field(i), index)

				if defaultValue != nil {
					field.Set(defaultValue.Convert(field.Type()))
				}
			}

			populateNestedExamples(field, index)
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			populateNestedExamples(v.MapIndex(key), index)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			populateNestedExamples(v.Index(i), index)
		}
	}
}
