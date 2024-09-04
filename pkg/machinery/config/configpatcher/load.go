// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configpatcher

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
)

type patch []map[string]any

// LoadPatch loads the strategic merge patch or JSON patch (JSON/YAML for JSON patch).
func LoadPatch(in []byte) (Patch, error) {
	// Try configloader first, as it is more strict about the config format
	cfg, strategicErr := configloader.NewFromBytes(in, configloader.WithAllowPatchDelete())
	if strategicErr == nil {
		return NewStrategicMergePatch(cfg), nil
	}

	var (
		jsonErr error
		p       jsonpatch.Patch
	)

	// try JSON first
	if p, jsonErr = jsonpatch.DecodePatch(in); jsonErr == nil {
		return p, nil
	}

	// try YAML
	var yamlPatch patch

	if err := yaml.Unmarshal(in, &yamlPatch); err != nil {
		// not YAML either, return previous error
		// see if input looks like JSON Patch as JSON
		if bytes.HasPrefix(bytes.TrimSpace(in), []byte("[")) {
			return nil, jsonErr
		}

		// nope, return config loading error (assume it was strategic merge patch)
		return nil, strategicErr
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
func LoadPatches(in []string) ([]Patch, error) {
	var result []Patch

	for _, patchString := range in {
		var (
			p        Patch
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

		// merge JSON patches if they come one after another
		_, isJSONPatch := p.(jsonpatch.Patch)
		lastJSONPatch := false

		if len(result) > 0 {
			if _, ok := result[len(result)-1].(jsonpatch.Patch); ok {
				lastJSONPatch = true
			}
		}

		if isJSONPatch && lastJSONPatch {
			result[len(result)-1] = append(result[len(result)-1].(jsonpatch.Patch), p.(jsonpatch.Patch)...)
		} else {
			result = append(result, p)
		}
	}

	return result, nil
}
