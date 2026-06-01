// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"k8s.io/client-go/third_party/forked/golang/template"
	"k8s.io/client-go/util/jsonpath"
)

// JSONPath outputs resources in JSONPath format.
type JSONPath struct {
	jsonPath *jsonpath.JSONPath
	json     *JSON
	writer   io.Writer
}

// NewJSONPath initializes JSONPath resource output.
func NewJSONPath(writer io.Writer, jsonPath *jsonpath.JSONPath) *JSONPath {
	return &JSONPath{
		jsonPath: jsonPath,
		json:     NewJSON(writer),
		writer:   writer,
	}
}

// WriteHeader implements output.Writer interface.
func (j *JSONPath) WriteHeader(definition *meta.ResourceDefinition, withEvents bool) error {
	return j.json.WriteHeader(definition, withEvents)
}

// printResult prints a reflect.Value as JSON if it's a map, array, slice or struct.
// But if it's just a 'scalar' type it prints it as a mere string.
func printResult(wr io.Writer, result reflect.Value) error {
	kind := result.Kind()
	if kind == reflect.Interface {
		kind = result.Elem().Kind()
	}

	outputJSON := kind == reflect.Map ||
		kind == reflect.Array ||
		kind == reflect.Slice ||
		kind == reflect.Struct

	var text []byte //nolint:prealloc // dynamic

	var err error

	if outputJSON {
		text, err = json.MarshalIndent(result.Interface(), "", "    ")
		if err != nil {
			return err
		}
	} else {
		text, err = valueToText(result)
	}

	if err != nil {
		return err
	}

	text = append(text, '\n')

	if _, err = wr.Write(text); err != nil {
		return err
	}

	return nil
}

// valueToText translates reflect value to corresponding text.
func valueToText(v reflect.Value) ([]byte, error) {
	iface, ok := template.PrintableValue(v)
	if !ok {
		return nil, fmt.Errorf("can't translate type %s to text", v.Type())
	}

	var buffer bytes.Buffer

	fmt.Fprint(&buffer, iface)

	return buffer.Bytes(), nil
}

// WriteResource implements output.Writer interface.
func (j *JSONPath) WriteResource(node string, r resource.Resource, event state.EventType) error {
	data, err := j.json.prepareEncodableData(node, r, event)
	if err != nil {
		return err
	}

	results, err := j.jsonPath.FindResults(data)
	if err != nil {
		return fmt.Errorf("error finding result for jsonpath: %w", err)
	}

	j.jsonPath.EnableJSONOutput(true)

	for _, resultGroup := range results {
		for _, result := range resultGroup {
			err = printResult(j.writer, result)
			if err != nil {
				return fmt.Errorf("error generating jsonpath results: %w", err)
			}
		}
	}

	return nil
}

// Flush implements output.Writer interface.
func (j *JSONPath) Flush() error {
	return nil
}
