// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"runtime"
	"strconv"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/handle"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"go.uber.org/zap"
)

// DNSUpstreamType is type of DNSUpstream resource.
const DNSUpstreamType = resource.Type("DNSUpstreams.net.talos.dev")

// DNSUpstream resource holds DNS resolver info.
type DNSUpstream = typed.Resource[DNSUpstreamSpec, DNSUpstreamExtension]

// DNSUpstreamSpec describes DNS upstreams status.
type DNSUpstreamSpec = handle.ResourceSpec[DNSUpstreamSpecSpec]

// DNSUpstreamSpecSpec describes DNS upstreams status.
type DNSUpstreamSpecSpec struct {
	Conn *DNSConn
}

// MarshalYAML implements yaml.Marshaler interface.
func (d DNSUpstreamSpecSpec) MarshalYAML() (any, error) {
	d.Conn.Healthcheck()

	return map[string]string{
		"healthy": strconv.FormatBool(d.Conn.Fails() == 0),
		"addr":    d.Conn.Addr(),
	}, nil
}

// NewDNSUpstream initializes a DNSUpstream resource.
func NewDNSUpstream(id resource.ID) *DNSUpstream {
	return typed.NewResource[DNSUpstreamSpec, DNSUpstreamExtension](
		resource.NewMetadata(NamespaceName, DNSUpstreamType, id, resource.VersionUndefined),
		DNSUpstreamSpec{Value: DNSUpstreamSpecSpec{}},
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
	Start(time.Duration)
}

// DNSConn is a wrapper around a Proxy.
type DNSConn struct {
	// Proxy is essentially a *proxy.Proxy interface. It's here because we don't want machinery to depend on coredns.
	// We could use a generic struct here, but without generic aliases the usage would look ugly.
	// Once generic aliases are here, redo the type above as `type DNSUpstream[P Proxy] = typed.Resource[...]`.
	proxy Proxy
}

// NewDNSConn initializes a new DNSConn.
func NewDNSConn(proxy Proxy, l *zap.Logger) *DNSConn {
	proxy.Start(500 * time.Millisecond)

	conn := &DNSConn{proxy: proxy}

	// Set the finalizer to stop the proxy when the DNSConn is garbage collected. Since the proxy already uses a finalizer
	// to stop the actual connections, this will not carry any noticeable performance overhead.
	//
	// TODO: replace with runtime.AddCleanup once https://github.com/golang/go/issues/67535 lands
	runtime.SetFinalizer(conn, func(conn *DNSConn) {
		conn.proxy.Stop()

		l.Info("dns connection garbage collected", zap.String("addr", conn.proxy.Addr()))
	})

	return conn
}

// Addr returns the address of the DNSConn.
func (u *DNSConn) Addr() string { return u.proxy.Addr() }

// Fails returns the number of fails of the DNSConn.
func (u *DNSConn) Fails() uint32 { return u.proxy.Fails() }

// Proxy returns the Proxy field of the DNSConn.
func (u *DNSConn) Proxy() Proxy { return u.proxy }

// Healthcheck kicks of a round of health checks for this DNSConn.
func (u *DNSConn) Healthcheck() { u.proxy.Healthcheck() }
