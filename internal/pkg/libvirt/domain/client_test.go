// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
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
	// requestError is what libvirt answers a power button press with.
	requestError libvirt.ErrorNumber
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

func (f *domainWireFixture) handle(conn net.Conn, call []byte) error {
	procedure := binary.BigEndian.Uint32(call[8:12])
	f.calls = append(f.calls, procedure)

	var payload []byte

	switch procedure {
	case 23: // DOMAIN_LOOKUP_BY_NAME
		return f.handleLookup(conn, call)
	case 10: // DOMAIN_CREATE_XML
		return f.handleCreate(conn, call)
	case 14: // DOMAIN_GET_XML_DESC
		name := decodeString(call[24:])
		payload = encodeString(f.records[name].xml)
	case 16: // DOMAIN_GET_INFO
		return replyDomainInfo(conn, call)
	case 150: // DOMAIN_IS_ACTIVE
		payload = binary.BigEndian.AppendUint32(nil, f.activeFlag(call))
	case 151: // DOMAIN_IS_PERSISTENT
		payload = binary.BigEndian.AppendUint32(nil, 0)
	case 12: // DOMAIN_DESTROY
		name := decodeString(call[24:])
		delete(f.records, name)
	case 258, 27: // DOMAIN_SHUTDOWN_FLAGS, DOMAIN_REBOOT
		return f.deliverRequest(conn, call)
	default:
		return fmt.Errorf("unexpected domain RPC %d", procedure)
	}

	return replyCall(conn, call, payload)
}

// handleCreate defines a domain and answers with it.
func (f *domainWireFixture) handleCreate(conn net.Conn, call []byte) error {
	payload, err := f.createDomain(call)
	if err != nil {
		return err
	}

	return replyCall(conn, call, payload)
}

// activeFlag reports what the record says about the domain being up.
func (f *domainWireFixture) activeFlag(call []byte) uint32 {
	if f.records[decodeString(call[24:])].inactive {
		return 0
	}

	return 1
}

// deliverRequest answers a request which is delivered rather than obeyed: the domain stays exactly
// as it was, unless the record was set up to refuse the request.
func (f *domainWireFixture) deliverRequest(conn net.Conn, call []byte) error {
	if record, ok := f.records[decodeString(call[24:])]; ok && record.requestError != libvirt.ErrOk {
		return replyDomainError(conn, call, record.requestError)
	}

	return replyCall(conn, call, nil)
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

// ownedDomain is the name every domain these fixtures define goes by.
const ownedDomain = "first"

// ownedRecord is a domain this machine defined, which the ownership checks accept.
func ownedRecord() domainRecord {
	id := libvirtdomain.UUID(uuid.MustParse(machineUUID), ownedDomain)
	digest := `<talos:definition xmlns:talos="https://talos.dev/libvirt/domain">deadbeef</talos:definition>`

	return domainRecord{
		identity: libvirtdomain.Domain{Name: ownedDomain, UUID: id},
		xml: `<domain><name>` + ownedDomain + `</name><uuid>` + id.String() + `</uuid>` +
			`<metadata>` + digest + `</metadata></domain>`,
	}
}

func TestShutdownAsksTheGuestAndLeavesTheDomainAlone(t *testing.T) {
	t.Parallel()

	record := ownedRecord()
	client, finished, served := openDomainFixture(t, record)

	require.NoError(t, client.Shutdown(record.identity))
	client.Close()
	require.NoError(t, <-served)

	fixture := <-finished
	require.Equal(t, 1, countProcedure(fixture.calls, 258), "the guest is asked exactly once per call")
	require.Zero(t, countProcedure(fixture.calls, 12), "asking the guest must not destroy the domain")
	require.Contains(t, fixture.records, "first", "the domain goes away with the guest, not with the request")
}

func TestShutdownRefusesDomainsThisMachineDoesNotOwn(t *testing.T) {
	t.Parallel()

	domain := libvirtdomain.Domain{Name: "first", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "first")}
	foreign := domainRecord{identity: libvirtdomain.Domain{Name: "first", UUID: uuid.New()}}
	unmarked := domainRecord{identity: domain, xml: `<domain><name>first</name></domain>`}

	for name, record := range map[string]domainRecord{"foreign UUID": foreign, "no metadata": unmarked} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client, finished, served := openDomainFixture(t, record)

			require.Error(t, client.Shutdown(domain))
			require.Error(t, client.Reboot(domain))
			client.Close()
			require.NoError(t, <-served)

			fixture := <-finished
			require.Zero(t, countProcedure(fixture.calls, 258))
			require.Zero(t, countProcedure(fixture.calls, 27))
		})
	}
}

// A domain which is already gone, or already not running, is in the state the caller asked for.
func TestShutdownOfAnAbsentDomainIsNotAnError(t *testing.T) {
	t.Parallel()

	domain := libvirtdomain.Domain{Name: "first", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "first")}
	client, finished, served := openDomainFixture(t)

	require.NoError(t, client.Shutdown(domain))
	client.Close()
	require.NoError(t, <-served)

	fixture := <-finished
	require.Zero(t, countProcedure(fixture.calls, 258))
}

func TestShutdownOfADomainWhichStoppedMidRequestIsNotAnError(t *testing.T) {
	t.Parallel()

	record := ownedRecord()
	record.requestError = libvirt.ErrOperationInvalid
	client, finished, served := openDomainFixture(t, record)

	require.NoError(t, client.Shutdown(record.identity))
	client.Close()
	require.NoError(t, <-served)

	fixture := <-finished
	require.Equal(t, 1, countProcedure(fixture.calls, 258))
}

func TestRebootAsksTheGuestAndKeepsTheDomain(t *testing.T) {
	t.Parallel()

	record := ownedRecord()
	client, finished, served := openDomainFixture(t, record)

	require.NoError(t, client.Reboot(record.identity))
	client.Close()
	require.NoError(t, <-served)

	fixture := <-finished
	require.Equal(t, 1, countProcedure(fixture.calls, 27))
	require.Zero(t, countProcedure(fixture.calls, 12))
	require.Contains(t, fixture.records, "first")
}

// Unlike a shutdown, a reboot of something which is not running has nothing to ask.
func TestRebootRequiresARunningDomain(t *testing.T) {
	t.Parallel()

	domain := libvirtdomain.Domain{Name: "first", UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "first")}

	absent, _, absentServed := openDomainFixture(t)
	require.ErrorIs(t, absent.Reboot(domain), libvirtdomain.ErrDomainNotRunning)
	absent.Close()
	require.NoError(t, <-absentServed)

	stopped := ownedRecord()
	stopped.inactive = true
	client, finished, served := openDomainFixture(t, stopped)

	require.ErrorIs(t, client.Reboot(domain), libvirtdomain.ErrDomainNotRunning)
	client.Close()
	require.NoError(t, <-served)

	fixture := <-finished
	require.Zero(t, countProcedure(fixture.calls, 27))
}
