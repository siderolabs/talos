// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain_test

import (
	"context"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"libvirt.org/go/libvirtxml"

	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
)

const machineUUID = "6b5baa32-47dd-4bcd-9c25-92870e3d43c9"

func TestUUID(t *testing.T) {
	t.Parallel()

	machine := uuid.MustParse(machineUUID)
	id := libvirtdomain.UUID(machine, "first")
	require.Equal(t, uuid.Version(8), id.Version())
	require.Equal(t, id, libvirtdomain.UUID(machine, "first"))
	require.NotEqual(t, id, libvirtdomain.UUID(machine, "second"))
	require.NotEqual(t, id, libvirtdomain.UUID(uuid.New(), "first"))
}

func encodeDomain(d libvirtdomain.Domain) []byte {
	payload := binary.BigEndian.AppendUint32(nil, uint32(len(d.Name)))
	payload = append(payload, d.Name...)
	payload = append(payload, make([]byte, (4-len(d.Name)%4)%4)...)
	payload = append(payload, d.UUID[:]...)

	return binary.BigEndian.AppendUint32(payload, 1) // active libvirt ID
}

func TestDomainsReturnsCompleteInventory(t *testing.T) {
	t.Parallel()

	first := libvirtdomain.Domain{Name: "first", UUID: uuid.New()}
	second := libvirtdomain.Domain{Name: "second", UUID: uuid.New()}

	raw, server := net.Pipe()
	defer server.Close() //nolint:errcheck

	served := make(chan error, 1)

	go func() {
		if err := serveHandshake(server); err != nil {
			served <- err

			return
		}

		call, err := readCall(server)
		if err != nil {
			served <- err

			return
		}

		if procedure := binary.BigEndian.Uint32(call[8:12]); procedure != 273 {
			served <- fmt.Errorf("expected CONNECT_LIST_ALL_DOMAINS, got %d", procedure)

			return
		}

		if needResults := binary.BigEndian.Uint32(call[24:28]); needResults != 1 {
			served <- fmt.Errorf("need_results = %d, want 1", needResults)

			return
		}

		payload := binary.BigEndian.AppendUint32(nil, 2)
		payload = append(payload, encodeDomain(first)...)
		payload = append(payload, encodeDomain(second)...)

		payload = binary.BigEndian.AppendUint32(payload, 2)
		if err = replyCall(server, call, payload); err != nil {
			served <- err

			return
		}

		call, err = readCall(server)
		if err == nil && binary.BigEndian.Uint32(call[8:12]) != 2 {
			err = fmt.Errorf("expected CONNECT_CLOSE, got %d", binary.BigEndian.Uint32(call[8:12]))
		}

		if err == nil {
			err = replyCall(server, call, nil)
		}

		served <- err
	}()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	client, err := libvirtdomain.New("", "qemu:///system").OpenConn(ctx, raw, cancel)
	require.NoError(t, err)

	domains, err := client.Domains()
	require.NoError(t, err)
	require.Equal(t, []libvirtdomain.Domain{first, second}, domains)
	client.Close()
	require.NoError(t, <-served)
}

type domainRecord struct {
	identity    libvirtdomain.Domain
	xml         string
	lookupError libvirt.ErrorNumber
	inactive    bool
}

type domainWireFixture struct {
	records map[string]domainRecord
	calls   []uint32
}

func decodeString(payload []byte) string {
	length := binary.BigEndian.Uint32(payload[:4])

	return string(payload[4 : 4+length])
}

func encodeString(value string) []byte {
	payload := binary.BigEndian.AppendUint32(nil, uint32(len(value)))
	payload = append(payload, value...)

	return append(payload, make([]byte, (4-len(value)%4)%4)...)
}

func replyDomainError(conn net.Conn, call []byte, code libvirt.ErrorNumber) error {
	payload := binary.BigEndian.AppendUint32(nil, uint32(code))
	payload = binary.BigEndian.AppendUint32(payload, 0) // error domain ID
	payload = binary.BigEndian.AppendUint32(payload, 0) // XDR padding
	payload = append(payload, encodeString("domain not found")...)
	payload = binary.BigEndian.AppendUint32(payload, 2) // error level

	reply := binary.BigEndian.AppendUint32(nil, uint32(28+len(payload)))
	reply = append(reply, call[:24]...)
	binary.BigEndian.PutUint32(reply[16:20], 1) // REMOTE_REPLY
	binary.BigEndian.PutUint32(reply[24:28], 1) // REMOTE_ERROR
	reply = append(reply, payload...)
	_, err := conn.Write(reply)

	return err
}

