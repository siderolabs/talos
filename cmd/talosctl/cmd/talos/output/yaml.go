// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	yaml "go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// YAML outputs resources in YAML format.
type YAML struct {
	needDashes bool
	withEvents bool
	writer     io.Writer
}

// NewYAML initializes YAML resource output.
func NewYAML(writer io.Writer) *YAML {
	return &YAML{
		writer: writer,
	}
}

// WriteHeader implements output.Writer interface.
func (y *YAML) WriteHeader(definition *meta.ResourceDefinition, withEvents bool) error {
	y.withEvents = withEvents

	return nil
}

// WriteResource implements output.Writer interface.
func (y *YAML) WriteResource(node string, r resource.Resource, event state.EventType) error {
	if r.Metadata().Type() == config.MachineConfigType && r.Metadata().Annotations().Empty() {
		// use a temporary wrapper to adjust YAML marshaling
		// for backwards compatibility with versions of Talos
		// which incorrectly marshal MachineConfig spec as YAML document
		// directly
		r = &mcYamlRepr{r}
	}

	out, err := resource.MarshalYAML(r)
	if err != nil {
		return err
	}

	if y.needDashes {
		fmt.Fprintln(y.writer, "---")
	}

	y.needDashes = true

	fmt.Fprintf(y.writer, "node: %s\n", node)

	if y.withEvents {
		fmt.Fprintf(y.writer, "event: %s\n", strings.ToLower(event.String()))
	}

	return yaml.NewEncoder(y.writer).Encode(out)
}

// Flush implements output.Writer interface.
func (y *YAML) Flush() error {
	return nil
}

type mcYamlRepr struct{ resource.Resource }

func (m *mcYamlRepr) Spec() any { return &mcYamlSpec{res: m.Resource} }

type mcYamlSpec struct{ res resource.Resource }

func (m *mcYamlSpec) MarshalYAML() (any, error) {
	out, err := yaml.Marshal(m.res.Spec())
	if err != nil {
		return nil, err
	}

	return string(out), err
}
