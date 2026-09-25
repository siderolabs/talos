// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package domain manages transient QEMU domains owned by this machine.
package domain

import (
	"context"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"net"
	"time"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"libvirt.org/go/libvirtxml"
)

const operationTimeout = 5 * time.Second

// Connector opens bounded sessions against one modular QEMU daemon.
type Connector struct {
	socket string
	uri    string
}

// New configures the daemon endpoint without opening a connection.
func New(socket, uri string) *Connector {
	return &Connector{socket: socket, uri: uri}
}

// Open bounds dialing, handshake, operations, and disconnect. go-libvirt RPCs
// have no context argument, so cancellation closes the transport.
func (c *Connector) Open(ctx context.Context) (Client, error) {
	sessionCtx, cancel := context.WithTimeout(ctx, operationTimeout)

	conn, err := (&net.Dialer{}).DialContext(sessionCtx, "unix", c.socket)
	if err != nil {
		cancel()

		return nil, err
	}

	return c.OpenConn(sessionCtx, conn, cancel)
}

// OpenConn connects over an already-connected transport for this connector.
// It takes ownership of conn and cancel; ctx must bound the full session.
func (c *Connector) OpenConn(ctx context.Context, conn net.Conn, cancel context.CancelFunc) (Client, error) {
	rpc := libvirt.NewWithDialer(dialers.NewAlreadyConnected(conn))
	client := &client{rpc: rpc, conn: conn, cancel: cancel}

	client.stopClose = context.AfterFunc(ctx, func() { closeTransport(conn) })
	if err := rpc.ConnectToURI(libvirt.ConnectURI(c.uri)); err != nil {
		client.Close()

		return nil, err
	}

	client.disconnect = rpc.Disconnect

	return client, nil
}

// Changing the namespace changes ownership of all existing domains.
var namespace = uuid.NewHash(sha256.New(), uuid.NameSpaceURL, []byte("https://talos.dev/virtual-machine-domains"), 8)

// UUID derives a stable, host-specific domain identity from a validated machine UUID and name.
func UUID(machine uuid.UUID, name string) uuid.UUID {
	return uuid.NewHash(sha256.New(), namespace, append(machine[:], []byte(name)...), 8)
}

// Domain identifies a domain by name and UUID; it does not prove ownership.
type Domain struct {
	Name string
	UUID uuid.UUID
}

// Client represents one bounded reconciliation session; callers must Close it.
type Client interface {
	Domains() ([]Domain, error)
	Start(Domain, string) error
	Remove(Domain) error
	Close()
}

type domainRPC interface {
	listRPC
	definitionRPC
	lifecycleRPC
}

type listRPC interface {
	ConnectListAllDomains(int32, libvirt.ConnectListAllDomainsFlags) ([]libvirt.Domain, uint32, error)
	DomainLookupByName(string) (libvirt.Domain, error)
}

type definitionRPC interface {
	DomainGetXMLDesc(libvirt.Domain, libvirt.DomainXMLFlags) (string, error)
	DomainIsPersistent(libvirt.Domain) (int32, error)
	DomainCreateXML(string, libvirt.DomainCreateFlags) (libvirt.Domain, error)
}

type lifecycleRPC interface {
	DomainIsActive(libvirt.Domain) (int32, error)
	DomainDestroy(libvirt.Domain) error
	DomainHasManagedSaveImage(libvirt.Domain, uint32) (int32, error)
	DomainManagedSaveRemove(libvirt.Domain, uint32) error
	DomainUndefineFlags(libvirt.Domain, libvirt.DomainUndefineFlagsValues) error
}

type client struct {
	rpc        domainRPC
	conn       net.Conn
	cancel     context.CancelFunc
	stopClose  func() bool
	disconnect func() error
}

func (c *client) Close() {
	defer closeTransport(c.conn)
	defer c.cancel()
	defer c.stopClose()

	if c.disconnect != nil {
		if err := c.disconnect(); err != nil {
			zap.L().Debug("failed to disconnect from libvirt", zap.Error(err))
		}
	}
}

func closeTransport(conn net.Conn) {
	if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		zap.L().Debug("failed to close libvirt transport", zap.Error(err))
	}
}

func (c *client) Domains() ([]Domain, error) {
	// The first argument is need_results (1 = return the list), not a result limit.
	// Zero flags include both active and inactive domains.
	domains, _, err := c.rpc.ConnectListAllDomains(1, 0)
	if err != nil {
		return nil, err
	}

	result := make([]Domain, 0, len(domains))
	for _, d := range domains {
		result = append(result, Domain{Name: d.Name, UUID: uuid.UUID(d.UUID)})
	}

	return result, nil
}

func (c *client) lookup(d Domain) (libvirt.Domain, bool, error) {
	found, err := c.rpc.DomainLookupByName(d.Name)
	if err != nil {
		if rpcErr, ok := errors.AsType[libvirt.Error](err); ok && rpcErr.Code == uint32(libvirt.ErrNoDomain) {
			return libvirt.Domain{}, false, nil
		}

		return libvirt.Domain{}, false, err
	}

	if found.UUID != libvirt.UUID(d.UUID) {
		return libvirt.Domain{}, false, fmt.Errorf("domain %q is not owned by this machine", d.Name)
	}

	return found, true, nil
}

func validateDomain(d Domain) error {
	if d.Name == "" || d.UUID == uuid.Nil {
		return errors.New("domain name and UUID must be nonempty")
	}

	return nil
}

