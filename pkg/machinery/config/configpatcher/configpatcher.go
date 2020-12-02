// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package configpatcher provides methods to patch Talos config.
package configpatcher

import (
	"fmt"

	jsonpatch "github.com/evanphx/json-patch"
	ghodssyaml "github.com/ghodss/yaml"
)

// JSON6902 is responsible for applying a JSON 6902 patch to the bootstrap data.
func JSON6902(talosMachineConfig []byte, patch jsonpatch.Patch) ([]byte, error) {
	jsonDecodedData, err := ghodssyaml.YAMLToJSON(talosMachineConfig)
	if err != nil {
		return nil, fmt.Errorf("failure converting talos machine config to json: %s", err)
	}

	jsonDecodedData, err = patch.Apply(jsonDecodedData)
	if err != nil {
		return nil, fmt.Errorf("failure applying rfc6902 patches to talos machine config: %s", err)
	}

	talosMachineConfig, err = ghodssyaml.JSONToYAML(jsonDecodedData)
	if err != nil {
		return nil, fmt.Errorf("failure converting talos machine config from json to yaml: %s", err)
	}

	return talosMachineConfig, nil
}
