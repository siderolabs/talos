// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8s provides resources which interface with Kubernetes.
//
//nolint:dupl
package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// ExtraManifestsConfigType is type of ExtraManifestsConfig resource.
const ExtraManifestsConfigType = resource.Type("ExtraManifestsConfigs.kubernetes.talos.dev")

// ExtraManifestsConfigID is a singleton resource ID for ExtraManifestsConfig.
const ExtraManifestsConfigID = resource.ID("extra-manifests")

// ExtraManifestsConfig represents configuration for extra bootstrap manifests.
type ExtraManifestsConfig = typed.Resource[ExtraManifestsConfigSpec, ExtraManifestsConfigRD]

// ExtraManifestsConfigSpec is configuration for extra bootstrap manifests.
type ExtraManifestsConfigSpec struct {
	ExtraManifests []ExtraManifest `yaml:"extraManifests"`
}

// ExtraManifest defines a single extra manifest to download.
type ExtraManifest struct {
	Name           string            `yaml:"name"`
	URL            string            `yaml:"url"`
	Priority       string            `yaml:"priority"`
	ExtraHeaders   map[string]string `yaml:"extraHeaders"`
	InlineManifest string            `yaml:"inlineManifest"`
}

// DeepCopy implements Deepcopyable.
//
// TODO: should be properly go-generated.
func (spec ExtraManifestsConfigSpec) DeepCopy() ExtraManifestsConfigSpec {
	return spec
}

// NewExtraManifestsConfig returns new ExtraManifestsConfig resource.
func NewExtraManifestsConfig() *ExtraManifestsConfig {
	return typed.NewResource[ExtraManifestsConfigSpec, ExtraManifestsConfigRD](
		resource.NewMetadata(ControlPlaneNamespaceName, ExtraManifestsConfigType, ExtraManifestsConfigID, resource.VersionUndefined),
		ExtraManifestsConfigSpec{})
}

// ExtraManifestsConfigRD defines ExtraManifestsConfig resource definition.
type ExtraManifestsConfigRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ExtraManifestsConfigRD) ResourceDefinition(_ resource.Metadata, _ ExtraManifestsConfigSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ExtraManifestsConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}
