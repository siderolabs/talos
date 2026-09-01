// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mountconfig

import (
	"reflect"
	"slices"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// Snapshot records the fsopen parameters used to create a mount.
type Snapshot struct {
	Parameters []block.ParameterSpec
}

// NewSnapshot records the parameters used to create a mount.
func NewSnapshot(parameters []block.ParameterSpec) Snapshot {
	result := make([]block.ParameterSpec, len(parameters))

	for i := range parameters {
		result[i] = parameters[i]
		result[i].Binary = slices.Clone(parameters[i].Binary)

		if parameters[i].String != nil {
			value := *parameters[i].String
			result[i].String = &value
		}
	}

	return Snapshot{Parameters: result}
}

// ParametersChanged reports whether recreating the filesystem mount is required.
func (snapshot Snapshot) ParametersChanged(parameters []block.ParameterSpec) bool {
	return !reflect.DeepEqual(snapshot.Parameters, parameters)
}
