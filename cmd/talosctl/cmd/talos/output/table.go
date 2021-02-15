// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package output

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"
)

// Table outputs resources in Table view.
type Table struct {
	w          tabwriter.Writer
	withEvents bool
}

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

	fields = append([]string{"NODE"}, fields...)

	_, err := fmt.Fprintln(&table.w, strings.Join(fields, "\t"))

	return err
}

// WriteResource implements output.Writer interface.
func (table *Table) WriteResource(node string, r resource.Resource, event state.EventType) error {
	values := []string{r.Metadata().Namespace(), r.Metadata().Type(), r.Metadata().ID(), r.Metadata().Version().String()}

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

	values = append([]string{node}, values...)

	_, err := fmt.Fprintln(&table.w, strings.Join(values, "\t"))

	return err
}

// Flush implements output.Writer interface.
func (table *Table) Flush() error {
	return table.w.Flush()
}
