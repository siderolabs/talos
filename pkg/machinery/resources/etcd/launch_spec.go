// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// LaunchSpecType is type of LaunchSpec resource.
const LaunchSpecType = resource.Type("EtcdLaunchSpecs.etcd.talos.dev")

// LaunchSpecID is the ID of the singleton LaunchSpec resource.
const LaunchSpecID = resource.ID("etcd")

// LaunchSpec describes how the etcd member on this machine should be started.
//
// The resource is produced by the etcd membership controller once it is decided how the etcd data directory
// is (or was) populated: a new cluster was bootstrapped, the machine joined an existing cluster,
// or the etcd data is already present.
//
// The absence of the resource means that etcd should not be running.
type LaunchSpec = typed.Resource[LaunchSpecSpec, LaunchSpecExtension]

// LaunchSpecSpec describes the way etcd is started.
//
//gotagsrewrite:gen
type LaunchSpecSpec struct {
	// InitialClusterState is the value of the etcd `--initial-cluster-state` flag.
	InitialClusterState string `yaml:"initialClusterState" protobuf:"1"`
	// InitialCluster is the value of the etcd `--initial-cluster` flag.
	//
	// It is empty if etcd already has the data (so the flag is not used),
	// or if the initial cluster is supplied in the extra arguments.
	InitialCluster string `yaml:"initialCluster,omitempty" protobuf:"2"`
	// InitialAdvertisePeerURLs is the value of the etcd `--initial-advertise-peer-urls` flag.
	//
	// It is set along with the InitialCluster, as the two should match: these are the peer URLs
	// of this member at the moment it was decided how to launch etcd.
	// If it is empty, the current advertised addresses are used.
	InitialAdvertisePeerURLs []string `yaml:"initialAdvertisePeerURLs,omitempty" protobuf:"3"`
}

// NewLaunchSpec initializes a LaunchSpec resource.
func NewLaunchSpec() *LaunchSpec {
	return typed.NewResource[LaunchSpecSpec, LaunchSpecExtension](
		resource.NewMetadata(NamespaceName, LaunchSpecType, LaunchSpecID, resource.VersionUndefined),
		LaunchSpecSpec{},
	)
}

// LaunchSpecExtension provides auxiliary methods for LaunchSpec.
type LaunchSpecExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (LaunchSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LaunchSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Initial Cluster State",
				JSONPath: "{.initialClusterState}",
			},
			{
				Name:     "Initial Cluster",
				JSONPath: "{.initialCluster}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[LaunchSpecSpec](LaunchSpecType, &LaunchSpec{})
	if err != nil {
		panic(err)
	}
}
