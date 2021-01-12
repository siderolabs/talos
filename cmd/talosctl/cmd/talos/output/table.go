// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package output

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/talos-systems/os-runtime/pkg/resource"
)

// Table outputs resources in Table view.
type Table struct {
	w tabwriter.Writer
}

// NewTable initializes table resource output.
func NewTable() *Table {
	output := &Table{}
	output.w.Init(os.Stdout, 0, 0, 3, ' ', 0)

	return output
}

// WriteHeader implements output.Writer interface.
func (table *Table) WriteHeader(definition resource.Resource) error {
	_, err := fmt.Fprintln(&table.w, "NODE\tNAMESPACE\tTYPE\tID")

	return err
}

// WriteResource implements output.Writer interface.
func (table *Table) WriteResource(node string, r resource.Resource) error {
	_, err := fmt.Fprintf(&table.w, "%s\t%s\t%s\t%s\n", node, r.Metadata().Namespace(), r.Metadata().Type(), r.Metadata().ID())

	return err
}

// Flush implements output.Writer interface.
func (table *Table) Flush() error {
	return table.w.Flush()
}
