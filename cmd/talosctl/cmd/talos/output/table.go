// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package output

import (
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/util/jsonpath"
)

// Table outputs resources in Table view.
type Table struct {
	w              tabwriter.Writer
	withEvents     bool
	displayType    string
	dynamicColumns []dynamicColumn
}

type dynamicColumn func(value any) (string, error)

// NewTable initializes table resource output.
func NewTable(writer io.Writer) *Table {
	output := &Table{}
	output.w.Init(writer, 0, 0, 3, ' ', 0)

	return output
}

// WriteHeader implements output.Writer interface.
func (table *Table) WriteHeader(definition *meta.ResourceDefinition, withEvents bool) error {
	table.withEvents = withEvents
	fields := []string{"NAMESPACE", "TYPE", "ID", "VERSION"}

	if withEvents {
		fields = slices.Insert(fields, 0, "*")
	}

	table.displayType = definition.TypedSpec().DisplayType

	for _, column := range definition.TypedSpec().PrintColumns {
		name := column.Name

		fields = append(fields, strings.ToUpper(name))

		expr := jsonpath.New(name)
		if err := expr.Parse(column.JSONPath); err != nil {
			return fmt.Errorf("error parsing column %q jsonpath: %w", name, err)
		}

		expr = expr.AllowMissingKeys(true)

		table.dynamicColumns = append(table.dynamicColumns, func(val any) (string, error) {
			var buf bytes.Buffer

			if e := expr.Execute(&buf, val); e != nil {
				return "", e
			}

			return buf.String(), nil
		})
	}

	fields = slices.Insert(fields, 0, "NODE")

	_, err := fmt.Fprintln(&table.w, strings.Join(fields, "\t"))

	return err
}

// WriteResource implements output.Writer interface.
func (table *Table) WriteResource(node string, r resource.Resource, event state.EventType) error {
	values := []string{r.Metadata().Namespace(), table.displayType, r.Metadata().ID(), r.Metadata().Version().String()}

	if table.withEvents {
		var label string

		switch event {
		case state.Created:
			label = "+"
		case state.Destroyed:
			label = "-"
		case state.Updated:
			label = " "
		case state.Bootstrapped, state.Errored, state.Noop:
			return nil
		}

		values = slices.Insert(values, 0, label)
	}

	yml, err := yaml.Marshal(r.Spec())
	if err != nil {
		return err
	}

	var unstructured any

	if err = yaml.Unmarshal(yml, &unstructured); err != nil {
		return err
	}

	for _, dynamicColumn := range table.dynamicColumns {
		var value string

		value, err = dynamicColumn(unstructured)
		if err != nil {
			return err
		}

		values = append(values, value)
	}

	values = slices.Insert(values, 0, node)

	_, err = fmt.Fprintln(&table.w, strings.Join(values, "\t"))

	return err
}

// Flush implements output.Writer interface.
func (table *Table) Flush() error {
	return table.w.Flush()
}
