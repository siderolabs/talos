// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package output

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"k8s.io/client-go/util/jsonpath"
)

// Table outputs resources in Table view.
type Table struct {
	w              tabwriter.Writer
	withEvents     bool
	displayType    string
	dynamicColumns []dynamicColumn
}

type dynamicColumn func(value interface{}) (string, error)

// NewTable initializes table resource output.
func NewTable() *Table {
	output := &Table{}
	output.w.Init(os.Stdout, 0, 0, 3, ' ', 0)

	return output
}

// WriteHeader implements output.Writer interface.
func (table *Table) WriteHeader(definition resource.Resource, withEvents bool) error {
	table.withEvents = withEvents
	fields := []string{"NAMESPACE", "TYPE", "ID", "VERSION"}

	if withEvents {
		fields = append([]string{"*"}, fields...)
	}

	resourceDefinitionSpec := definition.(*resource.Any).Value().(map[string]interface{}) //nolint:errcheck,forcetypeassert

	table.displayType = resourceDefinitionSpec["displayType"].(string) //nolint:errcheck,forcetypeassert

	for _, col := range resourceDefinitionSpec["printColumns"].([]interface{}) {
		column := col.(map[string]interface{}) //nolint:errcheck,forcetypeassert
		name := column["name"].(string)        //nolint:errcheck,forcetypeassert

		fields = append(fields, strings.ToUpper(name))

		expr := jsonpath.New(name)
		if err := expr.Parse(column["jsonPath"].(string)); err != nil {
			return fmt.Errorf("error parsing column %q jsonpath: %w", name, err)
		}

		expr = expr.AllowMissingKeys(true)

		table.dynamicColumns = append(table.dynamicColumns, func(val interface{}) (string, error) {
			var buf bytes.Buffer

			if e := expr.Execute(&buf, val); e != nil {
				return "", e
			}

			return buf.String(), nil
		})
	}

	fields = append([]string{"NODE"}, fields...)

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
		}

		values = append([]string{label}, values...)
	}

	for _, dynamicColumn := range table.dynamicColumns {
		value, err := dynamicColumn(r.(*resource.Any).Value())
		if err != nil {
			return err
		}

		values = append(values, value)
	}

	values = append([]string{node}, values...)

	_, err := fmt.Fprintln(&table.w, strings.Join(values, "\t"))

	return err
}

// Flush implements output.Writer interface.
func (table *Table) Flush() error {
	return table.w.Flush()
}
