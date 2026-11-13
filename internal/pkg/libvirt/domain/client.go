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
	"net/netip"
	"slices"
	"strings"
	"time"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	// operationTimeout bounds connecting to the daemon, and a bounded session for as long as it
	// does nothing slower than an ordinary RPC.
	operationTimeout = 5 * time.Second
	// mediaChangeTimeout bounds one medium change. Longer than an ordinary RPC because libvirt waits
	// on the guest to open a tray before it inserts a new medium, which it gives up to ten seconds of
	// back-off. Granted per change, so several of them in one session do not starve each other or
	// the work which follows.
	mediaChangeTimeout = 15 * time.Second
)

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
	dialCtx, dialCancel := context.WithTimeout(ctx, operationTimeout)
	defer dialCancel()

	conn, err := (&net.Dialer{}).DialContext(dialCtx, "unix", c.socket)
	if err != nil {
		return nil, err
	}

	stopDialClose := context.AfterFunc(dialCtx, func() { closeTransport(conn) })
	defer stopDialClose()

	// A watchdog rather than a fixed deadline, so the session can be granted more time for a slow
	// operation without every other session paying for it: a hung daemon still ends it after
	// operationTimeout.
	sessionCtx, cancel := context.WithCancel(ctx)
	watchdog := time.AfterFunc(operationTimeout, cancel)

	client, err := c.openConn(sessionCtx, conn, func() {
		watchdog.Stop()
		cancel()
	})
	if err != nil {
		return nil, err
	}

	client.watchdog = watchdog

	return client, nil
}

// OpenPersistent opens one inventory session for an event-driven controller.
// Only connection establishment is time-bounded; the caller closes the session.
func (c *Connector) OpenPersistent(ctx context.Context) (Client, error) {
	startupCtx, startupCancel := context.WithTimeout(ctx, operationTimeout)
	defer startupCancel()

	conn, err := (&net.Dialer{}).DialContext(startupCtx, "unix", c.socket)
	if err != nil {
		return nil, err
	}

	stopStartupClose := context.AfterFunc(startupCtx, func() { closeTransport(conn) })
	defer stopStartupClose()

	sessionCtx, cancel := context.WithCancel(ctx)

	return c.OpenConn(sessionCtx, conn, cancel)
}

