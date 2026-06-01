// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// EtcdEncryptionConfigType is type of EtcdEncryptionConfig secret resource.
const EtcdEncryptionConfigType = resource.Type("EtcdEncryptionConfigs.kubernetes.talos.dev")

// EtcdEncryptionConfigID is the ID of KubernetesEtcdEncryptionType resource.
const EtcdEncryptionConfigID = resource.ID("k8s")

// EtcdEncryptionConfig contains root (not generated) secrets.
type EtcdEncryptionConfig = typed.Resource[EtcdEncryptionConfigSpec, EtcdEncryptionConfigExtension]

// EtcdEncryptionConfigSpec describes root Kubernetes secrets.
//
//gotagsrewrite:gen
type EtcdEncryptionConfigSpec struct {
	Configuration string `yaml:"configuration" protobuf:"1"`
}

// NewEtcdEncryptionConfig initializes an EtcdEncryptionConfig resource.
func NewEtcdEncryptionConfig(id resource.ID) *EtcdEncryptionConfig {
	return typed.NewResource[EtcdEncryptionConfigSpec, EtcdEncryptionConfigExtension](
		resource.NewMetadata(NamespaceName, EtcdEncryptionConfigType, id, resource.VersionUndefined),
		EtcdEncryptionConfigSpec{},
	)
}

// EtcdEncryptionConfigExtension provides auxiliary methods for EtcdEncryptionConfig.
type EtcdEncryptionConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (EtcdEncryptionConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcdEncryptionConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[EtcdEncryptionConfigSpec](EtcdEncryptionConfigType, &EtcdEncryptionConfig{})
	if err != nil {
		panic(err)
	}
}
