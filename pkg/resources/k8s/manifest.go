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

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// ManifestType is type of Manifest resource.
const ManifestType = resource.Type("Manifests.kubernetes.talos.dev")

// Manifest resource holds definition of kubelet static pod.
type Manifest struct {
	md   resource.Metadata
	spec *manifestSpec
}

type manifestSpec struct {
	Items []*unstructured.Unstructured
}

func (spec *manifestSpec) MarshalYAML() (interface{}, error) {
	result := make([]map[string]interface{}, 0, len(spec.Items))

	for _, obj := range spec.Items {
		result = append(result, obj.Object)
	}

	return result, nil
}

// NewManifest initializes an empty Manifest resource.
func NewManifest(namespace resource.Namespace, id resource.ID) *Manifest {
	r := &Manifest{
		md:   resource.NewMetadata(namespace, ManifestType, id, resource.VersionUndefined),
		spec: &manifestSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Manifest) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Manifest) Spec() interface{} {
	return r.spec
}

func (r *Manifest) String() string {
	return fmt.Sprintf("k8s.Manifest(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *Manifest) DeepCopy() resource.Resource {
	spec := &manifestSpec{
		Items: make([]*unstructured.Unstructured, len(r.spec.Items)),
	}

	for i := range r.spec.Items {
		spec.Items[i] = r.spec.Items[i].DeepCopy()
	}

	return &Manifest{
		md:   r.md,
		spec: spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Manifest) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ManifestType,
		Aliases:          []resource.Type{},
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}

// SetYAML parses manifest from YAML.
func (r *Manifest) SetYAML(yamlBytes []byte) error {
	r.spec.Items = nil
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

		obj := new(unstructured.Unstructured)

		if err = json.Unmarshal(jsonManifest, obj); err != nil {
			return fmt.Errorf("error loading JSON manifest into unstructured: %w", err)
		}

		r.spec.Items = append(r.spec.Items, obj)
	}

	return nil
}

// Objects returns list of unstrustured object.
func (r *Manifest) Objects() []*unstructured.Unstructured {
	return r.spec.Items
}