func (f *domainWireFixture) handleLookup(conn net.Conn, call []byte) error {
	name := decodeString(call[24:])

	record, ok := f.records[name]
	if !ok {
		return replyDomainError(conn, call, libvirt.ErrNoDomain)
	}

	if record.lookupError != libvirt.ErrOk {
		return replyDomainError(conn, call, record.lookupError)
	}

	return replyCall(conn, call, encodeDomain(record.identity))
}

//nolint:gocyclo
func (f *domainWireFixture) handle(conn net.Conn, call []byte) error {
	procedure := binary.BigEndian.Uint32(call[8:12])
	f.calls = append(f.calls, procedure)

	var payload []byte

	switch procedure {
	case 23: // DOMAIN_LOOKUP_BY_NAME
		return f.handleLookup(conn, call)
	case 10: // DOMAIN_CREATE_XML
		var err error

		payload, err = f.createDomain(call)
		if err != nil {
			return err
		}
	case 14: // DOMAIN_GET_XML_DESC
		name := decodeString(call[24:])
		payload = encodeString(f.records[name].xml)
	case 16: // DOMAIN_GET_INFO
		return replyDomainInfo(conn, call)
	case 150: // DOMAIN_IS_ACTIVE
		name := decodeString(call[24:])

		var active uint32 = 1
		if f.records[name].inactive {
			active = 0
		}

		payload = binary.BigEndian.AppendUint32(nil, active)
	case 151: // DOMAIN_IS_PERSISTENT
		payload = binary.BigEndian.AppendUint32(nil, 0)
	case 12: // DOMAIN_DESTROY
		name := decodeString(call[24:])
		delete(f.records, name)
	case 174: // DOMAIN_UPDATE_DEVICE_FLAGS
		if err := f.updateDevice(call); err != nil {
			return replyDomainError(conn, call, libvirt.ErrConfigUnsupported)
		}
	case 264: // DOMAIN_SET_METADATA
		if err := f.setMetadata(call); err != nil {
			return replyDomainError(conn, call, libvirt.ErrXMLError)
		}
	default:
		return fmt.Errorf("unexpected domain RPC %d", procedure)
	}

	return replyCall(conn, call, payload)
}

func (f *domainWireFixture) createDomain(call []byte) ([]byte, error) {
	var description libvirtxml.Domain
	if err := description.Unmarshal(decodeString(call[24:])); err != nil {
		return nil, err
	}

	id, err := uuid.Parse(description.UUID)
	if err != nil {
		return nil, err
	}

	identity := libvirtdomain.Domain{Name: description.Name, UUID: id}
	f.records[identity.Name] = domainRecord{identity: identity, xml: decodeString(call[24:])}

	return encodeDomain(identity), nil
}

// domainArgsOffset is where a call's payload continues after its leading domain argument: a padded
// name, a 16-byte UUID and the libvirt ID.
func domainArgsOffset(call []byte) int {
	length := binary.BigEndian.Uint32(call[24:28])

	return 28 + int(length) + int((4-length%4)%4) + 16 + 4
}

// decodeOptString reads one XDR optional string, as go-libvirt encodes a nil-able argument, and
// reports where the next one starts.
func decodeOptString(payload []byte) (string, int) {
	if binary.BigEndian.Uint32(payload[:4]) == 0 {
		return "", 4
	}

	length := binary.BigEndian.Uint32(payload[4:8])

	return string(payload[8 : 8+length]), 8 + int(length) + int((4-length%4)%4)
}