// OpenConn connects over an already-connected transport for this connector.
// It takes ownership of conn and cancel; ctx must bound the full session.
func (c *Connector) OpenConn(ctx context.Context, conn net.Conn, cancel context.CancelFunc) (Client, error) {
	client, err := c.openConn(ctx, conn, cancel)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Connector) openConn(ctx context.Context, conn net.Conn, cancel context.CancelFunc) (*client, error) {
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

// ParseMachineUUID validates the machine UUID every domain of a host is named under, as published
// by SystemInformation. A nil UUID is refused: it would name every host's domains alike.
func ParseMachineUUID(machine string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(machine)
	if err != nil || parsed == uuid.Nil {
		return uuid.Nil, fmt.Errorf("invalid machine UUID %q", machine)
	}

	return parsed, nil
}

// Domain identifies a domain by name and UUID; it does not prove ownership.
type Domain struct {
	Name string
	UUID uuid.UUID
}

// Info holds selected fields from libvirt DomainGetInfo. Memory is in KiB.
type Info struct {
	State        uint32
	MaxMemoryKiB uint64
	MemoryKiB    uint64
	VCPUs        uint32
}

// GuestInterface is one network interface as reported by the qemu-guest-agent.
type GuestInterface struct {
	Name         string
	HardwareAddr string
	IPs          []netip.Prefix
}

// Client represents one bounded reconciliation session; callers must Close it.
type Client interface {
	Domains() ([]Domain, error)
	Active(Domain) (bool, error)
	Info(Domain) (Info, error)
	GuestInterfaces(Domain) ([]GuestInterface, error)
	Definition(Domain) (Definition, error)
	Start(Domain, string, ...StartOption) error
	Shutdown(Domain) error
	Reboot(Domain) error
	Remove(Domain) error
	Close()
}

type domainRPC interface {
	listRPC
	definitionRPC
	hotplugRPC
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

// hotplugRPC changes a running domain in place, rather than replacing its definition.
type hotplugRPC interface {
	DomainUpdateDeviceFlags(libvirt.Domain, string, libvirt.DomainDeviceModifyFlags) error
	DomainSetMetadata(libvirt.Domain, int32, libvirt.OptString, libvirt.OptString, libvirt.OptString, libvirt.DomainModificationImpact) error
}

type lifecycleRPC interface {
	DomainIsActive(libvirt.Domain) (int32, error)
	DomainGetInfo(libvirt.Domain) (uint8, uint64, uint64, uint16, uint64, error)
	DomainInterfaceAddresses(libvirt.Domain, uint32, uint32) ([]libvirt.DomainInterface, error)
	DomainDestroy(libvirt.Domain) error
	DomainShutdownFlags(libvirt.Domain, libvirt.DomainShutdownFlagValues) error
	DomainReboot(libvirt.Domain, libvirt.DomainRebootFlagValues) error
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
	// watchdog ends a bounded session which has run out of time, by canceling it. Nil for a
	// persistent session, which has no such bound.
	watchdog *time.Timer
}

// extend gives a bounded session d from now before it is ended. A persistent session is left alone.
func (c *client) extend(d time.Duration) {
	if c.watchdog != nil {
		c.watchdog.Reset(d)
	}
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

func (c *client) Active(domain Domain) (bool, error) {
	found, exists, err := c.lookup(domain)
	if err != nil || !exists {
		return false, err
	}

	active, err := c.rpc.DomainIsActive(found)
	if err != nil {
		return false, err
	}

	return active == 1, nil
}

// Info reads raw libvirt domain state and counters for any domain in the inventory.
func (c *client) Info(domain Domain) (Info, error) {
	found, exists, err := c.lookup(domain)
	if err != nil {
		return Info{}, err
	}

	if !exists {
		return Info{}, fmt.Errorf("domain %q disappeared before reading info", domain.Name)
	}

	state, maxMemory, memory, vcpus, _, err := c.rpc.DomainGetInfo(found)
	if err != nil {
		return Info{}, err
	}

	return Info{
		State:        uint32(state),
		MaxMemoryKiB: maxMemory,
		MemoryKiB:    memory,
		VCPUs:        uint32(vcpus),
	}, nil
}

// GuestInterfaces reports interfaces as seen by the qemu-guest-agent running inside the guest.
func (c *client) GuestInterfaces(domain Domain) ([]GuestInterface, error) {
	found, exists, err := c.lookup(domain)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("domain %q disappeared before reading guest interfaces", domain.Name)
	}

	raw, err := c.rpc.DomainInterfaceAddresses(found, uint32(libvirt.DomainInterfaceAddressesSrcAgent), 0)
	if err != nil {
		return nil, err
	}

	ifaces := make([]GuestInterface, 0, len(raw))

	for _, iface := range raw {
		converted := GuestInterface{
			Name: iface.Name,
		}

		if len(iface.Hwaddr) > 0 {
			converted.HardwareAddr = iface.Hwaddr[0]
		}

		for _, addr := range iface.Addrs {
			ip, err := netip.ParseAddr(addr.Addr)
			if err != nil {
				return nil, fmt.Errorf("parse guest interface %q address: %w", iface.Name, err)
			}

			prefix := netip.PrefixFrom(ip, int(addr.Prefix))
			if !prefix.IsValid() {
				return nil, fmt.Errorf("invalid guest interface %q prefix %s/%d", iface.Name, ip, addr.Prefix)
			}

			converted.IPs = append(converted.IPs, prefix)
		}

		ifaces = append(ifaces, converted)
	}

	return ifaces, nil
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

type startOptions struct {
	admissions []func() error
}

// StartOption configures one Start call.
type StartOption func(*startOptions)

// WithStartAdmission adds a check Start runs only when it is about to create or
// replace the domain: after an unchanged active domain has been left alone, and
// before an existing domain is removed. An error aborts Start and is returned
// as is, leaving any existing domain untouched.
func WithStartAdmission(check func() error) StartOption {
	return func(o *startOptions) {
		o.admissions = append(o.admissions, check)
	}
}

// Admit runs the admission checks of opts in order, stopping at the first error.
// Client implementations call it at the point Start would create or replace a domain.
func Admit(opts ...StartOption) error {
	var o startOptions

	for _, opt := range opts {
		opt(&o)
	}

	for _, check := range o.admissions {
		if err := check(); err != nil {
			return err
		}
	}

	return nil
}

// Start creates an active transient domain, changes the medium of a running one when that is all
// the new definition asks for, or else replaces it. Replacing a running domain interrupts the
// guest; changing a medium does not.
func (c *client) Start(d Domain, renderedXML string, opts ...StartOption) error {
	if err := validateDomain(d); err != nil {
		return err
	}

	desired, err := domainDefinition(d, renderedXML)
	if err != nil {
		return err
	}

	found, exists, err := c.lookup(d)
	if err != nil {
		return err
	}

	plan, live, err := c.planChange(found, exists, desired)
	if err != nil {
		return err
	}

	switch plan {
	case changeNone:
		return nil
	case changeMedia:
		// Deliberately not admitted: an admission check guards what a new domain would take from
		// the host, and a medium changing in a drive the running domain already has takes nothing.
		// Calling Admit here would stop guests over inventory that has not moved.
		if err = c.changeMedia(found, live, desired); err != nil {
			return fmt.Errorf("%w: %w", ErrMediaChange, err)
		}

		return nil
	case changeRestart:
	}

	return c.replace(d, desired, exists, opts...)
}

// replace defines the domain, taking any existing one down first. A guest in it is interrupted.
func (c *client) replace(d Domain, desired definition, exists bool, opts ...StartOption) error {
	if err := Admit(opts...); err != nil {
		return err
	}

	if exists {
		if err := c.Remove(d); err != nil {
			return err
		}
	}

	return c.createTransient(d, desired.text)
}

// ErrMediaChange marks a medium that could not be changed on the running domain, most often a guest
// which has locked its tray. The guest is left as it is and the change is retried, so this is a
// condition to wait on rather than a reason to stop anything.
var ErrMediaChange = errors.New("failed to change the medium of a running domain")

// changeMedia loads, swaps or ejects the medium of every hot-pluggable drive whose source moved.
func (c *client) changeMedia(found libvirt.Domain, live libvirtxml.Domain, desired definition) error {
	changes, err := pendingMediaChanges(live, desired.desc)
	if err != nil {
		return fmt.Errorf("domain %q: %w", found.Name, err)
	}

	for _, change := range changes {
		name := change.desired.Alias.Name

		update, err := mediaChangeXML(change.current, change.desired)
		if err != nil {
			return err
		}

		// Live only: a transient domain has no persistent definition to also change. Not forced
		// either, so a guest which has locked its tray is reported rather than overruled.
		c.extend(mediaChangeTimeout)
		err = c.rpc.DomainUpdateDeviceFlags(found, update, libvirt.DomainDeviceModifyLive)
		c.extend(operationTimeout)

		if err != nil {
			return fmt.Errorf("failed to change the medium of drive %q of domain %q: %w", name, found.Name, err)
		}
	}

	// Written last. Until it lands the domain still carries the previous digests, and the next pass
	// finds every medium already where it wants it and does nothing but write this again.
	return c.setDefinitionMetadata(found, desired)
}

// mediumChange is one hot-pluggable drive whose running medium is not the desired one.
type mediumChange struct {
	current *libvirtxml.DomainDisk
	desired *libvirtxml.DomainDisk
}

// pendingMediaChanges lists the hot-pluggable drives of the desired description whose medium in the
// running domain differs from the one the description puts in them.
//
// Drives are matched by alias: a hot-pluggable drive always carries one DeviceAlias produced, and
// libvirt keeps it on the running domain. A drive the running domain does not have is an error, as
// no medium change can reach it.
func pendingMediaChanges(live libvirtxml.Domain, desired *libvirtxml.Domain) ([]mediumChange, error) {
	if desired == nil || desired.Devices == nil {
		return nil, nil
	}

	running := map[string]*libvirtxml.DomainDisk{}

	if live.Devices != nil {
		for i := range live.Devices.Disks {
			if disk := &live.Devices.Disks[i]; configOwned(disk.Alias) {
				running[disk.Alias.Name] = disk
			}
		}
	}

	var changes []mediumChange

	for i := range desired.Devices.Disks {
		disk := &desired.Devices.Disks[i]
		if !hotPluggableMedium(disk) {
			continue
		}

		current, present := running[disk.Alias.Name]
		if !present {
			return nil, fmt.Errorf("the running domain has no drive %q", disk.Alias.Name)
		}

		if diskSourceFile(current) != diskSourceFile(disk) {
			changes = append(changes, mediumChange{current: current, desired: disk})
		}
	}

	return changes, nil
}

// mediaChangeXML is the running device with a new medium in it.
//
// Built from the running device rather than the rendered one because libvirt refuses an update
// whose element differs from what is running in any of the many fields the caller did not mean to
// change -- including the open tray an eject leaves behind, which no freshly rendered element
// carries. The core digest has already established that the two agree about everything but the
// medium, so taking the rest from the running device hides no difference.
func mediaChangeXML(current, desired *libvirtxml.DomainDisk) (string, error) {
	update := *current
	update.Source = desired.Source
	update.Address = nil
	update.BackingStore = nil
	update.Mirror = nil

	// libvirt accepts the alias it was given and rejects the one it assigned itself, so keep ours
	// -- which is also how it recognizes the device -- and drop anything else.
	if !configOwned(update.Alias) {
		update.Alias = nil
	}

	text, err := xml.Marshal(&update)
	if err != nil {
		return "", fmt.Errorf("marshal disk update: %w", err)
	}

	return string(text), nil
}

// normalizeEmptySources drops the source of every drive which has no medium in it.
//
// Reading XML back gives a drive with no source element a source carrying no file, because the
// element is optional but its type is not. Writing that out again produces an empty source element
// rather than none, so canonicalize it away: one spelling of an empty drive keeps the digests, the
// definition libvirt is handed and the update elements all saying the same thing.
func normalizeEmptySources(desc *libvirtxml.Domain) {
	if desc.Devices == nil {
		return
	}

	for i := range desc.Devices.Disks {
		disk := &desc.Devices.Disks[i]
		if disk.Source != nil && disk.Source.File != nil && disk.Source.File.File == "" {
			disk.Source = nil
		}
	}
}

// diskSourceFile is the host file a drive reads, or empty when there is no medium in it.
func diskSourceFile(disk *libvirtxml.DomainDisk) string {
	if disk.Source == nil || disk.Source.File == nil {
		return ""
	}

	return disk.Source.File.File
}

// aliasPrefix marks a device the machine configuration declares, as opposed to one the renderer
// adds on its own account. libvirt reserves the "ua-" prefix for aliases its callers set, and
// carries them through to the running domain, so one is both a mark and a stable device identity.
const aliasPrefix = "ua-talos-"

// DeviceAlias names a device the machine configuration declares. Renderers set it; it is what tells
// this package which devices a definition change is allowed to be applied to in place.
//
// kind and name must be the character set libvirt allows an alias, which is letters, digits,
// hyphens and underscores.
func DeviceAlias(kind, name string) string {
	return aliasPrefix + kind + "-" + name
}

// configOwned reports whether an alias is one DeviceAlias produced.
func configOwned(alias *libvirtxml.DomainAlias) bool {
	return alias != nil && strings.HasPrefix(alias.Name, aliasPrefix)
}

// hotPluggableMedium reports whether a drive's medium may be changed in place.
//
// Two conditions, and both are load-bearing. The drive has to be one the machine configuration
// declares, because a drive the renderer added for its own purposes -- a cloud-init seed -- is
// boot-time intent, and swapping it under a running guest would not be applying it. And it has to
// be a cdrom, because libvirt changes the source of removable drives only, so treating a fixed disk
// as one would leave a definition change unapplied rather than apply it.
func hotPluggableMedium(disk *libvirtxml.DomainDisk) bool {
	return configOwned(disk.Alias) && disk.Device == "cdrom"
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

const (
	metadataNamespace = "https://talos.dev/libvirt/domain"
	metadataPrefix    = "talos"
)

// definitionMetadata is the marker Talos writes into a domain it owns.
//
// Digest covers the whole definition. Configuration covers it with the fields a hardware profile
// controls left out, so a profile-only edit can wait for the next stop/start. Core covers the same
// definition with the medium of every hot-pluggable drive left out, so a desired definition which
// matches the running one in Core but not in Digest is one that differs only in media, and can be
// reached without restarting the guest. A definition with no hot-pluggable drive carries no Core,
// and so is never changed in place.
type definitionMetadata struct {
	XMLName       xml.Name `xml:"https://talos.dev/libvirt/domain definition"`
	Configuration string   `xml:"configuration,attr,omitempty"`
	Core          string   `xml:"core,attr,omitempty"`
	Digest        string   `xml:",chardata"`
}

// definition is one rendered domain XML, prepared for libvirt and digested.
type definition struct {
	// desc is the canonical description, before the ownership marker was added to it.
	desc *libvirtxml.Domain
	// text is what libvirt is handed, marker included.
	text          string
	full          string
	configuration string
	core          string
}

// changePlan is how a desired definition can be reached from what is running.
type changePlan int

const (
	// changeNone is a running domain which already matches.
	changeNone changePlan = iota
	// changeMedia is a running domain which differs only in the media of hot-pluggable drives.
	changeMedia
	// changeRestart is a domain which has to be replaced, interrupting any guest in it.
	changeRestart
)

func domainDefinition(d Domain, renderedXML string) (definition, error) {
	var desc libvirtxml.Domain

	if err := desc.Unmarshal(renderedXML); err != nil {
		return definition{}, fmt.Errorf("invalid domain XML: %w", err)
	}

	if desc.Name != d.Name {
		return definition{}, fmt.Errorf("domain XML name %q does not match %q", desc.Name, d.Name)
	}

	desc.UUID = d.UUID.String()

	normalizeEmptySources(&desc)

	// Serial capture belongs to the owned backend definition, where the host-specific
	// identity is known. Keep the PTY for exclusive live attachment and let virtlogd
	// bound retention. Never accept a log path from the rendered input.
	setSerialCapture(&desc, d.UUID)

	// All digests are taken before the marker is added.
	full, configuration, err := definitionDigests(desc)
	if err != nil {
		return definition{}, err
	}

	core, err := coreDigest(desc)
	if err != nil {
		return definition{}, err
	}

	result := definition{
		desc:          &desc,
		full:          full,
		configuration: configuration,
		core:          core,
	}

	if result.text, err = markedDefinition(desc, result); err != nil {
		return definition{}, err
	}

	return result, nil
}

// markedDefinition is the definition libvirt is handed: the canonical description with the
// ownership marker added to its metadata.
//
// Marshaled from a copy, so the description the media path reads stays the canonical one the
// digests were taken over.
func markedDefinition(desc libvirtxml.Domain, digests definition) (string, error) {
	marker, err := definitionMarker(digests)
	if err != nil {
		return "", err
	}

	if desc.Metadata == nil {
		desc.Metadata = &libvirtxml.DomainMetadata{}
	} else {
		metadata := *desc.Metadata
		desc.Metadata = &metadata
	}

	desc.Metadata.XML += marker

	text, err := desc.Marshal()
	if err != nil {
		return "", fmt.Errorf("marshal domain metadata: %w", err)
	}

	return text, nil
}

func setSerialCapture(desc *libvirtxml.Domain, id uuid.UUID) {
	if desc.Devices == nil {
		return
	}

	for i := range desc.Devices.Serials {
		serial := &desc.Devices.Serials[i]
		if serial.Source != nil && serial.Source.Pty != nil {
			serial.Log = &libvirtxml.DomainChardevLog{
				File:   fmt.Sprintf("%s/vm-%s-serial%d.log", constants.LogMountPoint, id, i),
				Append: "on",
			}
		}
	}
}

// definitionDigests reports the full and the profile-independent digest of a description.
func definitionDigests(desc libvirtxml.Domain) (full, configuration string, err error) {
	canonical, err := desc.Marshal()
	if err != nil {
		return "", "", fmt.Errorf("marshal domain XML: %w", err)
	}

	configuration, err = profileIndependentDigest(desc)
	if err != nil {
		return "", "", err
	}

	return fmt.Sprintf("%x", sha256.Sum256([]byte(canonical))), configuration, nil
}

// withoutProfile is a copy of desc with the fields a hardware profile controls left out.
//
// Only OS and CPU are copied; everything else, devices included, is shared with desc.
func withoutProfile(desc libvirtxml.Domain) libvirtxml.Domain {
	if desc.OS != nil && desc.OS.Type != nil {
		os := *desc.OS
		kind := *os.Type
		kind.Machine = ""
		os.Type = &kind
		desc.OS = &os
	}

	if desc.CPU != nil {
		cpu := *desc.CPU
		cpu.Mode = ""
		cpu.Match = ""
		cpu.Check = ""
		cpu.Migratable = ""
		cpu.Model = nil
		cpu.Features = nil
		desc.CPU = &cpu
	}

	return desc
}

// profileIndependentDigest excludes only fields controlled by a hardware profile.
// Topology, vCPU count, tuning, memory, devices and all other VM intent remain in
// this digest so a profile edit cannot hide an unrelated definition change.
func profileIndependentDigest(desc libvirtxml.Domain) (string, error) {
	masked := withoutProfile(desc)

	canonical, err := masked.Marshal()
	if err != nil {
		return "", fmt.Errorf("marshal profile-independent domain XML: %w", err)
	}

	return fmt.Sprintf("%x", sha256.Sum256([]byte(canonical))), nil
}

// coreDigest hashes the definition with the medium of every hot-pluggable drive and the fields a
// hardware profile controls left out.
//
// The profile fields are left out for the same reason the configuration digest leaves them out: a
// profile edit waits for the next stop/start, and a medium changed while one is waiting must still
// be changed in place rather than restart the guest early.
//
// Empty when there is nothing hot-pluggable in it, which reads downstream as "never change this one
// in place".
func coreDigest(desc libvirtxml.Domain) (string, error) {
	if desc.Devices == nil {
		return "", nil
	}

	// Masked on a copy of the disks rather than a reparse of the canonical text: the stability of
	// unmarshalling what libvirtxml marshaled is not something it promises, and a digest which
	// disagreed with itself would restart every guest on every pass.
	devices := *desc.Devices
	devices.Disks = slices.Clone(devices.Disks)
	masked := false

	for i := range devices.Disks {
		if hotPluggableMedium(&devices.Disks[i]) {
			devices.Disks[i].Source = nil
			masked = true
		}
	}

	if !masked {
		return "", nil
	}

	desc.Devices = &devices
	core := withoutProfile(desc)

	canonical, err := core.Marshal()
	if err != nil {
		return "", fmt.Errorf("marshal masked domain XML: %w", err)
	}

	return fmt.Sprintf("%x", sha256.Sum256([]byte(canonical))), nil
}

// definitionMarker renders the ownership marker. Never empty: libvirt takes an empty element as a
// request to delete the metadata, and the marker is what proves this machine owns the domain.
func definitionMarker(digests definition) (string, error) {
	if digests.full == "" {
		return "", errors.New("refusing to write an empty ownership marker")
	}

	text, err := xml.Marshal(definitionMetadata{
		Configuration: digests.configuration,
		Core:          digests.core,
		Digest:        digests.full,
	})
	if err != nil {
		return "", fmt.Errorf("marshal domain metadata: %w", err)
	}

	return string(text), nil
}

// setDefinitionMetadata rewrites the ownership marker of a running domain in place.
func (c *client) setDefinitionMetadata(found libvirt.Domain, desired definition) error {
	marker, err := definitionMarker(desired)
	if err != nil {
		return err
	}

	// Matched by namespace, so this replaces the marker and leaves any other metadata alone. Live
	// only: a transient domain has no persistent definition to write to, and asking for one fails.
	if err = c.rpc.DomainSetMetadata(found, int32(libvirt.DomainMetadataElement),
		libvirt.OptString{marker}, libvirt.OptString{metadataPrefix}, libvirt.OptString{metadataNamespace},
		libvirt.DomainAffectLive,
	); err != nil {
		return fmt.Errorf("failed to record the definition of domain %q: %w", found.Name, err)
	}

	return nil
}

// planChange decides how the running domain reaches the desired definition, and reports the running
// description it read on the way so the media path does not have to read it again.
func (c *client) planChange(found libvirt.Domain, exists bool, desired definition) (changePlan, libvirtxml.Domain, error) {
	if !exists {
		return changeRestart, libvirtxml.Domain{}, nil
	}

	live, current, err := c.liveDefinition(found)
	if err != nil {
		return changeRestart, live, err
	}

	persistent, err := c.rpc.DomainIsPersistent(found)
	if err != nil {
		return changeRestart, live, err
	}

	if persistent != 0 {
		return changeRestart, live, nil
	}

	active, err := c.rpc.DomainIsActive(found)
	if err != nil {
		return changeRestart, live, err
	}

	if active == 0 {
		return changeRestart, live, nil
	}

	// The marker is written only once every medium has been changed, so a change which failed part
	// way leaves it naming the previous definition while some drives already hold the new media.
	// A marker which matches is therefore believed only once the running drives agree with it.
	changes, err := pendingMediaChanges(live, desired.desc)
	if err != nil {
		// A configured drive is missing from the running domain, detached out of band: no medium
		// change can reach it, so retrying one would never converge. Replace the domain instead.
		return changeRestart, live, nil //nolint:nilerr
	}

	return decideChange(current, desired, len(changes) == 0), live, nil
}

// decideChange compares the ownership marker of an active domain with the desired definition.
// inSync reports whether the media in the running drives agree with the desired ones.
func decideChange(current definitionMetadata, desired definition, inSync bool) changePlan {
	switch {
	case inSync && current.recorded().Runs(desired.recorded()):
		return changeNone
	case desired.core != "" && current.Core == desired.core:
		return changeMedia
	default:
		return changeRestart
	}
}

// liveDefinition reads a running domain and the ownership marker Talos left in it.
func (c *client) liveDefinition(found libvirt.Domain) (libvirtxml.Domain, definitionMetadata, error) {
	var current libvirtxml.Domain

	currentXML, err := c.rpc.DomainGetXMLDesc(found, 0)
	if err != nil {
		return current, definitionMetadata{}, err
	}

	if err = current.Unmarshal(currentXML); err != nil {
		return current, definitionMetadata{}, err
	}

	if current.Metadata == nil {
		return current, definitionMetadata{}, fmt.Errorf("domain %q %w", found.Name, errNotOwned)
	}

	var wrapper struct {
		Definition definitionMetadata `xml:"https://talos.dev/libvirt/domain definition"`
	}

	if err = xml.Unmarshal([]byte("<metadata>"+current.Metadata.XML+"</metadata>"), &wrapper); err != nil {
		return current, definitionMetadata{}, fmt.Errorf("invalid domain metadata: %w", err)
	}

	if wrapper.Definition.Digest == "" {
		return current, definitionMetadata{}, fmt.Errorf("domain %q %w", found.Name, errNotOwned)
	}

	return current, wrapper.Definition, nil
}

// errNotOwned marks a domain which carries no Talos ownership marker.
var errNotOwned = errors.New("has no Talos ownership metadata")

// Definition is what a domain records, in its ownership marker, about the definition it runs.
type Definition struct {
	// Digest covers the whole definition.
	Digest string
	// Configuration covers it with the fields a hardware profile controls left out.
	Configuration string
}

// recorded is the part of the marker which says what the domain runs.
func (m definitionMetadata) recorded() Definition {
	return Definition{Digest: m.Digest, Configuration: m.Configuration}
}

// Runs reports whether a domain recording this already runs desired, as Start judges it: the whole
// of it, or all of it but a hardware profile edit, which waits for the next stop and start. Both
// digests are stored by libvirt, so this survives a machined reconnect without pretending libvirt's
// expanded live CPU XML is identical to the requested profile.
//
// The marker is written only once a medium change has gone through in full, so a domain still
// waiting on a guest to release its tray does not run the definition which asked for it.
func (r Definition) Runs(desired Definition) bool {
	return r.Digest == desired.Digest || (r.Configuration != "" && r.Configuration == desired.Configuration)
}

// DefinitionOf is what a domain started on renderedXML records about the definition it runs.
func DefinitionOf(d Domain, renderedXML string) (Definition, error) {
	desired, err := domainDefinition(d, renderedXML)
	if err != nil {
		return Definition{}, err
	}

	return desired.recorded(), nil
}

// recorded is what a domain started on this definition records.
func (d definition) recorded() Definition {
	return Definition{Digest: d.full, Configuration: d.configuration}
}

// Definition reads what a domain records about the definition it runs. A domain which carries no
// ownership marker records nothing, and reports the zero Definition.
func (c *client) Definition(domain Domain) (Definition, error) {
	found, exists, err := c.lookup(domain)
	if err != nil {
		return Definition{}, err
	}

	if !exists {
		return Definition{}, fmt.Errorf("domain %q disappeared before reading its definition", domain.Name)
	}

	_, current, err := c.liveDefinition(found)
	if errors.Is(err, errNotOwned) {
		return Definition{}, nil
	}

	if err != nil {
		return Definition{}, err
	}

	return current.recorded(), nil
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

// owned looks a claimed domain up and proves Talos defined it.
//
// It reports exists=false both when there is no such domain and when the domain went away between
// the inventory which named it and this call.
func (c *client) owned(d Domain) (libvirt.Domain, bool, error) {
	if err := validateDomain(d); err != nil {
		return libvirt.Domain{}, false, err
	}

	found, exists, err := c.lookup(d)
	if err != nil || !exists {
		return libvirt.Domain{}, false, err
	}

	if _, _, err = c.liveDefinition(found); err != nil {
		return libvirt.Domain{}, false, err
	}

	return found, true, nil
}

// hasCode reports whether err is a libvirt error with one of codes.
func hasCode(err error, codes ...libvirt.ErrorNumber) bool {
	rpcErr, ok := errors.AsType[libvirt.Error](err)

	return ok && slices.Contains(codes, libvirt.ErrorNumber(rpcErr.Code))
}

// Shutdown asks the guest of a claimed domain to power itself off by pressing the virtual ACPI
// power button.
//
// The request is delivered, not obeyed: a guest which ignores the button keeps running, and
// nothing here waits for or enforces an outcome. The domain is transient, so a guest which does
// power off takes its domain with it.
//
// A domain which is gone needs nothing pressed. One libvirt refuses the press for, such as a paused
// domain, reports ErrDomainNotRunning: nothing was delivered.
func (c *client) Shutdown(d Domain) error {
	found, exists, err := c.owned(d)
	if err != nil || !exists {
		return err
	}

	err = c.rpc.DomainShutdownFlags(found, libvirt.DomainShutdownAcpiPowerBtn)

	switch {
	case err == nil, hasCode(err, libvirt.ErrNoDomain):
		return nil
	case hasCode(err, libvirt.ErrOperationInvalid):
		return fmt.Errorf("%w: %w", ErrDomainNotRunning, err)
	default:
		return err
	}
}

// ErrDomainNotRunning says a domain exists but is not running, so a guest cannot be asked anything.
var ErrDomainNotRunning = errors.New("domain is not running")

// Reboot asks the guest of a claimed domain to restart itself through its virtual ACPI power
// button. As with Shutdown, the request is delivered rather than obeyed.
//
// The domain object, its definition and its ownership metadata all survive a reboot, so nothing
// downstream sees the definition change: that is libvirt's default
// <on_reboot>restart</on_reboot>, which nothing in the rendered definition overrides. Were it
// destroy instead, a reboot would read downstream as the domain going away, and the controller
// would cold-boot it from the definition which is live rather than leave it alone.
func (c *client) Reboot(d Domain) error {
	found, exists, err := c.owned(d)
	if err != nil {
		return err
	}

	if !exists {
		return ErrDomainNotRunning
	}

	active, err := c.rpc.DomainIsActive(found)
	if err != nil {
		return err
	}

	if active == 0 {
		return ErrDomainNotRunning
	}

	if err = c.rpc.DomainReboot(found, libvirt.DomainRebootAcpiPowerBtn); err != nil {
		if hasCode(err, libvirt.ErrNoDomain, libvirt.ErrOperationInvalid) {
			return ErrDomainNotRunning
		}

		return err
	}

	return nil
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

	if _, _, err = c.liveDefinition(found); err != nil {
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
