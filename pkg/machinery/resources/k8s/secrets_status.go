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

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// SecretsStatusType is type of SecretsStatus resource.
const SecretsStatusType = resource.Type("SecretStatuses.kubernetes.talos.dev")

// StaticPodSecretsStaticPodID is resource ID for SecretStatus resource for static pods.
const StaticPodSecretsStaticPodID = resource.ID("static-pods")

// SecretsStatus resource holds definition of rendered secrets.
type SecretsStatus = typed.Resource[SecretsStatusSpec, SecretsStatusRD]

// SecretsStatusSpec describes status of rendered secrets.
//
//gotagsrewrite:gen
type SecretsStatusSpec struct {
	Ready   bool   `yaml:"ready" protobuf:"1"`
	Version string `yaml:"version" protobuf:"2"`
}

// NewSecretsStatus initializes a SecretsStatus resource.
func NewSecretsStatus(namespace resource.Namespace, id resource.ID) *SecretsStatus {
	return typed.NewResource[SecretsStatusSpec, SecretsStatusRD](
		resource.NewMetadata(namespace, SecretsStatusType, id, resource.VersionUndefined),
		SecretsStatusSpec{},
	)
}

// SecretsStatusRD provides auxiliary methods for SecretsStatus.
type SecretsStatusRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (SecretsStatusRD) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SecretsStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: ControlPlaneNamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Ready",
				JSONPath: "{.ready}",
			},
			{
				Name:     "Secrets Version",
				JSONPath: "{.version}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[SecretsStatusSpec](SecretsStatusType, &SecretsStatus{})
	if err != nil {
		panic(err)
	}
}