// updateDevice changes the medium of a drive of a stored domain, the way libvirt does.
//
// It is as strict as libvirt is: an element which differs from the running device anywhere but its
// source is refused, which is what makes "build the update from the running device" testable rather
// than merely intended. An eject leaves the tray open, and an insert closes it again.
//
//nolint:gocyclo
func (f *domainWireFixture) updateDevice(call []byte) error {
	name := decodeString(call[24:])

	record, ok := f.records[name]
	if !ok {
		return fmt.Errorf("domain %q does not exist", name)
	}

	var update libvirtxml.DomainDisk
	if err := xml.Unmarshal([]byte(decodeString(call[domainArgsOffset(call):])), &update); err != nil {
		return err
	}

	if update.Target == nil {
		return errors.New("device update names no target")
	}

	var description libvirtxml.Domain
	if err := description.Unmarshal(record.xml); err != nil {
		return err
	}

	for i := range description.Devices.Disks {
		disk := &description.Devices.Disks[i]
		if disk.Target == nil || disk.Target.Dev != update.Target.Dev {
			continue
		}

		if err := requireOnlySourceDiffers(*disk, update); err != nil {
			return err
		}

		// Reading XML back gives a drive with no source element one carrying no file, so judge the
		// medium by the file and not by the element.
		disk.Source, disk.Target.Tray = update.Source, ""

		if update.Source == nil || update.Source.File == nil || update.Source.File.File == "" {
			disk.Source, disk.Target.Tray = nil, "open"
		}

		updatedXML, err := description.Marshal()
		if err != nil {
			return err
		}

		record.xml = updatedXML
		f.records[name] = record

		return nil
	}

	return fmt.Errorf("domain %q has no drive %q", name, update.Target.Dev)
}

// requireOnlySourceDiffers models the field-by-field comparison libvirt makes before it touches a
// medium, in the only form this fixture needs: everything but the source has to match.
func requireOnlySourceDiffers(running, update libvirtxml.DomainDisk) error {
	running.Source, update.Source = nil, nil

	before, err := xml.Marshal(&running)
	if err != nil {
		return err
	}

	after, err := xml.Marshal(&update)
	if err != nil {
		return err
	}

	if string(before) != string(after) {
		return fmt.Errorf("cannot modify a field of the disk: %s is not %s", after, before)
	}

	return nil
}

// setMetadata replaces the element of the given namespace, as libvirt does, leaving the rest of the
// domain's metadata alone.
func (f *domainWireFixture) setMetadata(call []byte) error {
	name := decodeString(call[24:])

	record, ok := f.records[name]
	if !ok {
		return fmt.Errorf("domain %q does not exist", name)
	}

	payload := call[domainArgsOffset(call)+4:] // the metadata type precedes the strings

	metadata, read := decodeOptString(payload)
	if metadata == "" {
		return errors.New("refusing to delete the metadata")
	}

	_, keyRead := decodeOptString(payload[read:])

	uri, _ := decodeOptString(payload[read+keyRead:])
	if uri == "" {
		return errors.New("element metadata needs a namespace")
	}

	var description libvirtxml.Domain
	if err := description.Unmarshal(record.xml); err != nil {
		return err
	}

	if description.Metadata == nil {
		description.Metadata = &libvirtxml.DomainMetadata{}
	}

	description.Metadata.XML = replaceNamespacedElement(description.Metadata.XML, uri, metadata)

	updatedXML, err := description.Marshal()
	if err != nil {
		return err
	}

	record.xml = updatedXML
	f.records[name] = record

	return nil
}

// replaceNamespacedElement swaps the one child declaring uri, or appends when there is none. Enough
// for this fixture, whose domains carry a single metadata element.
func replaceNamespacedElement(existing, uri, element string) string {
	if strings.Contains(existing, uri) {
		return element
	}

	return existing + element
}

func replyDomainInfo(conn net.Conn, call []byte) error {
	payload := binary.BigEndian.AppendUint32(nil, uint32(libvirt.DomainRunning))
	payload = binary.BigEndian.AppendUint64(payload, 1048576)
	payload = binary.BigEndian.AppendUint64(payload, 524288)
	payload = binary.BigEndian.AppendUint32(payload, 2)
	payload = binary.BigEndian.AppendUint64(payload, 4000) // libvirt always returns CPU time, even though we do not expose it.

	return replyCall(conn, call, payload)
}

func openDomainFixture(t *testing.T, records ...domainRecord) (libvirtdomain.Client, <-chan *domainWireFixture, <-chan error) {
	t.Helper()

	raw, server := net.Pipe()

	fixture := &domainWireFixture{records: make(map[string]domainRecord)}
	for _, record := range records {
		fixture.records[record.identity.Name] = record
	}

	finished := make(chan *domainWireFixture, 1)
	served := make(chan error, 1)

	go func() {
		defer server.Close() //nolint:errcheck
		defer func() { finished <- fixture }()

		if err := serveHandshake(server); err != nil {
			served <- err

			return
		}

		for {
			call, err := readCall(server)
			if err != nil {
				served <- err

				return
			}

			if binary.BigEndian.Uint32(call[8:12]) == 2 { // CONNECT_CLOSE
				served <- replyCall(server, call, nil)

				return
			}

			if err = fixture.handle(server, call); err != nil {
				served <- err

				return
			}
		}
	}()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	client, err := libvirtdomain.New("", "qemu:///system").OpenConn(ctx, raw, cancel)
	require.NoError(t, err)

	return client, finished, served
}

