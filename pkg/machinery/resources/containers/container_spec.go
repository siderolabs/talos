// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ContainerSpecType is type of ContainerSpec resource.
const ContainerSpecType = resource.Type("ContainerSpecs.containers.talos.dev")

// ContainerSpec resource holds the desired state of a single container.
//
// It is the long-lived unit: it exists for as long as its ContainerConfig document does, and
// outlives any number of executions. A single execution is a ContainerInstanceSpec.
type ContainerSpec = typed.Resource[ContainerSpecSpec, ContainerSpecExtension]

// ContainerSpecSpec is the spec for ContainerSpec.
//
//gotagsrewrite:gen
type ContainerSpecSpec struct {
	// Image is the OCI reference in canonical form, already normalized by ContainerConfigController.
	Image string `yaml:"image" protobuf:"1"`

	Entrypoint  []string           `yaml:"entrypoint,omitempty" protobuf:"2"`
	Args        []string           `yaml:"args,omitempty" protobuf:"3"`
	WorkingDir  string             `yaml:"workingDir,omitempty" protobuf:"4"`
	RunAs       ContainerRunAsSpec `yaml:"runAs,omitempty" protobuf:"5"`
	Environment []string           `yaml:"environment,omitempty" protobuf:"6"`

	Mounts []ContainerMountSpec `yaml:"mounts,omitempty" protobuf:"7"`

	Security  ContainerSecuritySpec  `yaml:"security,omitempty" protobuf:"8"`
	Network   ContainerNetworkSpec   `yaml:"network,omitempty" protobuf:"9"`
	Resources ContainerResourcesSpec `yaml:"resources,omitempty" protobuf:"10"`
	DependsOn ContainerDependsOnSpec `yaml:"dependsOn,omitempty" protobuf:"11"`
}

// ContainerMountSpec is a resolved mount.
//
// Exactly one of VolumeID, Tmpfs or HostPath describes the source; Kind says which.
//
//gotagsrewrite:gen
type ContainerMountSpec struct {
	// Kind is one of "userVolume", "tmpfs" or "hostPath".
	Kind string `yaml:"kind" protobuf:"1"`
	// VolumeID is the block volume ID for a userVolume mount, e.g. "u-web-content".
	VolumeID string `yaml:"volumeID,omitempty" protobuf:"2"`
	// Source is the host path for a hostPath mount.
	Source string `yaml:"source,omitempty" protobuf:"3"`
	// Destination inside the container.
	Destination string `yaml:"destination" protobuf:"4"`
	// Size of a tmpfs mount, in bytes; zero means the kernel default.
	Size uint64 `yaml:"size,omitempty" protobuf:"5"`
	// Options with the read-only default already applied.
	Options []string `yaml:"options,omitempty" protobuf:"6"`
}

// Mount kinds.
const (
	MountKindUserVolume = "userVolume"
	MountKindTmpfs      = "tmpfs"
	MountKindHostPath   = "hostPath"
)

// ContainerSecuritySpec is the resolved security posture.
//
//gotagsrewrite:gen
type ContainerSecuritySpec struct {
	// Privileged grants all grantable capabilities and all devices, matching what extension
	// services get implicitly.
	Privileged bool `yaml:"privileged,omitempty" protobuf:"1"`

	CapabilitiesAdd  []string `yaml:"capabilitiesAdd,omitempty" protobuf:"2"`
	CapabilitiesDrop []string `yaml:"capabilitiesDrop,omitempty" protobuf:"3"`
}

// ContainerNetworkSpec is the resolved network configuration.
//
//gotagsrewrite:gen
type ContainerNetworkSpec struct {
	// HostNetwork shares the host network namespace instead of creating an empty one.
	HostNetwork bool `yaml:"hostNetwork,omitempty" protobuf:"1"`
}

// ContainerResourcesSpec is the resolved cgroup configuration, in bytes and millicores.
//
// Zero means unset, which for a limit means unlimited.
//
//gotagsrewrite:gen
type ContainerResourcesSpec struct {
	MemoryLimit uint64 `yaml:"memoryLimit,omitempty" protobuf:"1"`
	CPULimit    uint64 `yaml:"cpuLimit,omitempty" protobuf:"2"`
}

// ContainerDependsOnSpec is the resolved dependency set.
//
//gotagsrewrite:gen
type ContainerDependsOnSpec struct {
	Paths      []string `yaml:"paths,omitempty" protobuf:"1"`
	Networks   []string `yaml:"networks,omitempty" protobuf:"2"`
	Time       bool     `yaml:"time,omitempty" protobuf:"3"`
	Containers []string `yaml:"containers,omitempty" protobuf:"4"`
}

// ContainerRunAsSpec is the resolved uid/gid override.
//
// Nil means use the image's own USER for that half.
//
//gotagsrewrite:gen
type ContainerRunAsSpec struct {
	UID *int32 `yaml:"uid,omitempty" protobuf:"1"`
	GID *int32 `yaml:"gid,omitempty" protobuf:"2"`
}

// NewContainerSpec initializes a ContainerSpec resource.
func NewContainerSpec(namespace resource.Namespace, id resource.ID) *ContainerSpec {
	return typed.NewResource[ContainerSpecSpec, ContainerSpecExtension](
		resource.NewMetadata(namespace, ContainerSpecType, id, resource.VersionUndefined),
		ContainerSpecSpec{},
	)
}

// ContainerSpecExtension is auxiliary resource data for ContainerSpec.
type ContainerSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ContainerSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ContainerSpecType,
		Aliases:          []resource.Type{"containerspec", "containerspecs"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Image",
				JSONPath: `{.image}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(ContainerSpecType, &ContainerSpec{})
	if err != nil {
		panic(err)
	}
}
