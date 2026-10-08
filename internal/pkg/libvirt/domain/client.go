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
	// operationTimeout bounds connecting to the daemon.
	operationTimeout = 5 * time.Second
	// sessionTimeout bounds one bounded session as a whole. Longer than connecting because the work
	// inside it is not all cheap: libvirt waits on the guest to open a tray before it inserts a new
	// medium, which it gives up to ten seconds of back-off.
	sessionTimeout = 30 * time.Second
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

	sessionCtx, cancel := context.WithTimeout(ctx, sessionTimeout)

	return c.OpenConn(sessionCtx, conn, cancel)
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

// Info holds selected fields from libvirt DomainGetInfo. Memory is in KiB.
type Info struct {
	State        uint32
	MaxMemoryKiB uint64
	MemoryKiB    uint64
	VCPUs        uint32
}

// Client represents one bounded reconciliation session; callers must Close it.
type Client interface {
	Domains() ([]Domain, error)
	Active(Domain) (bool, error)
	Info(Domain) (Info, error)
	Start(Domain, string, ...StartOption) error
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

func collectStartOptions(opts ...StartOption) startOptions {
	var o startOptions

	for _, opt := range opts {
		opt(&o)
	}

	return o
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
	for _, check := range collectStartOptions(opts...).admissions {
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
	running := runningDisks(live)

	for i := range desired.desc.Devices.Disks {
		disk := &desired.desc.Devices.Disks[i]
		if !hotPluggableMedium(disk) {
			continue
		}

		name := disk.Alias.Name

		current, present := matchRunningDisk(running, disk)
		if !present {
			return fmt.Errorf("domain %q is not running the drive %q", found.Name, name)
		}

		if diskSourceFile(current) == diskSourceFile(disk) {
			continue
		}

		update, err := mediaChangeXML(current, disk)
		if err != nil {
			return err
		}

		// Live only: a transient domain has no persistent definition to also change. Not forced
		// either, so a guest which has locked its tray is reported rather than overruled.
		if err = c.rpc.DomainUpdateDeviceFlags(found, update, libvirt.DomainDeviceModifyLive); err != nil {
			return fmt.Errorf("failed to change the medium of drive %q of domain %q: %w", name, found.Name, err)
		}
	}

	// Written last. Until it lands the domain still carries the previous digests, and the next pass
	// finds every medium already where it wants it and does nothing but write this again.
	return c.setDefinitionMetadata(found, desired)
}

// runningDisks indexes the disks of a running domain under every identity they can be found by.
func runningDisks(live libvirtxml.Domain) map[string]*libvirtxml.DomainDisk {
	running := map[string]*libvirtxml.DomainDisk{}

	if live.Devices == nil {
		return running
	}

	for i := range live.Devices.Disks {
		disk := &live.Devices.Disks[i]

		for _, key := range deviceKeys(disk) {
			running[key] = disk
		}
	}

	return running
}

// matchRunningDisk finds the running disk a rendered one describes, by alias first.
func matchRunningDisk(running map[string]*libvirtxml.DomainDisk, disk *libvirtxml.DomainDisk) (*libvirtxml.DomainDisk, bool) {
	for _, key := range deviceKeys(disk) {
		if current, present := running[key]; present {
			return current, true
		}
	}

	return nil, false
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

// deviceKey identifies one disk across the rendered and the running definition.
//
// The alias is the identity libvirt was given and carries through, and unlike a target device it
// exists for devices that have no target at all. The target device is kept as a fallback for a
// definition whose alias did not survive.
func deviceKeys(disk *libvirtxml.DomainDisk) []string {
	var keys []string

	if disk.Alias != nil && disk.Alias.Name != "" {
		keys = append(keys, "alias:"+disk.Alias.Name)
	}

	if disk.Target != nil && disk.Target.Dev != "" {
		keys = append(keys, "dev:"+disk.Target.Dev)
	}

	return keys
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
// Digest covers the whole definition. Core covers the same definition with the medium of every
// hot-pluggable drive left out, so a desired definition which matches the running one in Core but
// not in Digest is one that differs only in media, and can be reached without restarting the guest.
// A domain started before Core existed simply carries none, and so is never changed in place.
type definitionMetadata struct {
	XMLName xml.Name `xml:"https://talos.dev/libvirt/domain definition"`
	Core    string   `xml:"core,attr,omitempty"`
	Digest  string   `xml:",chardata"`
}

// definition is one rendered domain XML, prepared for libvirt and digested.
type definition struct {
	// desc is the canonical description, before the ownership marker was added to it.
	desc *libvirtxml.Domain
	// text is what libvirt is handed, marker included.
	text string
	full string
	core string
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
	if desc.Devices != nil {
		for i := range desc.Devices.Serials {
			serial := &desc.Devices.Serials[i]
			if serial.Source != nil && serial.Source.Pty != nil {
				serial.Log = &libvirtxml.DomainChardevLog{
					File:   fmt.Sprintf("%s/vm-%s-serial%d.log", constants.LogMountPoint, d.UUID, i),
					Append: "on",
				}
			}
		}
	}

	// Both digests are taken before the marker is added, so a Talos which did not compute the core
	// digest yet produces the same full digest for the same definition: upgrading does not restart
	// running domains, it only leaves them unable to change media until they next restart anyway.
	canonical, err := desc.Marshal()
	if err != nil {
		return definition{}, fmt.Errorf("marshal domain XML: %w", err)
	}

	core, err := coreDigest(&desc)
	if err != nil {
		return definition{}, err
	}

	result := definition{
		desc: &desc,
		full: fmt.Sprintf("%x", sha256.Sum256([]byte(canonical))),
		core: core,
	}

	if result.text, err = markedDefinition(desc, result.full, result.core); err != nil {
		return definition{}, err
	}

	return result, nil
}

// markedDefinition is the definition libvirt is handed: the canonical description with the
// ownership marker added to its metadata.
//
// Marshaled from a copy, so the description the media path reads stays the canonical one the
// digests were taken over.
func markedDefinition(desc libvirtxml.Domain, full, core string) (string, error) {
	marker, err := definitionMarker(full, core)
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

// coreDigest hashes the definition with the medium of every hot-pluggable drive left out.
//
// Empty when there is nothing hot-pluggable in it, which reads downstream as "never change this one
// in place".
func coreDigest(desc *libvirtxml.Domain) (string, error) {
	if desc.Devices == nil {
		return "", nil
	}

	var masked []*libvirtxml.DomainDisk

	for i := range desc.Devices.Disks {
		if disk := &desc.Devices.Disks[i]; hotPluggableMedium(disk) {
			masked = append(masked, disk)
		}
	}

	if len(masked) == 0 {
		return "", nil
	}

	// Masked in place and put back, rather than hashing a reparse of the canonical text: the
	// stability of unmarshalling what libvirtxml marshaled is not something it promises, and a
	// digest which disagreed with itself would restart every guest on every pass.
	sources := make([]*libvirtxml.DomainDiskSource, len(masked))

	for i, disk := range masked {
		sources[i], disk.Source = disk.Source, nil
	}

	defer func() {
		for i, disk := range masked {
			disk.Source = sources[i]
		}
	}()

	canonical, err := desc.Marshal()
	if err != nil {
		return "", fmt.Errorf("marshal masked domain XML: %w", err)
	}

	return fmt.Sprintf("%x", sha256.Sum256([]byte(canonical))), nil
}

// definitionMarker renders the ownership marker. Never empty: libvirt takes an empty element as a
// request to delete the metadata, and the marker is what proves this machine owns the domain.
func definitionMarker(full, core string) (string, error) {
	if full == "" {
		return "", errors.New("refusing to write an empty ownership marker")
	}

	text, err := xml.Marshal(definitionMetadata{Core: core, Digest: full})
	if err != nil {
		return "", fmt.Errorf("marshal domain metadata: %w", err)
	}

	return string(text), nil
}

// setDefinitionMetadata rewrites the ownership marker of a running domain in place.
func (c *client) setDefinitionMetadata(found libvirt.Domain, desired definition) error {
	marker, err := definitionMarker(desired.full, desired.core)
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

	switch {
	case active == 0:
		return changeRestart, live, nil
	case current.Digest == desired.full:
		return changeNone, live, nil
	case desired.core != "" && current.Core == desired.core:
		return changeMedia, live, nil
	default:
		return changeRestart, live, nil
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
		return current, definitionMetadata{}, fmt.Errorf("domain %q has no Talos ownership metadata", found.Name)
	}

	var wrapper struct {
		Definition definitionMetadata `xml:"https://talos.dev/libvirt/domain definition"`
	}

	if err = xml.Unmarshal([]byte("<metadata>"+current.Metadata.XML+"</metadata>"), &wrapper); err != nil {
		return current, definitionMetadata{}, fmt.Errorf("invalid domain metadata: %w", err)
	}

	if wrapper.Definition.Digest == "" {
		return current, definitionMetadata{}, fmt.Errorf("domain %q has no Talos ownership metadata", found.Name)
	}

	return current, wrapper.Definition, nil
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