// Start creates an active transient domain or replaces an owned domain when its
// definition changes. Replacing a running domain interrupts the guest.
func (c *client) Start(d Domain, renderedXML string) error {
	if err := validateDomain(d); err != nil {
		return err
	}

	desired, digest, err := domainDefinition(d, renderedXML)
	if err != nil {
		return err
	}

	found, exists, err := c.lookup(d)
	if err != nil {
		return err
	}

	restart, err := c.needsRestart(found, exists, digest)
	if err != nil {
		return err
	}

	if !restart {
		return nil
	}

	if exists {
		if err = c.Remove(d); err != nil {
			return err
		}
	}

	return c.createTransient(d, desired)
}

func (c *client) createTransient(d Domain, desired string) error {
	found, err := c.rpc.DomainCreateXML(desired, 0)
	if err != nil {
		return err
	}

	if found.UUID != libvirt.UUID(d.UUID) {
		return fmt.Errorf("started domain %q has unexpected UUID", d.Name)
	}

	persistent, err := c.rpc.DomainIsPersistent(found)
	if err != nil {
		return err
	}

	if persistent != 0 {
		return fmt.Errorf("started domain %q is unexpectedly persistent", d.Name)
	}

	return nil
}

const metadataNamespace = "https://talos.dev/libvirt/domain"

func domainDefinition(d Domain, renderedXML string) (string, string, error) {
	var desc libvirtxml.Domain

	if err := desc.Unmarshal(renderedXML); err != nil {
		return "", "", fmt.Errorf("invalid domain XML: %w", err)
	}

	if desc.Name != d.Name {
		return "", "", fmt.Errorf("domain XML name %q does not match %q", desc.Name, d.Name)
	}

	desc.UUID = d.UUID.String()

	canonical, err := desc.Marshal()
	if err != nil {
		return "", "", fmt.Errorf("marshal domain XML: %w", err)
	}

	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(canonical)))

	if desc.Metadata == nil {
		desc.Metadata = &libvirtxml.DomainMetadata{}
	}

	desc.Metadata.XML += fmt.Sprintf(`<talos:definition xmlns:talos="%s">%s</talos:definition>`, metadataNamespace, digest)

	definition, err := desc.Marshal()
	if err != nil {
		return "", "", fmt.Errorf("marshal domain metadata: %w", err)
	}

	return definition, digest, nil
}

func (c *client) needsRestart(found libvirt.Domain, exists bool, digest string) (bool, error) {
	if !exists {
		return true, nil
	}

	currentDigest, err := c.definitionDigest(found)
	if err != nil {
		return false, err
	}

	persistent, err := c.rpc.DomainIsPersistent(found)
	if err != nil {
		return false, err
	}

	if persistent != 0 {
		return true, nil
	}

	active, err := c.rpc.DomainIsActive(found)
	if err != nil {
		return false, err
	}

	return active == 0 || currentDigest != digest, nil
}

func (c *client) definitionDigest(found libvirt.Domain) (string, error) {
	currentXML, err := c.rpc.DomainGetXMLDesc(found, 0)
	if err != nil {
		return "", err
	}

	var current libvirtxml.Domain

	if err = current.Unmarshal(currentXML); err != nil {
		return "", err
	}

	if current.Metadata == nil {
		return "", fmt.Errorf("domain %q has no Talos ownership metadata", found.Name)
	}

	var metadata struct {
		Digest string `xml:"https://talos.dev/libvirt/domain definition"`
	}

	if err = xml.Unmarshal([]byte("<metadata>"+current.Metadata.XML+"</metadata>"), &metadata); err != nil {
		return "", fmt.Errorf("invalid domain metadata: %w", err)
	}

	if metadata.Digest == "" {
		return "", fmt.Errorf("domain %q has no Talos ownership metadata", found.Name)
	}

	return metadata.Digest, nil
}

func (c *client) removeManagedSave(found libvirt.Domain) error {
	persistent, err := c.rpc.DomainIsPersistent(found)
	if err != nil || persistent == 0 {
		return err
	}

	saved, err := c.rpc.DomainHasManagedSaveImage(found, 0)
	if err != nil || saved == 0 {
		return err
	}

	return c.rpc.DomainManagedSaveRemove(found, 0)
}

func (c *client) stopIfActive(found libvirt.Domain) error {
	active, err := c.rpc.DomainIsActive(found)
	if err != nil || active == 0 {
		return err
	}

	return c.rpc.DomainDestroy(found)
}

// Remove destroys a claimed domain. Previously defined persistent domains are
// also undefined so upgrading the controller cannot leave them behind.
func (c *client) Remove(d Domain) error {
	if err := validateDomain(d); err != nil {
		return err
	}

	found, exists, err := c.lookup(d)
	if err != nil || !exists {
		return err
	}

	if _, err = c.definitionDigest(found); err != nil {
		return err
	}

	persistent, err := c.rpc.DomainIsPersistent(found)
	if err != nil {
		return err
	}

	if err = c.stopIfActive(found); err != nil {
		return err
	}

	if persistent == 0 {
		return nil
	}

	if err = c.removeManagedSave(found); err != nil {
		return err
	}

	return c.rpc.DomainUndefineFlags(found, libvirt.DomainUndefineManagedSave|libvirt.DomainUndefineNvram)
}
