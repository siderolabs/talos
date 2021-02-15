// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package output

import (
	"fmt"
	"os"

	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"
	"gopkg.in/yaml.v3"
)

// YAML outputs resources in YAML format.
type YAML struct {
	needDashes bool
}

// NewYAML initializes YAML resource output.
func NewYAML() *YAML {
	return &YAML{}
}

// WriteHeader implements output.Writer interface.
func (y *YAML) WriteHeader(definition resource.Resource, withEvents bool) error {
	return nil
}

// WriteResource implements output.Writer interface.
func (y *YAML) WriteResource(node string, r resource.Resource, event state.EventType) error {
	out, err := resource.MarshalYAML(r)
	if err != nil {
		return err
	}

	if y.needDashes {
		fmt.Fprintln(os.Stdout, "---")
	}

	y.needDashes = true

	fmt.Fprintf(os.Stdout, "node: %s\n", node)

	return yaml.NewEncoder(os.Stdout).Encode(out)
}

// Flush implements output.Writer interface.
func (y *YAML) Flush() error {
	return nil
}
