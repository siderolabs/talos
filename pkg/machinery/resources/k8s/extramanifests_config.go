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
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/talos-systems/talos/pkg/machinery/proto"
)

// ExtraManifestsConfigType is type of ExtraManifestsConfig resource.
const ExtraManifestsConfigType = resource.Type("ExtraManifestsConfigs.kubernetes.talos.dev")

// ExtraManifestsConfigID is a singleton resource ID for ExtraManifestsConfig.
const ExtraManifestsConfigID = resource.ID("extra-manifests")

// ExtraManifestsConfig represents configuration for extra bootstrap manifests.
type ExtraManifestsConfig = typed.Resource[ExtraManifestsConfigSpec, ExtraManifestsConfigRD]

// ExtraManifestsConfigSpec is configuration for extra bootstrap manifests.
//
//gotagsrewrite:gen
type ExtraManifestsConfigSpec struct {
	ExtraManifests []ExtraManifest `yaml:"extraManifests" protobuf:"1"`
}

// ExtraManifest defines a single extra manifest to download.
//
//gotagsrewrite:gen
type ExtraManifest struct {
	Name           string            `yaml:"name" protobuf:"1"`
	URL            string            `yaml:"url" protobuf:"2"`
	Priority       string            `yaml:"priority" protobuf:"3"`
	ExtraHeaders   map[string]string `yaml:"extraHeaders" protobuf:"4"`
	InlineManifest string            `yaml:"inlineManifest" protobuf:"5"`
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

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ExtraManifestsConfigSpec](ExtraManifestsConfigType, &ExtraManifestsConfig{})
	if err != nil {
		panic(err)
	}
}
