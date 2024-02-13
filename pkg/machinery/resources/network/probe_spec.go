// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"errors"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ProbeSpecType is type of ProbeSpec resource.
const ProbeSpecType = resource.Type("ProbeSpecs.net.talos.dev")

// ProbeSpec resource holds Probe specification to be run.
type ProbeSpec = typed.Resource[ProbeSpecSpec, ProbeSpecExtension]

// ProbeSpecSpec describes the Probe.
//
//gotagsrewrite:gen
type ProbeSpecSpec struct {
	// Interval between the probes.
	Interval time.Duration `yaml:"interval" protobuf:"1"`
	// FailureThreshold is the number of consecutive failures for the probe to be considered failed after having succeeded.
	FailureThreshold int `yaml:"failureThreshold" protobuf:"2"`
	// One of the probe types should be specified, for now it's only TCP.
	TCP TCPProbeSpec `yaml:"tcp,omitempty" protobuf:"3"`
	// Configuration layer.
	ConfigLayer ConfigLayer `yaml:"layer" protobuf:"4"`
}

// ID returns the ID of the resource based on the spec.
func (spec *ProbeSpecSpec) ID() (resource.ID, error) {
	var zeroTCP TCPProbeSpec

	if spec.TCP == zeroTCP {
		return "", errors.New("no probe type specified")
	}

	return fmt.Sprintf("tcp:%s", spec.TCP.Endpoint), nil
}

// Equal returns true if the specs are equal.
func (spec ProbeSpecSpec) Equal(other ProbeSpecSpec) bool {
	return spec == other
}

// TCPProbeSpec describes the TCP Probe.
//
//gotagsrewrite:gen
type TCPProbeSpec struct {
	// Endpoint to probe: host:port.
	Endpoint string `yaml:"endpoint" protobuf:"1"`
	// Timeout for the probe.
	Timeout time.Duration `yaml:"timeout" protobuf:"2"`
}

// NewProbeSpec initializes a ProbeSpec resource.
func NewProbeSpec(namespace resource.Namespace, id resource.ID) *ProbeSpec {
	return typed.NewResource[ProbeSpecSpec, ProbeSpecExtension](
		resource.NewMetadata(namespace, ProbeSpecType, id, resource.VersionUndefined),
		ProbeSpecSpec{},
	)
}

// ProbeSpecExtension provides auxiliary methods for ProbeSpec.
type ProbeSpecExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (ProbeSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ProbeSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ProbeSpecSpec](ProbeSpecType, &ProbeSpec{})
	if err != nil {
		panic(err)
	}
}