func countProcedure(calls []uint32, procedure uint32) int {
	var count int

	for _, call := range calls {
		if call == procedure {
			count++
		}
	}

	return count
}

func TestInfoReadsLibvirtMetrics(t *testing.T) {
	t.Parallel()

	domain := libvirtdomain.Domain{Name: "external", UUID: uuid.New()}
	client, finished, served := openDomainFixture(t, domainRecord{identity: domain})

	info, err := client.Info(domain)
	require.NoError(t, err)
	require.Equal(t, uint32(libvirt.DomainRunning), info.State)
	require.Equal(t, uint64(1048576), info.MaxMemoryKiB)
	require.Equal(t, uint64(524288), info.MemoryKiB)
	require.Equal(t, uint32(2), info.VCPUs)

	client.Close()
	require.NoError(t, <-served)
	require.Equal(t, 1, countProcedure((<-finished).calls, 16))
}

func TestActiveReadsDomainState(t *testing.T) {
	t.Parallel()

	owned := libvirtdomain.Domain{Name: "first", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "first")}
	inactive := libvirtdomain.Domain{Name: "second", UUID: uuid.New()}
	client, finished, served := openDomainFixture(t,
		domainRecord{identity: owned},
		domainRecord{identity: inactive, inactive: true},
	)

	active, err := client.Active(owned)
	require.NoError(t, err)
	require.True(t, active)

	active, err = client.Active(inactive)
	require.NoError(t, err)
	require.False(t, active)

	active, err = client.Active(libvirtdomain.Domain{Name: "missing", UUID: uuid.New()})
	require.NoError(t, err)
	require.False(t, active)

	active, err = client.Active(libvirtdomain.Domain{Name: owned.Name, UUID: uuid.New()})
	require.ErrorContains(t, err, "not owned")
	require.False(t, active)

	client.Close()
	require.NoError(t, <-served)
	require.Equal(t, 2, countProcedure((<-finished).calls, 150))
}

func TestTransientDomainLifecycle(t *testing.T) {
	t.Parallel()

	id := libvirtdomain.UUID(uuid.MustParse(machineUUID), "first")
	domain := libvirtdomain.Domain{Name: "first", UUID: id}
	client, finished, served := openDomainFixture(t)

	initial := `<domain type="kvm"><name>first</name><vcpu>1</vcpu></domain>`
	updated := `<domain type="kvm"><name>first</name><vcpu>2</vcpu></domain>`

	require.NoError(t, client.Start(domain, initial))
	require.NoError(t, client.Start(domain, initial), "unchanged reapply must not restart")
	require.NoError(t, client.Start(domain, updated))
	require.NoError(t, client.Remove(domain))
	require.NoError(t, client.Remove(domain), "already absent removal is idempotent")
	client.Close()
	require.NoError(t, <-served)

	fixture := <-finished
	require.Empty(t, fixture.records)
	require.Equal(t, 2, countProcedure(fixture.calls, 10), "initial and updated definitions each start exactly once")
	require.Equal(t, 2, countProcedure(fixture.calls, 12), "update and removal each destroy exactly once")
	require.Zero(t, countProcedure(fixture.calls, 273), "named operations must not enumerate unrelated domains")
}

func TestForeignNameCollision(t *testing.T) {
	t.Parallel()

	domain := libvirtdomain.Domain{Name: "first", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "first")}
	foreign := domainRecord{identity: libvirtdomain.Domain{Name: "first", UUID: uuid.New()}}
	client, finished, served := openDomainFixture(t, foreign)

	require.ErrorContains(t, client.Start(domain, `<domain><name>first</name></domain>`), "not owned")
	require.ErrorContains(t, client.Remove(domain), "not owned")
	client.Close()
	require.NoError(t, <-served)

	fixture := <-finished
	require.Equal(t, foreign, fixture.records["first"])
	require.Zero(t, countProcedure(fixture.calls, 10))
	require.Zero(t, countProcedure(fixture.calls, 12))
}

