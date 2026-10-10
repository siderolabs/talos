// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package storage manages host-local persistent directory pools.
package storage

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"time"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"github.com/google/uuid"
	"libvirt.org/go/libvirtxml"
)

const operationTimeout = 5 * time.Second

// VolumeOperationTimeout is the budget a session doing volume work is given.
//
// Larger than operationTimeout: creating a volume is real filesystem work behind an RPC, where pool
// metadata calls are not. A session which expires mid-create has its transport closed while libvirt
// carries on server-side, which is exactly what the staged name in volume.go exists to survive.
const VolumeOperationTimeout = 2 * time.Minute

// Connector opens bounded sessions against one modular storage daemon.
type Connector struct {
	socket string
	uri    string
}

// New configures the daemon endpoint without opening a connection.
func New(socket, uri string) *Connector {
	return &Connector{socket: socket, uri: uri}
}

// Open bounds dialing, the handshake, RPCs and graceful disconnect with one
// timeout context. Cancellation closes the transport because go-libvirt RPCs
// do not accept a context.
func (c *Connector) Open(ctx context.Context) (EvidenceClient, error) {
	return c.OpenWithTimeout(ctx, operationTimeout)
}

// OpenWithTimeout is Open with an explicit session budget, for callers whose work does not fit the
// default one.
func (c *Connector) OpenWithTimeout(ctx context.Context, timeout time.Duration) (EvidenceClient, error) {
	sessionCtx, cancel := context.WithTimeout(ctx, timeout)

	conn, err := (&net.Dialer{}).DialContext(sessionCtx, "unix", c.socket)
	if err != nil {
		cancel()

		return nil, err
	}

	return c.OpenConn(sessionCtx, conn, cancel)
}

// OpenConn connects over an already-connected transport for this connector.
// It takes ownership of conn and cancel; ctx must bound the full session.
func (c *Connector) OpenConn(ctx context.Context, conn net.Conn, cancel context.CancelFunc) (EvidenceClient, error) {
	rpc := libvirt.NewWithDialer(dialers.NewAlreadyConnected(conn))
	client := &client{conn: conn, rpc: rpc, cancel: cancel}

	client.stopClose = context.AfterFunc(ctx, func() { closeTransport(conn) })
	if err := rpc.ConnectToURI(libvirt.ConnectURI(c.uri)); err != nil {
		client.Close()

		return nil, err
	}

	// Only a completed handshake may send CONNECT_CLOSE during cleanup.
	client.disconnect = rpc.Disconnect

	return client, nil
}

// The URL is a fixed namespace identifier, not a network endpoint. Changing
// this identity changes pool UUIDs and breaks recognition of existing ownership.
var namespace = uuid.NewHash(sha256.New(), uuid.NameSpaceURL, []byte("https://talos.dev/storage-pools"), 8)

// UUID is stable across configuration changes and controller restarts, but
// deliberately differs across hosts. A fixed-width binary machine UUID makes
// the concatenation unambiguous, without relying on a separator convention.
// The caller must supply a validated, nonzero machine UUID.
//
// SHA-256 (version 8) rather than a SHA-1 version 5 UUID: SHA-1 panics under
// FIPS 140-only mode, and this runs at package init.
func UUID(machine uuid.UUID, name string) uuid.UUID {
	return uuid.NewHash(sha256.New(), namespace, append(machine[:], []byte(name)...), 8)
}

// Pool is the identity returned by libvirt, including inactive persistent pools.
type Pool struct {
	Name string
	UUID uuid.UUID
	// Target is the pool's directory as defined in libvirt; empty when unknown.
	Target string
	// Type, Active, Persistent and Autostart are observations, not desired state.
	Type       string
	Active     bool
	Persistent bool
	Autostart  bool
}

// OperationOutcome describes the evidence from this invocation, not recovery of a prior one.
// Unknown must be fenced: a lost mutating RPC reply can leave daemon work in flight.
type OperationOutcome uint8

const (
	NoMutationSubmitted OperationOutcome = iota
	Finished
	Unknown
)

