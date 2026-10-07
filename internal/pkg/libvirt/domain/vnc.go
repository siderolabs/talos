// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"sync"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"libvirt.org/go/libvirtxml"
)

// OpenVNC attaches to the live, libvirt-allocated Unix graphics socket. Checks
// before and after dialing share one control connection and a five-second bound.
// libvirt allocates monotonically increasing active IDs in this daemon lifetime;
// exhausting the signed ID allocator during this bound is not supported.
func (c *Connector) OpenVNC(ctx context.Context, d Domain) (io.ReadWriteCloser, error) {
	if err := validateDomain(d); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	startup, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	raw, err := (&net.Dialer{}).DialContext(startup, "unix", c.socket)
	if err != nil {
		return nil, err
	}

	return c.openVNCConn(ctx, startup, raw, d, (&net.Dialer{}).DialContext)
}

//nolint:gocyclo // Keep attachment failure cleanup adjacent to each setup operation.
func (c *Connector) openVNCConn(ctx, startup context.Context, raw net.Conn, d Domain, dial func(context.Context, string, string) (net.Conn, error)) (_ io.ReadWriteCloser, retErr error) {
	session, cancel := context.WithCancel(ctx)

	stopStartup := context.AfterFunc(startup, cancel)
	defer stopStartup()

	stopControl := context.AfterFunc(session, func() { closeTransport(raw) })
	rpc := libvirt.NewWithDialer(dialers.NewAlreadyConnected(raw))

	success := false
	defer func() {
		if !success {
			cancel()
			stopControl()
			closeTransport(raw)
		}

		if retErr != nil && startup.Err() != nil {
			retErr = startup.Err()
		}
	}()

	if err := rpc.ConnectToURI(libvirt.ConnectURI(c.uri)); err != nil {
		return nil, err
	}

	go func() {
		select {
		case <-rpc.Disconnected():
			cancel()
		case <-session.Done():
		}
	}()

	before, socket, err := vncDomain(rpc, d)
	if err != nil {
		return nil, err
	}

	conn, err := dial(session, "unix", socket)
	if err != nil {
		return nil, err
	}

	stopGraphics := context.AfterFunc(session, func() { closeTransport(conn) })

	defer func() {
		if !success {
			stopGraphics()
			closeTransport(conn)
		}
	}()

	after, path, err := vncDomain(rpc, d)
	if err != nil {
		return nil, err
	}

	if before != after || socket != path {
		return nil, consolePreconditionError("VNC domain incarnation or socket changed during attachment")
	}

	if err = startup.Err(); err != nil {
		return nil, err
	}

	if err = session.Err(); err != nil {
		return nil, err
	}

	if !stopStartup() {
		return nil, context.DeadlineExceeded
	}

	success = true

	return &vncStream{Conn: conn, close: func() {
		cancel()
		closeTransport(conn)
		closeTransport(raw)
		stopGraphics()
		stopControl()
	}}, nil
}

func vncDomain(rpc *libvirt.Libvirt, d Domain) (libvirt.Domain, string, error) {
	found, err := rpc.DomainLookupByName(d.Name)
	if err != nil {
		return found, "", err
	}

	if found.Name != d.Name || found.UUID != libvirt.UUID(d.UUID) || found.ID <= 0 {
		return found, "", consolePreconditionError("VNC domain identity or active ID mismatch")
	}

	active, err := rpc.DomainIsActive(found)
	if err != nil {
		return found, "", err
	}

	if active != 1 {
		return found, "", consolePreconditionError("VNC domain is not active")
	}

	text, err := rpc.DomainGetXMLDesc(found, 0)
	if err != nil {
		return found, "", err
	}

	socket, err := vncSocket(text, d, found.ID)

	return found, socket, err
}

//nolint:gocyclo // Validate the complete live identity and endpoint before dialing.
func vncSocket(text string, d Domain, id int32) (string, error) {
	var desc libvirtxml.Domain
	if err := desc.Unmarshal(text); err != nil {
		return "", consolePreconditionError("malformed VNC domain XML")
	}

	if desc.Name != d.Name || desc.UUID != d.UUID.String() || desc.Metadata == nil {
		return "", consolePreconditionError("VNC domain XML ownership mismatch")
	}

	var metadata struct {
		Digest string `xml:"https://talos.dev/libvirt/domain definition"`
	}
	if err := xml.Unmarshal([]byte("<metadata>"+desc.Metadata.XML+"</metadata>"), &metadata); err != nil || metadata.Digest == "" {
		return "", consolePreconditionError("VNC domain has no Talos ownership metadata")
	}

	if desc.Devices == nil || len(desc.Devices.Graphics) != 1 || desc.Devices.Graphics[0].VNC == nil {
		return "", consolePreconditionError("VNC domain has no unique VNC endpoint")
	}

	vnc := desc.Devices.Graphics[0].VNC
	if len(vnc.Listeners) != 1 || vnc.Listeners[0].Socket == nil || vnc.Listen != "" || vnc.Port > 0 {
		return "", consolePreconditionError("VNC endpoint is not a Unix socket")
	}

	socket := vnc.Listeners[0].Socket.Socket
	if vnc.Socket != "" && vnc.Socket != socket {
		return "", consolePreconditionError("VNC socket attributes disagree")
	}

	return vncSocketPath(d, id, socket)
}

func vncSocketPath(d Domain, id int32, socket string) (string, error) {
	// Talos VM names are ASCII; libvirt prefixes their first 20 characters with the active ID.
	name := d.Name
	if len(name) > 20 {
		name = name[:20]
	}

	if id <= 0 || !filepath.IsAbs(socket) || filepath.Clean(socket) != socket || filepath.Base(socket) != "vnc.sock" || filepath.Base(filepath.Dir(socket)) != fmt.Sprintf("domain-%d-%s", id, name) {
		return "", consolePreconditionError("VNC socket is not bound to the active domain ID")
	}

	return socket, nil
}

type vncStream struct {
	net.Conn
	once  sync.Once
	close func()
}

func (s *vncStream) Close() error {
	s.once.Do(s.close)

	return nil
}
