// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8s provides resources which interface with Kubernetes.
package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// AuditPolicyConfigType is type of AuditPolicyConfig resource.
const AuditPolicyConfigType = resource.Type("AuditPolicyConfigs.kubernetes.talos.dev")

// AuditPolicyConfigID is a singleton resource ID for AuditPolicyConfig.
const AuditPolicyConfigID = resource.ID("audit-policy")

// AuditPolicyConfig represents configuration for kube-apiserver audit policy.
type AuditPolicyConfig = typed.Resource[AuditPolicyConfigSpec, AuditPolicyConfigExtension]

// AuditPolicyConfigSpec is audit policy configuration for kube-apiserver.
//
//gotagsrewrite:gen
type AuditPolicyConfigSpec struct {
	Config map[string]any `yaml:"config" protobuf:"1"`
}

// NewAuditPolicyConfig returns new AuditPolicyConfig resource.
func NewAuditPolicyConfig() *AuditPolicyConfig {
	return typed.NewResource[AuditPolicyConfigSpec, AuditPolicyConfigExtension](
		resource.NewMetadata(ControlPlaneNamespaceName, AuditPolicyConfigType, AuditPolicyConfigID, resource.VersionUndefined),
		AuditPolicyConfigSpec{})
}

// AuditPolicyConfigExtension defines AuditPolicyConfig resource definition.
type AuditPolicyConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (AuditPolicyConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AuditPolicyConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[AuditPolicyConfigSpec](AuditPolicyConfigType, &AuditPolicyConfig{})
	if err != nil {
		panic(err)
	}
}