// mutationEvidence is scoped to one operation. An unknown reply is sticky even
// if subsequent reads or RPCs happen to succeed.
type mutationEvidence struct{ outcome OperationOutcome }

func (e *mutationEvidence) record(err error) {
	if e.outcome == Unknown {
		return
	}

	if err == nil {
		e.outcome = Finished

		return
	}

	// The pinned go-libvirt client produces libvirt.Error only from a decoded
	// terminal StatusError reply. Every other error may be a lost reply (including
	// a write error from a partially transmitted request).
	if rpcErr, ok := errors.AsType[libvirt.Error](err); ok {
		// Every decoded status code is terminal; the specific code does not
		// change whether the daemon has finished processing this request.
		_ = rpcErr.Code
		e.outcome = Finished
	} else {
		e.outcome = Unknown
	}
}

// Client is a bounded session; callers must Close it after each reconciliation.
// A fresh connection on each pass avoids retaining poisoned RPC connections.
type Client interface {
	Pools() ([]Pool, error)
	Ensure(Pool, string, func() error) error
	Remove(Pool) error
	Stop(Pool) error

	// Volume looks a volume up within a pool. There is deliberately no verb which deletes one: a
	// volume outlives the configuration that asked for it, exactly as a pool's contents outlive the
	// pool, and nothing on this interface can take a guest's data away.
	Volume(Pool, string) (Volume, bool, error)
	CreateVolume(Pool, string, string, uint64, string, string) (Volume, error)
	ResizeVolume(Pool, string, uint64) error

	Close()
}

// EvidenceClient exposes per-invocation outcomes without changing legacy Client fakes.
// Callers needing a safe fence must use these methods, not infer it from Client errors.
type EvidenceClient interface {
	Client
	EnsureOperation(Pool, string, func() error) (OperationOutcome, error)
	RemoveOperation(Pool) (OperationOutcome, error)
	StopOperation(Pool) (OperationOutcome, error)
	ResizeVolumeOperation(Pool, string, uint64) (OperationOutcome, error)
}

//nolint:interfacebloat // libvirt's own RPC surface; it is mirrored here, not designed here.
type poolRPC interface {
	ConnectListAllStoragePools(int32, libvirt.ConnectListAllStoragePoolsFlags) ([]libvirt.StoragePool, uint32, error)
	StoragePoolLookupByName(string) (libvirt.StoragePool, error)
	StoragePoolGetXMLDesc(libvirt.StoragePool, libvirt.StorageXMLFlags) (string, error)
	StoragePoolIsPersistent(libvirt.StoragePool) (int32, error)
	StoragePoolGetAutostart(libvirt.StoragePool) (int32, error)
	StoragePoolSetAutostart(libvirt.StoragePool, int32) error
	StoragePoolIsActive(libvirt.StoragePool) (int32, error)
	StoragePoolDestroy(libvirt.StoragePool) error
	StoragePoolDefineXML(string, uint32) (libvirt.StoragePool, error)
	StoragePoolCreate(libvirt.StoragePool, libvirt.StoragePoolCreateFlags) error
	StoragePoolUndefine(libvirt.StoragePool) error
	StoragePoolRefresh(libvirt.StoragePool, uint32) error
	StorageVolLookupByName(libvirt.StoragePool, string) (libvirt.StorageVol, error)
	StorageVolCreateXML(libvirt.StoragePool, string, libvirt.StorageVolCreateFlags) (libvirt.StorageVol, error)
	StorageVolGetXMLDesc(libvirt.StorageVol, uint32) (string, error)
	StorageVolGetInfo(libvirt.StorageVol) (int8, uint64, uint64, error)
	StorageVolGetPath(libvirt.StorageVol) (string, error)
	StorageVolResize(libvirt.StorageVol, uint64, libvirt.StorageVolResizeFlags) error
}

type client struct {
	rpc        poolRPC
	conn       net.Conn
	cancel     context.CancelFunc
	stopClose  func() bool
	disconnect func() error
}

