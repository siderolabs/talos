// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ProbeStatusType is type of ProbeStatus resource.
const ProbeStatusType = resource.Type("ProbeStatuses.net.talos.dev")

// ProbeStatus resource holds Probe result.
type ProbeStatus = typed.Resource[ProbeStatusSpec, ProbeStatusExtension]

// ProbeStatusSpec describes the Probe.
//
//gotagsrewrite:gen
type ProbeStatusSpec struct {
	// Success of the check.
	Success bool `yaml:"success" protobuf:"1"`
	// Last error of the probe.
	LastError string `yaml:"lastError" protobuf:"2"`
}

// NewProbeStatus initializes a ProbeStatus resource.
func NewProbeStatus(namespace resource.Namespace, id resource.ID) *ProbeStatus {
	return typed.NewResource[ProbeStatusSpec, ProbeStatusExtension](
		resource.NewMetadata(namespace, ProbeStatusType, id, resource.VersionUndefined),
		ProbeStatusSpec{},
	)
}

// ProbeStatusExtension provides auxiliary methods for ProbeStatus.
type ProbeStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ProbeStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ProbeStatusType,
		Aliases:          []resource.Type{"probe", "probes"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Success",
				JSONPath: "{.success}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ProbeStatusSpec](ProbeStatusType, &ProbeStatus{})
	if err != nil {
		panic(err)
	}
}
