// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// InstanceID builds the ID of a ContainerInstanceSpec from a container name and generation.
//
// Generations are numbered rather than reusing the container name so that creating the next
// instance never has to wait for the previous one to be destroyed.
func InstanceID(container string, generation uint64) resource.ID {
	return fmt.Sprintf("%s-%d", container, generation)
}

// ContainerInstanceSpecType is type of ContainerInstanceSpec resource.
const ContainerInstanceSpecType = resource.Type("ContainerInstanceSpecs.containers.talos.dev")

// ContainerInstanceSpec resource represents a single execution attempt of a container.
//
// Its existence is the instruction to run; its destruction is the instruction to stop. Restart is
// therefore a resource event rather than a loop inside a goroutine: the previous instance
// terminates, and the next generation replaces it.
//
// The ID is <container>-<generation>; see InstanceID.
type ContainerInstanceSpec = typed.Resource[ContainerInstanceSpecSpec, ContainerInstanceSpecExtension]

// ContainerInstanceSpecSpec is the spec for ContainerInstanceSpec.
//
// It carries a resolved snapshot of everything needed to run one execution, so whatever runs it
// never has to re-read the container spec or image status. That keeps the execution independent of
// later changes to those inputs: a spec change destroys this instance rather than mutating it.
//
//gotagsrewrite:gen
type ContainerInstanceSpecSpec struct {
	// ContainerID is the name of the owning container, i.e. the ContainerSpec ID.
	ContainerID string `yaml:"containerID" protobuf:"1"`
	// Generation is this instance's sequence number for that container.
	Generation uint64 `yaml:"generation" protobuf:"2"`

	// Image is the digest-resolved reference to run.
	Image string `yaml:"image" protobuf:"3"`

	Entrypoint  []string           `yaml:"entrypoint,omitempty" protobuf:"4"`
	Args        []string           `yaml:"args,omitempty" protobuf:"5"`
	WorkingDir  string             `yaml:"workingDir,omitempty" protobuf:"6"`
	RunAs       ContainerRunAsSpec `yaml:"runAs,omitempty" protobuf:"7"`
	Environment []string           `yaml:"environment,omitempty" protobuf:"8"`

	// Mounts are fully resolved, with host source paths filled in.
	Mounts []ResolvedMountSpec `yaml:"mounts,omitempty" protobuf:"9"`

	Security  ContainerSecuritySpec  `yaml:"security,omitempty" protobuf:"10"`
	Network   ContainerNetworkSpec   `yaml:"network,omitempty" protobuf:"11"`
	Resources ContainerResourcesSpec `yaml:"resources,omitempty" protobuf:"12"`
}

// ResolvedMountSpec is a mount with its host-side source resolved.
//
//gotagsrewrite:gen
type ResolvedMountSpec struct {
	Kind string `yaml:"kind" protobuf:"1"`
	// Source is the host path to bind from; empty for tmpfs.
	Source      string   `yaml:"source,omitempty" protobuf:"2"`
	Destination string   `yaml:"destination" protobuf:"3"`
	Size        uint64   `yaml:"size,omitempty" protobuf:"4"`
	Options     []string `yaml:"options,omitempty" protobuf:"5"`
}

// NewContainerInstanceSpec initializes a ContainerInstanceSpec resource.
func NewContainerInstanceSpec(namespace resource.Namespace, id resource.ID) *ContainerInstanceSpec {
	return typed.NewResource[ContainerInstanceSpecSpec, ContainerInstanceSpecExtension](
		resource.NewMetadata(namespace, ContainerInstanceSpecType, id, resource.VersionUndefined),
		ContainerInstanceSpecSpec{},
	)
}