func (c *client) Close() {
	// Keep the session timer/cancellation callback armed while Disconnect waits
	// for its reply. On any error (or deadline) the transport is still closed.
	defer closeTransport(c.conn)
	defer c.cancel()
	defer c.stopClose()

	if c.disconnect != nil {
		if err := c.disconnect(); err != nil {
			return
		}
	}
}

func closeTransport(conn net.Conn) {
	// Cleanup is best-effort: a canceled or expired session is never reused,
	// and reporting a close failure cannot restore its outstanding RPCs.
	conn.Close() //nolint:errcheck
}

func (c *client) Pools() ([]Pool, error) {
	// Flags 0 lists every pool: an owned orphan may be active or inactive.
	pools, _, err := c.rpc.ConnectListAllStoragePools(1, 0)
	if err != nil {
		return nil, err
	}

	result := make([]Pool, 0, len(pools))

	for _, pool := range pools {
		text, err := c.rpc.StoragePoolGetXMLDesc(pool, 0)
		if err != nil {
			return nil, err
		}

		var desc libvirtxml.StoragePool
		if err = desc.Unmarshal(text); err != nil {
			return nil, err
		}

		persistent, err := c.rpc.StoragePoolIsPersistent(pool)
		if err != nil {
			return nil, err
		}

		autostart, err := c.rpc.StoragePoolGetAutostart(pool)
		if err != nil {
			return nil, err
		}

		active, err := c.rpc.StoragePoolIsActive(pool)
		if err != nil {
			return nil, err
		}

		observed := Pool{Name: pool.Name, UUID: uuid.UUID(pool.UUID), Type: desc.Type, Persistent: persistent != 0, Autostart: autostart != 0, Active: active != 0}
		if desc.Target != nil {
			observed.Target = desc.Target.Path
		}

		result = append(result, observed)
	}

	return result, nil
}

func (c *client) target(p libvirt.StoragePool) (string, error) {
	text, err := c.rpc.StoragePoolGetXMLDesc(p, 0)
	if err != nil {
		return "", err
	}

	var desc libvirtxml.StoragePool
	if err = desc.Unmarshal(text); err != nil {
		return "", err
	}

	if desc.Target == nil {
		return "", nil
	}

	return desc.Target.Path, nil
}

func (c *client) lookup(pool Pool) (libvirt.StoragePool, bool, error) {
	p, err := c.rpc.StoragePoolLookupByName(pool.Name)
	if err != nil {
		if rpcErr, ok := errors.AsType[libvirt.Error](err); ok && rpcErr.Code == uint32(libvirt.ErrNoStoragePool) {
			return libvirt.StoragePool{}, false, nil
		}

		return libvirt.StoragePool{}, false, volumeReadRPCError(err)
	}

	if p.UUID != libvirt.UUID(pool.UUID) {
		return libvirt.StoragePool{}, false, fmt.Errorf("pool %q is not owned by this machine", pool.Name)
	}

	return p, true, nil
}

// Ensure disables libvirt autostart: only Talos may activate after the mount
// hold exists. prepare must create only the pool directory, on a held mount.
func (c *client) Ensure(pool Pool, target string, prepare func() error) error {
	_, err := c.EnsureOperation(pool, target, prepare)

	return err
}

// EnsureOperation returns evidence for every mutating RPC in this invocation.
// A matching inactive pool is activated without destructive stop/redefine.
func (c *client) EnsureOperation(pool Pool, target string, prepare func() error) (OperationOutcome, error) {
	var evidence mutationEvidence

	p, exists, err := c.lookup(pool)
	if err != nil {
		return evidence.outcome, err
	}

	redefine := !exists
	if exists {
		redefine, err = c.prepareExisting(p, target, &evidence)
		if err != nil {
			return evidence.outcome, err
		}
	}

	if err = prepare(); err != nil {
		return evidence.outcome, err
	}

	if redefine {
		p, err = c.define(pool, target)
		evidence.record(err)

		if err != nil {
			return evidence.outcome, err
		}
	}

	if err = c.disableAutostart(p, &evidence); err != nil {
		return evidence.outcome, err
	}

	active, err := c.rpc.StoragePoolIsActive(p)
	if err != nil {
		return evidence.outcome, err
	}

	if active == 0 {
		err = c.rpc.StoragePoolCreate(p, 0)
		evidence.record(err)

		return evidence.outcome, err
	}

	return evidence.outcome, nil
}

