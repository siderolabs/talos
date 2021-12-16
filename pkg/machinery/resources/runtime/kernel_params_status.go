// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// KernelParamStatusType is type of KernelParam resource.
const KernelParamStatusType = resource.Type("KernelParamStatuses.runtime.talos.dev")

// KernelParamStatus resource holds defined sysctl flags status.
type KernelParamStatus struct {
	md   resource.Metadata
	spec KernelParamStatusSpec
}

// KernelParamStatusSpec describes status of the defined sysctls.
type KernelParamStatusSpec struct {
	Current     string `yaml:"current"`
	Default     string `yaml:"default"`
	Unsupported bool   `yaml:"unsupported"`
}

// NewKernelParamStatus initializes a KernelParamStatus resource.
func NewKernelParamStatus(namespace resource.Namespace, id resource.ID) *KernelParamStatus {
	r := &KernelParamStatus{
		md:   resource.NewMetadata(namespace, KernelParamStatusType, id, resource.VersionUndefined),
		spec: KernelParamStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *KernelParamStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *KernelParamStatus) Spec() interface{} {
	return r.spec
}

func (r *KernelParamStatus) String() string {
	return fmt.Sprintf("runtime.KernelParamStatus.(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *KernelParamStatus) DeepCopy() resource.Resource {
	return &KernelParamStatus{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *KernelParamStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KernelParamStatusType,
		Aliases:          []resource.Type{"sysctls", "kernelparameters", "kernelparams"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Current",
				JSONPath: `{.current}`,
			},
			{
				Name:     "Default",
				JSONPath: `{.default}`,
			},
			{
				Name:     "Unsupported",
				JSONPath: `{.unsupported}`,
			},
		},
	}
}

// TypedSpec allows to access the KernelParamStatusSpec with the proper type.
func (r *KernelParamStatus) TypedSpec() *KernelParamStatusSpec {
	return &r.spec
}
