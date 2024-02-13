// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configpatcher

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	jsonpatch "github.com/evanphx/json-patch"
	ghodssyaml "github.com/ghodss/yaml"
	"gopkg.in/yaml.v3"
)

// JSON6902 is responsible for applying a JSON 6902 patch to the bootstrap data.
func JSON6902(talosMachineConfig []byte, patch jsonpatch.Patch) ([]byte, error) {
	// check number of input documents
	numDocuments, err := countYAMLDocuments(talosMachineConfig)
	if err != nil {
		return nil, err
	}

	if numDocuments != 1 {
		return nil, errors.New("JSON6902 patches are not supported for multi-document machine configuration")
	}

	// apply JSON patch
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

func countYAMLDocuments(talosMachineConfig []byte) (int, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(talosMachineConfig))

	numDocuments := 0

	for {
		var docs yaml.Node

		err := decoder.Decode(&docs)
		if err == io.EOF {
			break
		}

		if err != nil {
			return 0, fmt.Errorf("failure decoding talos machine config: %s", err)
		}

		if docs.Kind != yaml.DocumentNode {
			return 0, errors.New("talos machine config is not a yaml document")
		}

		numDocuments++
	}

	return numDocuments, nil
}
