// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package output

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"gopkg.in/yaml.v3"
)

// JSON outputs resources in JSON format.
type JSON struct {
	withEvents bool
}

// NewJSON initializes JSON resource output.
func NewJSON() *JSON {
	return &JSON{}
}

// WriteHeader implements output.Writer interface.
func (j *JSON) WriteHeader(definition resource.Resource, withEvents bool) error {
	j.withEvents = withEvents

	return nil
}

// WriteResource implements output.Writer interface.
func (j *JSON) WriteResource(node string, r resource.Resource, event state.EventType) error {
	out, err := resource.MarshalYAML(r)
	if err != nil {
		return err
	}

	yamlBytes, err := yaml.Marshal(out)
	if err != nil {
		return err
	}

	var data map[string]interface{}

	err = yaml.Unmarshal(yamlBytes, &data)
	if err != nil {
		return err
	}

	data["node"] = node

	if j.withEvents {
		data["event"] = strings.ToLower(event.String())
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")

	return enc.Encode(data)
}

// Flush implements output.Writer interface.
func (j *JSON) Flush() error {
	return nil
}
