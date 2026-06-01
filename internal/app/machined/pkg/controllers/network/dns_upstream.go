// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/coredns/coredns/plugin/pkg/proxy"
	"github.com/coredns/coredns/plugin/pkg/transport"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/dns/doh"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// DNSUpstreamController is a controller that manages DNS upstreams.
type DNSUpstreamController struct{}

// Name implements controller.Controller interface.
func (ctrl *DNSUpstreamController) Name() string {
	return "network.DNSUpstreamController"
}

// Inputs implements controller.Controller interface.
func (ctrl *DNSUpstreamController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.HostDNSConfigType,
			ID:        optional.Some(network.HostDNSConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.ResolverStatusType,
			ID:        optional.Some(network.ResolverID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *DNSUpstreamController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.DNSUpstreamType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *DNSUpstreamController) Run(ctx context.Context, r controller.Runtime, l *zap.Logger) error {
	defer cleanupUpstream(context.Background(), r, nil, l)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.run(ctx, r, l); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *DNSUpstreamController) run(ctx context.Context, r controller.Runtime, l *zap.Logger) error {
	touchedIDs := map[resource.ID]struct{}{}

	defer cleanupUpstream(ctx, r, touchedIDs, l)

	cfg, err := safe.ReaderGetByID[*network.HostDNSConfig](ctx, r, network.HostDNSConfigID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if !cfg.TypedSpec().Enabled {
		// host DNS is disabled, cleanup all upstreams
		return nil
	}

	rs, err := safe.ReaderGetByID[*network.ResolverStatus](ctx, r, network.ResolverID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	initConn, err := existingConnections(ctx, r)
	if err != nil {
		return err
	}

	for i, srv := range rs.TypedSpec().NameServers {
		if err = safe.WriterModify[*network.DNSUpstream](
			ctx,
			r,
			network.NewDNSUpstream(fmt.Sprintf("#%03d %s %s", i, srv.Protocol, srv.Addr)),
			func(u *network.DNSUpstream) error {
				touchedIDs[u.Metadata().ID()] = struct{}{}

				initConn(&u.TypedSpec().Value, srv.Protocol, srv.Addr.String(), srv.TLSServerName, l)

				return nil
			},
		); err != nil {
			return err
		}
	}

	return nil
}

func existingConnections(ctx context.Context, r controller.Runtime) (func(*network.DNSUpstreamSpecSpec, nethelpers.DNSProtocol, string, string, *zap.Logger), error) {
	upstream, err := safe.ReaderListAll[*network.DNSUpstream](ctx, r)
	if err != nil {
		return nil, err
	}

	existingConn := make(map[string]*network.DNSConn, upstream.Len())

	for u := range upstream.All() {
		existingConn[u.TypedSpec().Value.Conn.Addr()] = u.TypedSpec().Value.Conn
	}

	return func(spec *network.DNSUpstreamSpecSpec, protocol nethelpers.DNSProtocol, remoteHost, tlsServerName string, l *zap.Logger) {
		var port string

		switch protocol {
		case nethelpers.DNSProtocolDefault:
			port = transport.Port
		case nethelpers.DNSProtocolDNSOverTLS:
			port = transport.TLSPort
		case nethelpers.DNSProtocolDNSOverHTTP:
			port = transport.HTTPSPort
		default:
			panic(fmt.Sprintf("unsupported DNS protocol: %s", protocol))
		}

		remoteAddr := net.JoinHostPort(remoteHost, port)
		if spec.Conn != nil && spec.Conn.Addr() == remoteAddr {
			l.Debug("reusing existing upstream spec", zap.String("addr", remoteAddr))

			return
		}

		defer func(c *network.DNSConn) {
			if c != nil {
				c.Close()
			}
		}(spec.Conn)

		if conn, ok := existingConn[remoteAddr]; ok {
			spec.Conn = conn.NewRef()

			l.Debug("reusing existing upstream connection", zap.String("addr", remoteAddr))

			return
		}

		spec.Conn = network.NewDNSConn(newUpstreamProxy(protocol, remoteHost, remoteAddr, tlsServerName))

		l.Debug(
			"created new upstream connection",
			zap.String("addr", remoteAddr),
			zap.Stringer("protocol", protocol),
			zap.String("tls_server_name", tlsServerName),
		)

		existingConn[remoteAddr] = spec.Conn
	}, nil
}

func newUpstreamProxy(protocol nethelpers.DNSProtocol, remoteHost, remoteAddr, tlsServerName string) network.Proxy {
	switch protocol {
	case nethelpers.DNSProtocolDefault:
		return proxy.NewProxy(remoteHost, remoteAddr, transport.DNS)
	case nethelpers.DNSProtocolDNSOverTLS:
		p := proxy.NewProxy(remoteHost, remoteAddr, transport.TLS)
		p.SetTLSConfig(&tls.Config{
			ServerName: tlsServerName,
			MinVersion: tls.VersionTLS13,
		})

		return p
	case nethelpers.DNSProtocolDNSOverHTTP:
		return doh.NewProxy(remoteAddr, tlsServerName)
	default:
		panic(fmt.Sprintf("unsupported DNS protocol: %s", protocol))
	}
}

func cleanupUpstream(ctx context.Context, r controller.Runtime, touchedIDs map[resource.ID]struct{}, l *zap.Logger) {
	list, err := safe.ReaderListAll[*network.DNSUpstream](ctx, r)
	if err != nil {
		l.Error("error listing upstreams", zap.Error(err))

		return
	}

	for val := range list.All() {
		md := val.Metadata()

		if _, ok := touchedIDs[md.ID()]; ok {
			continue
		}

		if conn := val.TypedSpec().Value.Conn; conn != nil {
			conn.Close()
		}

		if err = r.Destroy(ctx, md); err != nil {
			l.Error("error destroying upstream", zap.Error(err), zap.String("id", md.ID()))

			return
		}

		l.Debug("destroyed dns upstream", zap.String("addr", md.ID()))
	}
}
