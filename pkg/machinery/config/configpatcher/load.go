// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configpatcher

import (
	"encoding/json"
	"os"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"gopkg.in/yaml.v3"
)

type patch []map[string]interface{}

// LoadPatch loads the JSON patch either from JSON or YAML.
func LoadPatch(in []byte) (p jsonpatch.Patch, err error) {
	var jsonErr error

	// try JSON first
	if p, jsonErr = jsonpatch.DecodePatch(in); jsonErr == nil {
		return p, nil
	}

	// try YAML
	var yamlPatch patch

	if err = yaml.Unmarshal(in, &yamlPatch); err != nil {
		// not YAML either, return JSON error
		return p, jsonErr
	}

	p = make(jsonpatch.Patch, 0, len(yamlPatch))

	for _, yp := range yamlPatch {
		op := make(jsonpatch.Operation, len(yp))

		for key, value := range yp {
			m, err := json.Marshal(value)
			if err != nil {
				return p, err
			}

			op[key] = (*json.RawMessage)(&m)
		}

		p = append(p, op)
	}

	return p, nil
}

// LoadPatches loads the JSON patch either from value literal or from a file if the patch starts with '@'.
func LoadPatches(in []string) (jsonpatch.Patch, error) {
	var result jsonpatch.Patch

	for _, patchString := range in {
		var (
			p        jsonpatch.Patch
			contents []byte
			err      error
		)

		if strings.HasPrefix(patchString, "@") {
			filename := patchString[1:]

			contents, err = os.ReadFile(filename)
			if err != nil {
				return result, err
			}
		} else {
			contents = []byte(patchString)
		}

		p, err = LoadPatch(contents)
		if err != nil {
			return result, err
		}

		result = append(result, p...)
	}

	return result, nil
}
