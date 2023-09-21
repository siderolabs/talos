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

	"github.com/siderolabs/gen/xslices"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
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
//
//nolint:gocyclo
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

		// if the manifest is a list, we will unwrap it
		if obj.IsList() {
			if err = obj.EachListItem(func(item runtime.Object) error {
				obj, ok := item.(*unstructured.Unstructured)
				if !ok {
					return fmt.Errorf("list item is not Unstructured: %T", item)
				}

				a.Manifest.TypedSpec().Items = append(a.Manifest.TypedSpec().Items, k8s.SingleManifest{Object: obj.Object})

				return nil
			}); err != nil {
				return fmt.Errorf("error unwrapping a List: %w", err)
			}
		} else {
			a.Manifest.TypedSpec().Items = append(a.Manifest.TypedSpec().Items, k8s.SingleManifest{Object: obj.Object})
		}
	}

	return nil
}

// Objects returns list of unstructured object.
func (a manifest) Objects() []*unstructured.Unstructured {
	return xslices.Map(a.Manifest.TypedSpec().Items, func(item k8s.SingleManifest) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: item.Object}
	})
}
