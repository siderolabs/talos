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

// KubeletKubeconfigType is type of KubeletKubeconfig resource.
const KubeletKubeconfigType = resource.Type("KubeletKubeconfigs.kubernetes.talos.dev")

// KubeletKubeconfigID is a singleton resource ID for KubeletKubeconfig.
const KubeletKubeconfigID = resource.ID("kubelet")

// KubeletKubeconfig resource exposes the on-disk kubelet kubeconfig state so
// that consumers can detect when the file has changed and rebuild their
// Kubernetes clients (the informer's reflector doesn't bubble up
// connection-refused errors against a stale endpoint).
type KubeletKubeconfig = typed.Resource[KubeletKubeconfigSpec, KubeletKubeconfigExtension]

// KubeletKubeconfigSpec describes the current kubelet kubeconfig file.
//
//gotagsrewrite:gen
type KubeletKubeconfigSpec struct {
	// Hash is a content digest of the kubeconfig file. It changes whenever the
	// file contents change, which is the signal consumers use to rebuild their
	// Kubernetes clients.
	Hash string `yaml:"hash" protobuf:"1"`
}

// NewKubeletKubeconfig initializes a KubeletKubeconfig resource.
func NewKubeletKubeconfig(namespace resource.Namespace, id resource.ID) *KubeletKubeconfig {
	return typed.NewResource[KubeletKubeconfigSpec, KubeletKubeconfigExtension](
		resource.NewMetadata(namespace, KubeletKubeconfigType, id, resource.VersionUndefined),
		KubeletKubeconfigSpec{},
	)
}

// KubeletKubeconfigExtension provides auxiliary methods for KubeletKubeconfig.
type KubeletKubeconfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (KubeletKubeconfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubeletKubeconfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Hash",
				JSONPath: "{.hash}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[KubeletKubeconfigSpec](KubeletKubeconfigType, &KubeletKubeconfig{})
	if err != nil {
		panic(err)
	}
}
