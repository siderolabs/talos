// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"fmt"
	"io/fs"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// EtcFileSpecType is type of EtcFile resource.
const EtcFileSpecType = resource.Type("EtcFileSpecs.files.talos.dev")

// EtcFileSpec resource holds contents of the file which should be put to `/etc` directory.
type EtcFileSpec struct {
	md   resource.Metadata
	spec EtcFileSpecSpec
}

// EtcFileSpecSpec describes status of rendered secrets.
type EtcFileSpecSpec struct {
	Contents []byte      `yaml:"contents"`
	Mode     fs.FileMode `yaml:"mode"`
}

// NewEtcFileSpec initializes a EtcFileSpec resource.
func NewEtcFileSpec(namespace resource.Namespace, id resource.ID) *EtcFileSpec {
	r := &EtcFileSpec{
		md:   resource.NewMetadata(namespace, EtcFileSpecType, id, resource.VersionUndefined),
		spec: EtcFileSpecSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *EtcFileSpec) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *EtcFileSpec) Spec() interface{} {
	return r.spec
}

func (r *EtcFileSpec) String() string {
	return fmt.Sprintf("network.EtcFileSpec(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *EtcFileSpec) DeepCopy() resource.Resource {
	return &EtcFileSpec{
		md: r.md,
		spec: EtcFileSpecSpec{
			Contents: append([]byte(nil), r.spec.Contents...),
			Mode:     r.spec.Mode,
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *EtcFileSpec) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcFileSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *EtcFileSpec) TypedSpec() *EtcFileSpecSpec {
	return &r.spec
}
