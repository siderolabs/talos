// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package time

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// NTPStatusType is type of NTPStatus resource.
const NTPStatusType = resource.Type("NTPStatuses.v1alpha1.talos.dev")

// NTPStatusID is the ID of the singleton resource.
const NTPStatusID = resource.ID("node")

// NTPStatus describes the state of the NTP client.
//
// This resource is purely observational: it is updated on every NTP poll, and
// it is deliberately not an input to any other controller, so that the churn
// doesn't trigger reconciliation elsewhere (see [Status] instead).
type NTPStatus = typed.Resource[NTPStatusSpec, NTPStatusExtension]

// NTPStatusSpec describes the state of the NTP client.
//
//gotagsrewrite:gen
type NTPStatusSpec struct {
	// SpikeDetected indicates that the last time measurement was discarded by the spike filter.
	//
	// A discarded measurement is not applied to the clock at all, so a filter which keeps
	// rejecting looks exactly like a clock which is never corrected.
	SpikeDetected bool `yaml:"spikeDetected" protobuf:"1"`

	// ConsecutiveSpikes is the number of time measurements discarded in a row by the spike filter.
	ConsecutiveSpikes int `yaml:"consecutiveSpikes" protobuf:"2"`
}

// NewNTPStatus initializes an NTPStatus resource.
func NewNTPStatus() *NTPStatus {
	return typed.NewResource[NTPStatusSpec, NTPStatusExtension](
		resource.NewMetadata(v1alpha1.NamespaceName, NTPStatusType, NTPStatusID, resource.VersionUndefined),
		NTPStatusSpec{},
	)
}

// NTPStatusExtension provides auxiliary methods for NTPStatus.
type NTPStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (NTPStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NTPStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: v1alpha1.NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Spike Detected",
				JSONPath: "{.spikeDetected}",
			},
			{
				Name:     "Consecutive Spikes",
				JSONPath: "{.consecutiveSpikes}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[NTPStatusSpec](NTPStatusType, &NTPStatus{})
	if err != nil {
		panic(err)
	}
}