func (c *client) define(pool Pool, target string) (libvirt.StoragePool, error) {
	desc := libvirtxml.StoragePool{
		Type:   "dir",
		Name:   pool.Name,
		UUID:   pool.UUID.String(),
		Target: &libvirtxml.StoragePoolTarget{Path: target},
	}

	text, err := desc.Marshal()
	if err != nil {
		return libvirt.StoragePool{}, err
	}

	return c.rpc.StoragePoolDefineXML(text, 0)
}

// prepareExisting disables daemon autostart and stops a pool before redefining it.
func (c *client) prepareExisting(p libvirt.StoragePool, target string, evidence *mutationEvidence) (bool, error) {
	text, err := c.rpc.StoragePoolGetXMLDesc(p, 0)
	if err != nil {
		return false, err
	}

	var desc libvirtxml.StoragePool
	if err = desc.Unmarshal(text); err != nil {
		return false, err
	}

	if desc.Type != "dir" {
		return false, fmt.Errorf("pool %q is not a directory pool", p.Name)
	}

	persistent, err := c.rpc.StoragePoolIsPersistent(p)
	if err != nil {
		return false, err
	}

	if persistent != 0 {
		if err = c.disableAutostart(p, evidence); err != nil {
			return false, err
		}
	}

	redefine := desc.Target == nil || desc.Target.Path != target || persistent == 0
	if redefine {
		return true, c.stop(p, evidence)
	}

	return false, nil
}

// disableAutostart records the reply only when a mutation was submitted.
func (c *client) disableAutostart(p libvirt.StoragePool, evidence *mutationEvidence) error {
	// An unchanged healthy pool needs no mutating RPC on each reconcile.
	autostart, err := c.rpc.StoragePoolGetAutostart(p)
	if err != nil {
		return err
	}

	if autostart == 0 {
		return nil
	}

	err = c.rpc.StoragePoolSetAutostart(p, 0)
	evidence.record(err)

	return err
}

func (c *client) stop(p libvirt.StoragePool, evidence *mutationEvidence) error {
	active, err := c.rpc.StoragePoolIsActive(p)
	if err != nil {
		return err
	}

	if active != 0 {
		err = c.rpc.StoragePoolDestroy(p)
		evidence.record(err)

		return err
	}

	return nil
}

// Stop leaves the persistent definition and all data intact.
func (c *client) Stop(pool Pool) error {
	_, err := c.StopOperation(pool)

	return err
}

// StopOperation returns the outcome of stopping an active pool, or no mutation for an absent/inactive pool.
func (c *client) StopOperation(pool Pool) (OperationOutcome, error) {
	var evidence mutationEvidence

	p, exists, err := c.lookup(pool)
	if err != nil || !exists {
		return evidence.outcome, err
	}

	err = c.stop(p, &evidence)

	return evidence.outcome, err
}

// Remove always stops and undefines, regardless of pool contents. It never
// deletes a storage pool's data, enumerates files or calls StoragePoolDelete.
func (c *client) Remove(pool Pool) error {
	_, err := c.RemoveOperation(pool)

	return err
}

// RemoveOperation reports completed partial progress separately from a lost reply.
func (c *client) RemoveOperation(pool Pool) (OperationOutcome, error) {
	var evidence mutationEvidence

	p, exists, err := c.lookup(pool)
	if err != nil || !exists {
		return evidence.outcome, err
	}

	if err = c.stop(p, &evidence); err != nil {
		return evidence.outcome, err
	}

	err = c.rpc.StoragePoolUndefine(p)
	evidence.record(err)

	return evidence.outcome, err
}