func TestDomainLookupErrors(t *testing.T) {
	t.Parallel()

	domain := libvirtdomain.Domain{Name: "first", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "first")}

	for _, code := range []libvirt.ErrorNumber{libvirt.ErrNoConnect, libvirt.ErrNoStoragePool} {
		t.Run(code.String(), func(t *testing.T) {
			t.Parallel()

			client, finished, served := openDomainFixture(t, domainRecord{identity: domain, lookupError: code})
			require.Error(t, client.Start(domain, `<domain><name>first</name></domain>`))
			require.Error(t, client.Remove(domain))
			client.Close()
			require.NoError(t, <-served)

			fixture := <-finished
			require.Zero(t, countProcedure(fixture.calls, 10))
			require.Zero(t, countProcedure(fixture.calls, 12))
		})
	}
}

func TestInvalidDomainDefinitionsDoNotCallLibvirt(t *testing.T) {
	t.Parallel()

	valid := libvirtdomain.Domain{Name: "first", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "first")}
	client, finished, served := openDomainFixture(t)

	require.Error(t, client.Start(libvirtdomain.Domain{Name: "first"}, `<domain><name>first</name></domain>`))
	require.Error(t, client.Start(libvirtdomain.Domain{UUID: valid.UUID}, `<domain><name>first</name></domain>`))
	require.Error(t, client.Start(valid, `<not-a-domain/>`))
	require.Error(t, client.Start(valid, `<domain><name>different</name></domain>`))
	require.Error(t, client.Remove(libvirtdomain.Domain{}))
	client.Close()
	require.NoError(t, <-served)

	require.Empty(t, (<-finished).calls)
}

func TestMatchingUUIDWithoutOwnershipMetadata(t *testing.T) {
	t.Parallel()

	domain := libvirtdomain.Domain{Name: "first", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "first")}
	foreign := domainRecord{identity: domain, xml: `<domain><name>first</name><uuid>` + domain.UUID.String() + `</uuid></domain>`}
	client, finished, served := openDomainFixture(t, foreign)

	require.ErrorContains(t, client.Start(domain, `<domain><name>first</name></domain>`), "no Talos ownership metadata")
	require.ErrorContains(t, client.Remove(domain), "no Talos ownership metadata")
	client.Close()
	require.NoError(t, <-served)

	fixture := <-finished
	require.Equal(t, foreign, fixture.records["first"])
	require.Zero(t, countProcedure(fixture.calls, 10))
	require.Zero(t, countProcedure(fixture.calls, 12))
}

func TestStartAdmission(t *testing.T) {
	t.Parallel()

	domain := libvirtdomain.Domain{Name: "first", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "first")}
	initial := `<domain type="kvm"><name>first</name><vcpu>1</vcpu></domain>`
	updated := `<domain type="kvm"><name>first</name><vcpu>2</vcpu></domain>`
	rejected := errors.New("placement rejected")

	admission := func(calls *int, err error) libvirtdomain.StartOption {
		return libvirtdomain.WithStartAdmission(func() error {
			*calls++

			return err
		})
	}

	t.Run("new domain", func(t *testing.T) {
		t.Parallel()

		client, finished, served := openDomainFixture(t)

		var calls int

		require.ErrorIs(t, client.Start(domain, initial, admission(&calls, rejected)), rejected)
		require.Equal(t, 1, calls)
		require.NoError(t, client.Start(domain, initial, admission(&calls, nil)))
		require.Equal(t, 2, calls)
		client.Close()
		require.NoError(t, <-served)

		fixture := <-finished
		require.Equal(t, 1, countProcedure(fixture.calls, 10), "a rejected start must not create the domain")
		require.Contains(t, fixture.records, "first")
	})

	t.Run("unchanged active domain", func(t *testing.T) {
		t.Parallel()

		client, finished, served := openDomainFixture(t)
		require.NoError(t, client.Start(domain, initial))

		var calls int

		require.NoError(t, client.Start(domain, initial, admission(&calls, rejected)))
		require.Zero(t, calls, "an unchanged running domain is left alone without admission")
		client.Close()
		require.NoError(t, <-served)

		fixture := <-finished
		require.Equal(t, 1, countProcedure(fixture.calls, 10))
		require.Zero(t, countProcedure(fixture.calls, 12))
	})

	t.Run("replacement", func(t *testing.T) {
		t.Parallel()

		client, finished, served := openDomainFixture(t)
		require.NoError(t, client.Start(domain, initial))

		var calls int

		require.ErrorIs(t, client.Start(domain, updated, admission(&calls, rejected)), rejected)
		require.Equal(t, 1, calls)
		require.NoError(t, client.Start(domain, initial), "a rejected replacement keeps the old domain running")
		require.NoError(t, client.Start(domain, updated, admission(&calls, nil), admission(&calls, nil)))
		require.Equal(t, 3, calls, "every admission check runs")
		client.Close()
		require.NoError(t, <-served)

		fixture := <-finished
		require.Equal(t, 2, countProcedure(fixture.calls, 10), "only the admitted replacement is created")
		require.Equal(t, 1, countProcedure(fixture.calls, 12), "only the admitted replacement destroys the old domain")
	})

	t.Run("inactive domain", func(t *testing.T) {
		t.Parallel()

		client, finished, served := openDomainFixture(t)
		require.NoError(t, client.Start(domain, initial))
		client.Close()
		require.NoError(t, <-served)

		record := (<-finished).records["first"]
		record.inactive = true

		client, finished, served = openDomainFixture(t, record)

		var calls int

		require.ErrorIs(t, client.Start(domain, initial, admission(&calls, rejected)), rejected)
		require.Equal(t, 1, calls, "restarting a stopped domain is admitted like a new one")
		client.Close()
		require.NoError(t, <-served)

		fixture := <-finished
		require.Zero(t, countProcedure(fixture.calls, 10))
		require.Zero(t, countProcedure(fixture.calls, 12))
		require.Equal(t, record, fixture.records["first"])
	})

	t.Run("invalid definition", func(t *testing.T) {
		t.Parallel()

		client, finished, served := openDomainFixture(t)

		var calls int

		require.Error(t, client.Start(domain, `<not-a-domain/>`, admission(&calls, nil)))
		require.Zero(t, calls)
		client.Close()
		require.NoError(t, <-served)
		require.Empty(t, (<-finished).calls)
	})
}

