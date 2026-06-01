// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// KubernetesType is type of Kubernetes resource.
const KubernetesType = resource.Type("KubernetesSecrets.secrets.talos.dev")

// KubernetesID is a resource ID of singleton instance.
const KubernetesID = resource.ID("k8s-certs")

// Kubernetes contains K8s generated secrets.
//
// Kubernetes resource contains secrets which require reload of the control plane pods if updated.
type Kubernetes = typed.Resource[KubernetesCertsSpec, KubernetesExtension]

// KubernetesCertsSpec describes generated Kubernetes certificates.
//
//gotagsrewrite:gen
type KubernetesCertsSpec struct {
	SchedulerKubeconfig         string `yaml:"schedulerKubeconfig" protobuf:"4"`
	ControllerManagerKubeconfig string `yaml:"controllerManagerKubeconfig" protobuf:"5"`

	// Admin-level kubeconfig with access through the localhost endpoint and cluster endpoints.
	LocalhostAdminKubeconfig string `yaml:"localhostAdminKubeconfig" protobuf:"6"`
	AdminKubeconfig          string `yaml:"adminKubeconfig" protobuf:"7"`
}

// NewKubernetes initializes a Kubernetes resource.
func NewKubernetes() *Kubernetes {
	return typed.NewResource[KubernetesCertsSpec, KubernetesExtension](
		resource.NewMetadata(NamespaceName, KubernetesType, KubernetesID, resource.VersionUndefined),
		KubernetesCertsSpec{},
	)
}

// KubernetesExtension provides auxiliary methods for Kubernetes.
type KubernetesExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (KubernetesExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[KubernetesCertsSpec](KubernetesType, &Kubernetes{})
	if err != nil {
		panic(err)
	}
}
