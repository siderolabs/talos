// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package decoder

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// Selector represents a delete selector for a document.
type Selector interface {
	config.Document
	DocIdx() int
	ApplyTo(config.Document) error
}

type selector struct {
	path          []string
	docIdx        int
	docAPIVersion string
	docKind       string
	key           string
	value         string
}

func (s *selector) Kind() string           { return s.docKind }
func (s *selector) APIVersion() string     { return s.docAPIVersion }
func (s *selector) Clone() config.Document { return new(s.clone()) }
func (s *selector) DocIdx() int            { return s.docIdx }

func (s *selector) PathAsString() string { return strings.Join(s.path, ".") }

func (s *selector) clone() selector {
	return selector{
		path:          slices.Clone(s.path),
		docIdx:        s.docIdx,
		docAPIVersion: s.docAPIVersion,
		docKind:       s.docKind,
		key:           s.key,
		value:         s.value,
	}
}

func (s *selector) String() string { return s.toString("") }

func (s *selector) toString(more string) string {
	var builder strings.Builder

	writeThing := func(key, val string) {
		if val != "" {
			if builder.Len() > 1 {
				builder.WriteString(", ")
			}

			builder.WriteString(key)
			builder.WriteRune(':')
			builder.WriteString(val)
		}
	}

	builder.WriteRune('{')
	writeThing("path", s.PathAsString())
	writeThing("apiVersion", s.docAPIVersion)
	writeThing("kind", s.docKind)
	writeThing("key", s.key)
	writeThing("value", s.value)
	writeThing("idx", strconv.Itoa(s.docIdx))

	if more != "" {
		builder.WriteString(", ")
		builder.WriteString(more)
	}

	builder.WriteRune('}')

	return builder.String()
}

// ErrZeroedDocument is returned when the document is empty after applying the delete selector.
var ErrZeroedDocument = errors.New("document is empty now")

// ApplyTo applies the delete selector to the given document.
func (s *selector) ApplyTo(doc config.Document) error {
	if err := s.applyTo(doc); err != nil {
		return fmt.Errorf("patch delete: path '%s' in document '%s/%s': %w", s.PathAsString(), doc.APIVersion(), doc.Kind(), err)
	}

	return nil
}

func (s *selector) applyTo(doc config.Document) error {
	if s.docKind != doc.Kind() || s.docAPIVersion != doc.APIVersion() {
		return fmt.Errorf(
			"incorrect document type for %s/%s",
			s.docAPIVersion,
			s.docKind,
		)
	}

	val := reflect.ValueOf(doc)

	if val.Kind() != reflect.Pointer {
		return fmt.Errorf("document type is not a pointer")
	}

	if len(s.path) == 0 {
		if doc.Kind() == "" {
			return errors.New("can't delete the root of the legacy document")
		}

		return ErrZeroedDocument
	}

	err := deleteForPath(val.Elem(), s.path, s.key, s.value)
	if err != nil {
		return fmt.Errorf("failed to delete path '%s': %w", s.PathAsString(), err)
	}

	return nil
}

var searchForType = reflect.TypeFor[string]()

// ErrLookupFailed is returned when the lookup failed.
var ErrLookupFailed = errors.New("lookup failed")

//nolint:gocyclo
func deleteForPath(val reflect.Value, path []string, key, value string) error {
	if len(path) == 0 {
		return errors.New("path is empty")
	}

	if val.Kind() == reflect.Pointer || val.Kind() == reflect.Interface {
		if val.IsNil() {
			return ErrLookupFailed
		}

		return deleteForPath(val.Elem(), path, key, value)
	}

	searchFor := path[0]
	path = path[1:]
	valType := val.Type()

	switch val.Kind() { //nolint:exhaustive
	case reflect.Struct:
		// Lookup using yaml tag
		for i := range val.NumField() {
			structField := valType.Field(i)

			yamlTagRaw, ok := structField.Tag.Lookup("yaml")
			if !ok {
				continue
			}

			yamlTags := strings.Split(yamlTagRaw, ",")
			if yamlTags[0] == searchFor {
				if len(path) == 0 {
					val.Field(i).SetZero()

					return nil
				}

				return deleteForPath(val.Field(i), path, key, value)
			}
		}
	case reflect.Map:
		if val.IsNil() {
			break
		}

		keyType := valType.Key()

		// Try assingable and convertible types for key search
		if searchForType.AssignableTo(keyType) || searchForType.ConvertibleTo(keyType) {
			searchForVal := reflect.ValueOf(searchFor)

			if searchForType != keyType {
				searchForVal = searchForVal.Convert(keyType)
			}

			if idx := val.MapIndex(searchForVal); idx.IsValid() {
				if len(path) == 0 {
					val.SetMapIndex(searchForVal, reflect.Value{})

					return nil
				}

				return deleteForPath(idx, path, key, value)
			}
		}
	case reflect.Slice:
		return deleteStructFrom(val, searchFor, path, key, value)
	}

	return ErrLookupFailed
}

//nolint:gocyclo
func deleteStructFrom(searchIn reflect.Value, searchFor string, path []string, key, value string) error {
	switch {
	case len(path) != 0:
		return errors.New("searching for complex paths in slices is not supported")
	case searchFor == "":
		return errors.New("searching for '' in a slice is not supported")
	case searchFor[0] != '[':
		return errors.New("searching for non-integer keys in slices is not supported")
	case searchIn.Kind() != reflect.Slice:
		return errors.New("searching for a key in a non-slice")
	}

	for i := 0; i < searchIn.Len(); i++ { //nolint:intrange
		elem := searchIn.Index(i)

		for elem.Kind() == reflect.Pointer {
			elem = elem.Elem()
		}

		if elem.Kind() != reflect.Struct && elem.Kind() != reflect.Map {
			continue
		}

		elemType := elem.Type()

		if elem.Kind() == reflect.Struct {
			for j := range elemType.NumField() {
				structField := elemType.Field(j)

				yamlTagRaw, ok := structField.Tag.Lookup("yaml")
				if !ok {
					continue
				}

				yamlTags := strings.Split(yamlTagRaw, ",")
				if yamlTags[0] != key {
					continue
				}

				if elem.Field(j).String() != value {
					continue
				}

				searchIn.Set(reflect.AppendSlice(searchIn.Slice(0, i), searchIn.Slice(i+1, searchIn.Len())))

				return nil
			}
		} else {
			continue
		}
	}

	return ErrLookupFailed
}

type namedSelector struct {
	selector

	name string
}

func (n *namedSelector) Name() string   { return n.name }
func (n *namedSelector) String() string { return n.toString("name:" + n.name) }
func (n *namedSelector) Clone() config.Document {
	return &namedSelector{selector: n.selector.clone(), name: n.name}
}

// ApplyTo applies the delete selector to the given document.
func (n *namedSelector) ApplyTo(doc config.Document) error {
	if err := n.applyTo(doc); err != nil {
		return fmt.Errorf("named patch delete: document %s/%s: %w", doc.APIVersion(), doc.Kind(), err)
	}

	return nil
}

func (n *namedSelector) applyTo(doc config.Document) error {
	namedDoc, ok := doc.(config.NamedDocument)
	if !ok {
		return errors.New("not a named document, expected " + n.name)
	}

	if n.name != namedDoc.Name() {
		return fmt.Errorf("name mismatch, expected %s, got %s", n.name, namedDoc.Name())
	}

	return n.selector.applyTo(doc)
}