// cdromDomainName is the one domain the medium cases work on.
const cdromDomainName = "first"

// cdromDomain renders a one-drive domain the way the renderer renders one: the drive carries the
// alias naming it a device the machine configuration declares. An empty source is a drive with no
// medium in it, which has no source element and so no source type on the disk either.
func cdromDomain(vcpu int, source string) string {
	return cdromDomainWithAlias(vcpu, source, "cdrom", libvirtdomain.DeviceAlias("disk", "install"))
}

// cdromDomainWithAlias is cdromDomain with the device's identity spelled out, for the cases which
// turn on what the alias says the device is.
func cdromDomainWithAlias(vcpu int, source, device, alias string) string {
	diskType, element := "", ""
	if source != "" {
		diskType, element = ` type="file"`, fmt.Sprintf(`<source file=%q/>`, source)
	}

	aliasElement := ""
	if alias != "" {
		aliasElement = fmt.Sprintf(`<alias name=%q/>`, alias)
	}

	return fmt.Sprintf(`<domain type="kvm"><name>%s</name><vcpu>%d</vcpu><devices>`+
		`<disk%s device=%q><driver name="qemu" type="raw"/>%s<target dev="sda" bus="sata"/><readonly/>%s</disk>`+
		`</devices></domain>`, cdromDomainName, vcpu, diskType, device, element, aliasElement)
}

// liveSource is the medium the fixture currently has in the drive.
func liveSource(t *testing.T, fixture *domainWireFixture, name string) string {
	t.Helper()

	var description libvirtxml.Domain
	require.NoError(t, description.Unmarshal(fixture.records[name].xml))

	disk := description.Devices.Disks[0]
	if disk.Source == nil || disk.Source.File == nil {
		return ""
	}

	return disk.Source.File.File
}

