// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"strconv"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/handle"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// DNSUpstreamType is type of DNSUpstream resource.
const DNSUpstreamType = resource.Type("DNSUpstreams.net.talos.dev")

// DNSUpstream resource holds DNS resolver info.
type DNSUpstream = typed.Resource[DNSUpstreamSpec, DNSUpstreamExtension]

// DNSUpstreamSpec describes DNS upstreams status.
type DNSUpstreamSpec = handle.ResourceSpec[*DNSUpstreamSpecSpec]

// DNSUpstreamSpecSpec describes DNS upstreams status.
type DNSUpstreamSpecSpec struct {
	// Proxy is essentially a *proxy.Proxy interface. It's here because we don't want machinery to depend on coredns.
	// We could use a generic struct here, but without generic aliases the usage would look ugly.
	// Once generic aliases are here, redo the type above as `type DNSUpstream[P Proxy] = typed.Resource[...]`.
	Prx Proxy
}

// MarshalYAML implements yaml.Marshaler interface.
func (d *DNSUpstreamSpecSpec) MarshalYAML() (interface{}, error) {
	d.Prx.Healthcheck()

	return map[string]string{
		"healthy": strconv.FormatBool(d.Prx.Fails() == 0),
		"addr":    d.Prx.Addr(),
	}, nil
}

// NewDNSUpstream initializes a DNSUpstream resource.
func NewDNSUpstream(id resource.ID) *DNSUpstream {
	return typed.NewResource[DNSUpstreamSpec, DNSUpstreamExtension](
		resource.NewMetadata(NamespaceName, DNSUpstreamType, id, resource.VersionUndefined),
		DNSUpstreamSpec{Value: &DNSUpstreamSpecSpec{}},
	)
}

// DNSUpstreamExtension provides auxiliary methods for DNSUpstream.
type DNSUpstreamExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (DNSUpstreamExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DNSUpstreamType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Healthy",
				JSONPath: "{.healthy}",
			},
			{
				Name:     "Address",
				JSONPath: "{.addr}",
			},
		},
	}
}

// Proxy is essentially a proxy.Proxy interface. It's here because we don't want machinery to depend on coredns.
// The good thing we don't need any additional methods from coredns, so we can use a simple interface.
type Proxy interface {
	Addr() string
	Fails() uint32
	Healthcheck()
	Stop()
	Start(duration time.Duration)
}
