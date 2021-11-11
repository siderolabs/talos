// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/talos-systems/talos/pkg/resources/k8s"
)

// Manifest adapter provides conversion from procfs.
//
//nolint:revive,golint
func Manifest(r *k8s.Manifest) manifest {
	return manifest{
		Manifest: r,
	}
}

type manifest struct {
	*k8s.Manifest
}

// SetYAML parses manifest from YAML.
func (a manifest) SetYAML(yamlBytes []byte) error {
	a.Manifest.TypedSpec().Items = nil
	reader := yaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(yamlBytes)))

	for {
		yamlManifest, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}

		yamlManifest = bytes.TrimSpace(yamlManifest)

		if len(yamlManifest) == 0 {
			continue
		}

		jsonManifest, err := yaml.ToJSON(yamlManifest)
		if err != nil {
			return fmt.Errorf("error converting manifest to JSON: %w", err)
		}

		if bytes.Equal(jsonManifest, []byte("null")) || bytes.Equal(jsonManifest, []byte("{}")) {
			// skip YAML docs which contain only comments
			continue
		}

		var obj unstructured.Unstructured

		if err = json.Unmarshal(jsonManifest, &obj); err != nil {
			return fmt.Errorf("error loading JSON manifest into unstructured: %w", err)
		}

		a.Manifest.TypedSpec().Items = append(a.Manifest.TypedSpec().Items, obj.Object)
	}

	return nil
}

// Objects returns list of unstructured object.
func (a manifest) Objects() []*unstructured.Unstructured {
	result := make([]*unstructured.Unstructured, len(a.Manifest.TypedSpec().Items))

	for i := range result {
		result[i] = &unstructured.Unstructured{
			Object: a.Manifest.TypedSpec().Items[i],
		}
	}

	return result
}