func TestMediaChangeLeavesTheDomainRunning(t *testing.T) {
	t.Parallel()

	domain := libvirtdomain.Domain{Name: "first", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "first")}
	client, finished, served := openDomainFixture(t)

	require.NoError(t, client.Start(domain, cdromDomain(1, "/lib/a.iso")))
	require.NoError(t, client.Start(domain, cdromDomain(1, "/lib/a.iso")),
		"unchanged reapply must do nothing")
	require.NoError(t, client.Start(domain, cdromDomain(1, "/lib/b.iso")),
		"swapping the medium must not restart")
	require.NoError(t, client.Start(domain, cdromDomain(1, "")),
		"ejecting the medium must not restart")
	require.NoError(t, client.Start(domain, cdromDomain(1, "/lib/a.iso")),
		"loading a medium into a drive left open by an eject must not restart")

	client.Close()
	require.NoError(t, <-served)

	fixture := <-finished
	require.Equal(t, "/lib/a.iso", liveSource(t, fixture, "first"))
	require.Equal(t, 1, countProcedure(fixture.calls, 10), "the domain is defined exactly once")
	require.Zero(t, countProcedure(fixture.calls, 12), "no medium change destroys the domain")
	require.Equal(t, 3, countProcedure(fixture.calls, 174), "one device update per medium that moved")
	require.Equal(t, 3, countProcedure(fixture.calls, 264), "every applied change records its digests")
}

func TestMediaChangeOnlyCoversWhatItWasToldAbout(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		device string
		alias  string
	}{
		{
			// A device the renderer added on its own account, such as a cloud-init seed, carries no
			// alias naming it a declared one. Its medium is boot-time intent, not something to swap
			// under a guest which already read it.
			name:   "drive is not a declared device",
			device: "cdrom",
			alias:  "",
		},
		{
			// libvirt changes the source of removable drives only, so a declared fixed disk has to
			// fall back to a restart rather than be sent an update it would refuse.
			name:   "declared device is not removable",
			device: "disk",
			alias:  libvirtdomain.DeviceAlias("disk", "install"),
		},
		{
			// An alias libvirt assigned itself says nothing about who declared the device.
			name:   "alias is not one of ours",
			device: "cdrom",
			alias:  "ide0-0-0",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			domain := libvirtdomain.Domain{Name: "first", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "first")}
			client, finished, served := openDomainFixture(t)

			require.NoError(t, client.Start(domain, cdromDomainWithAlias(1, "/lib/a.iso", test.device, test.alias)))
			require.NoError(t, client.Start(domain, cdromDomainWithAlias(1, "/lib/b.iso", test.device, test.alias)))

			client.Close()
			require.NoError(t, <-served)

			fixture := <-finished
			require.Equal(t, 2, countProcedure(fixture.calls, 10), "the source change redefines the domain")
			require.Equal(t, 1, countProcedure(fixture.calls, 12))
			require.Zero(t, countProcedure(fixture.calls, 174))
			require.Zero(t, countProcedure(fixture.calls, 264))
		})
	}
}

func TestChangeOutsideTheMediumRestarts(t *testing.T) {
	t.Parallel()

	domain := libvirtdomain.Domain{Name: "first", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "first")}
	client, finished, served := openDomainFixture(t)

	require.NoError(t, client.Start(domain, cdromDomain(1, "/lib/a.iso")))
	require.NoError(t, client.Start(domain, cdromDomain(2, "/lib/b.iso")),
		"a medium moving alongside anything else is not a medium change")

	client.Close()
	require.NoError(t, <-served)

	fixture := <-finished
	require.Equal(t, 2, countProcedure(fixture.calls, 10))
	require.Equal(t, 1, countProcedure(fixture.calls, 12))
	require.Zero(t, countProcedure(fixture.calls, 174))
}

// A domain defined before the core digest existed carries none, and is restarted rather than
// changed in place: nothing recorded what its definition was apart from its media.
func TestDomainWithoutCoreDigestRestarts(t *testing.T) {
	t.Parallel()

	id := libvirtdomain.UUID(uuid.MustParse(machineUUID), "first")
	domain := libvirtdomain.Domain{Name: "first", UUID: id}

	var description libvirtxml.Domain
	require.NoError(t, description.Unmarshal(cdromDomain(1, "/lib/a.iso")))

	description.UUID = id.String()
	description.Metadata = &libvirtxml.DomainMetadata{
		XML: `<talos:definition xmlns:talos="https://talos.dev/libvirt/domain">stale</talos:definition>`,
	}

	legacy, err := description.Marshal()
	require.NoError(t, err)

	client, finished, served := openDomainFixture(t, domainRecord{identity: domain, xml: legacy})

	require.NoError(t, client.Start(domain, cdromDomain(1, "/lib/b.iso")))

	client.Close()
	require.NoError(t, <-served)

	fixture := <-finished
	require.Equal(t, 1, countProcedure(fixture.calls, 10))
	require.Equal(t, 1, countProcedure(fixture.calls, 12))
	require.Zero(t, countProcedure(fixture.calls, 174))
}
