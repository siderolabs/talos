// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// SecurityStateType is the type of the security state resource.
const SecurityStateType = resource.Type("SecurityStates.talos.dev")

// SecurityStateID is the ID of the security state resource.
const SecurityStateID = resource.ID("securitystate")

// SecurityState is the security state resource.
type SecurityState = typed.Resource[SecurityStateSpec, SecurityStateExtension]

//go:generate enumer -type=SELinuxState -linecomment -text

// SELinuxState describes the current SELinux status.
type SELinuxState int

// SELinux state.
//
//structprotogen:gen_enum
const (
	SELinuxStateDisabled   SELinuxState = iota // disabled
	SELinuxStatePermissive                     // enabled, permissive
	SELinuxStateEnforcing                      // enabled, enforcing
)

// SecurityStateSpec describes the security state resource properties.
//
//gotagsrewrite:gen
type SecurityStateSpec struct {
	SecureBoot               bool         `yaml:"secureBoot" protobuf:"1"`
	UKISigningKeyFingerprint string       `yaml:"ukiSigningKeyFingerprint,omitempty" protobuf:"2"`
	PCRSigningKeyFingerprint string       `yaml:"pcrSigningKeyFingerprint,omitempty" protobuf:"3"`
	SELinuxState             SELinuxState `yaml:"selinuxState,omitempty" protobuf:"4"`
	BootedWithUKI            bool         `yaml:"bootedWithUKI,omitempty" protobuf:"5"`
}

// NewSecurityStateSpec initializes a security state resource.
func NewSecurityStateSpec(namespace resource.Namespace) *SecurityState {
	return typed.NewResource[SecurityStateSpec, SecurityStateExtension](
		resource.NewMetadata(namespace, SecurityStateType, SecurityStateID, resource.VersionUndefined),
		SecurityStateSpec{},
	)
}

// SecurityStateExtension provides auxiliary methods for SecurityState.
type SecurityStateExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (SecurityStateExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SecurityStateType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "SecureBoot",
				JSONPath: `{.secureBoot}`,
			},
			{
				Name:     "UKISigningKeyFingerprint",
				JSONPath: `{.ukiSigningKeyFingerprint}`,
			},
			{
				Name:     "PCRSigningKeyFingerprint",
				JSONPath: `{.pcrSigningKeyFingerprint}`,
			},
			{
				Name:     "SELinuxState",
				JSONPath: `{.selinuxState}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[SecurityStateSpec](SecurityStateType, &SecurityState{})
	if err != nil {
		panic(err)
	}
}
