// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/crypto/x509"

	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// ClientSpecNamespaceName is the namespace of the ClientSpec resource.
//
// The resource contains the private key of the etcd client certificate, so it lives with other secrets.
const ClientSpecNamespaceName = secrets.NamespaceName

// ClientSpecType is type of ClientSpec resource.
const ClientSpecType = resource.Type("EtcdClientSpecs.etcd.talos.dev")

// ClientSpecID is the ID of the singleton ClientSpec resource.
const ClientSpecID = resource.ID("etcd")

// ClientSpec describes how to connect to etcd as a client: the credentials, and the endpoints.
type ClientSpec = typed.Resource[ClientSpecSpec, ClientSpecExtension]

// ClientSpecSpec describes the etcd client configuration.
//
//gotagsrewrite:gen
type ClientSpecSpec struct {
	// AcceptedCAs is the list of CA certificates the etcd client trusts.
	AcceptedCAs []*x509.PEMEncodedCertificate `yaml:"acceptedCAs" protobuf:"1"`
	// Certificate is the etcd client certificate (with the key).
	Certificate *x509.PEMEncodedCertificateAndKey `yaml:"certificate" protobuf:"2"`
	// LocalEndpoints is the list of endpoints (host:port) of the etcd member running on this machine.
	LocalEndpoints []string `yaml:"localEndpoints" protobuf:"3"`
	// ClusterEndpoints is the list of endpoints (host:port) of the etcd members across the cluster.
	//
	// The endpoints are the discovered control plane machines, so it is not guaranteed that etcd is running on each of them.
	// The list is empty until the control plane machines are discovered.
	ClusterEndpoints []string `yaml:"clusterEndpoints" protobuf:"4"`
}

// NewClientSpec initializes a ClientSpec resource.
func NewClientSpec() *ClientSpec {
	return typed.NewResource[ClientSpecSpec, ClientSpecExtension](
		resource.NewMetadata(ClientSpecNamespaceName, ClientSpecType, ClientSpecID, resource.VersionUndefined),
		ClientSpecSpec{},
	)
}

// ClientSpecExtension provides auxiliary methods for ClientSpec.
type ClientSpecExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ClientSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ClientSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: ClientSpecNamespaceName,
		Sensitivity:      meta.Sensitive,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Local Endpoints",
				JSONPath: "{.localEndpoints}",
			},
			{
				Name:     "Cluster Endpoints",
				JSONPath: "{.clusterEndpoints}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ClientSpecSpec](ClientSpecType, &ClientSpec{})
	if err != nil {
		panic(err)
	}
}