// ContainerInstanceSpecExtension is auxiliary resource data for ContainerInstanceSpec.
type ContainerInstanceSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ContainerInstanceSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ContainerInstanceSpecType,
		Aliases:          []resource.Type{"containerinstancespec", "containerinstancespecs"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Container",
				JSONPath: `{.containerID}`,
			},
			{
				Name:     "Generation",
				JSONPath: `{.generation}`,
			},
			{
				Name:     "Image",
				JSONPath: `{.image}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(ContainerInstanceSpecType, &ContainerInstanceSpec{}); err != nil {
		panic(err)
	}
}

// InSyncWithContainerSpec reports whether the instance still matches the given container spec.
//
// The instance carries a resolved snapshot precisely so this comparison is possible: a running
// container is never mutated in place, it is replaced.
func (instanceSpec ContainerInstanceSpecSpec) InSyncWithContainerSpec(ctx context.Context, r controller.Reader, s *ContainerSpecSpec) (bool, error) {
	imageDigest, err := GetImageDigest(ctx, r, instanceSpec.ContainerID, s.Image.Ref)
	if err != nil {
		return false, err
	}

	if imageDigest != "" && imageDigest != instanceSpec.Image {
		return false, nil
	}

	resolvedMounts, mountsReady := ResolveInstanceMounts(s.Mounts)

	if !mountsReady {
		return false, nil
	}

	inSync := ProcessEqual(s, &instanceSpec) &&
		ResolvedMountsEqual(resolvedMounts, instanceSpec.Mounts) &&
		SecurityEqual(s.Security, instanceSpec.Security) &&
		s.Network == instanceSpec.Network &&
		s.Resources == instanceSpec.Resources

	return inSync, nil
}

// GetImageDigest returns the digest an instance of this container should run, or an empty string if
// imageRef has not been resolved to one.
//
// The status is only believed when it describes imageRef itself: an edited reference leaves the
// previous reference's status in place until ImageController re-pulls, and running those bytes under
// the new configuration would be running an image nobody asked for.
func GetImageDigest(ctx context.Context, r controller.Reader, containerID, imageRef string) (string, error) {
	imageStatus, err := safe.ReaderGetByID[*ContainerImageStatus](ctx, r, containerID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return "", nil
		}

		return "", fmt.Errorf("failed to get image status %q: %w", containerID, err)
	}

	if imageStatus.TypedSpec().Phase != ContainerImagePhaseReady || imageStatus.TypedSpec().Image != imageRef {
		return "", nil
	}

	return imageStatus.TypedSpec().Digest, nil
}

// ResolveInstanceMounts resolves a container's mounts from the ContainerSpec.
func ResolveInstanceMounts(mounts []ContainerMountSpec) (resolved []ResolvedMountSpec, ready bool) {
	ready = true

	for _, mount := range mounts {
		switch mount.Kind {
		case MountKindUserVolume:
			// A userVolume mount needs MountController to learn its real host path, which doesn't
			// exist yet, so any containers mounting userVolumes will deadlock as not-ready. This
			// will start working once we complete the mount implementation.
			ready = false
		case MountKindTmpfs:
			resolved = append(resolved, ResolvedMountSpec{
				Kind:        mount.Kind,
				Destination: mount.Destination,
				Size:        mount.Size,
				Options:     mount.Options,
			})
		case MountKindHostPath:
			resolved = append(resolved, ResolvedMountSpec{
				Kind:        mount.Kind,
				Source:      mount.Source,
				Destination: mount.Destination,
				Options:     mount.Options,
			})
		}
	}

	return resolved, ready
}

// ProcessEqual compares the parts of the spec that describe the process itself.
func ProcessEqual(s *ContainerSpecSpec, i *ContainerInstanceSpecSpec) bool {
	return slices.Equal(s.Entrypoint, i.Entrypoint) &&
		slices.Equal(s.Args, i.Args) &&
		s.WorkingDir == i.WorkingDir &&
		RunAsEqual(s.RunAs, i.RunAs) &&
		slices.Equal(s.Environment, i.Environment)
}

// SecurityEqual compares two security specs field by field, as they carry slices.
func SecurityEqual(a, b ContainerSecuritySpec) bool {
	return a.Privileged == b.Privileged &&
		slices.Equal(a.CapabilitiesAdd, b.CapabilitiesAdd) &&
		slices.Equal(a.CapabilitiesDrop, b.CapabilitiesDrop)
}

// RunAsEqual compares two RunAs specs, treating nil UID/GID halves as equal only to each other.
func RunAsEqual(a, b ContainerRunAsSpec) bool {
	return Int32PtrEqual(a.UID, b.UID) && Int32PtrEqual(a.GID, b.GID)
}

// Int32PtrEqual compares two int32 pointers, treating nil as equal only to nil.
func Int32PtrEqual(a, b *int32) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

// ResolvedMountsEqual compares two resolved mount lists field by field, as they carry slices.
func ResolvedMountsEqual(a, b []ResolvedMountSpec) bool {
	return slices.EqualFunc(a, b, func(x, y ResolvedMountSpec) bool {
		return x.Kind == y.Kind &&
			x.Source == y.Source &&
			x.Destination == y.Destination &&
			x.Size == y.Size &&
			slices.Equal(x.Options, y.Options)
	})
}
