// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"libvirt.org/go/libvirtxml"

	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
)

func TestSerialCaptureDefinition(t *testing.T) {
	t.Parallel()

	identity := libvirtdomain.Domain{
		Name: "serial-guest",
		UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), "serial-guest"),
	}
	client, finished, served := openDomainFixture(t)
	text := `<domain type="kvm"><name>serial-guest</name><devices><serial type="pty"/><console type="pty"><target type="serial" port="0"/></console></devices></domain>`
	require.NoError(t, client.Start(identity, text))
	require.NoError(t, client.Start(identity, text), "unchanged definition must not reopen the log")
	client.Close()
	require.NoError(t, <-served)

	fixture := <-finished

	var description libvirtxml.Domain
	require.NoError(t, description.Unmarshal(fixture.records[identity.Name].xml))
	require.Len(t, description.Devices.Serials, 1)
	serial := description.Devices.Serials[0]
	require.NotNil(t, serial.Source.Pty, "capture must preserve the interactive PTY")
	require.NotNil(t, serial.Log)
	require.Equal(t, "/var/log/vm-"+identity.UUID.String()+"-serial0.log", serial.Log.File)
	require.Equal(t, "on", serial.Log.Append)
	require.Nil(t, description.Devices.Consoles[0].Log, "the serial alias must not acquire a second logger")
	require.Equal(t, 1, countProcedure(fixture.calls, 10))
}

func TestSerialCaptureIdentityAndReopen(t *testing.T) {
	t.Parallel()

	machine := uuid.MustParse(machineUUID)
	identities := []libvirtdomain.Domain{
		{
			Name: "first",
			UUID: libvirtdomain.UUID(machine, "first"),
		},
		{
			Name: "second",
			UUID: libvirtdomain.UUID(machine, "second"),
		},
		{
			Name: "first",
			UUID: libvirtdomain.UUID(uuid.MustParse("26796c2a-5a31-4ee7-8a59-84ad587d1993"), "first"),
		},
	}
	paths := make(map[string]struct{})

	for _, identity := range identities {
		client, finished, served := openDomainFixture(t)
		text := `<domain><name>` + identity.Name + `</name><devices><serial type="pty"><log file="/guest-supplied" append="off"/></serial></devices></domain>`
		require.NoError(t, client.Start(identity, text))
		require.NoError(t, client.Remove(identity))
		require.NoError(t, client.Start(identity, text), "recreation must reuse the append-enabled identity path")
		client.Close()
		require.NoError(t, <-served)

		fixture := <-finished

		var description libvirtxml.Domain
		require.NoError(t, description.Unmarshal(fixture.records[identity.Name].xml))
		log := description.Devices.Serials[0].Log
		require.NotNil(t, log)
		require.Equal(t, "/var/log/vm-"+identity.UUID.String()+"-serial0.log", log.File)
		require.Equal(t, "on", log.Append)
		require.NotContains(t, fixture.records[identity.Name].xml, "/guest-supplied")

		_, exists := paths[log.File]
		require.False(t, exists, "different host/name identities must not share a log")

		paths[log.File] = struct{}{}

		require.Equal(t, 2, countProcedure(fixture.calls, 10))
	}
}

func TestSerialCaptureOnlyUsesSerialPTYs(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		devices string
	}{
		{
			name:    "disabled",
			devices: `<devices/>`,
		},
		{
			name:    "file",
			devices: `<devices><serial type="file"><source path="/existing"/></serial></devices>`,
		},
		{
			name:    "console-only",
			devices: `<devices><console type="pty"><target type="virtio"/></console></devices>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			identity := libvirtdomain.Domain{
				Name: test.name,
				UUID: libvirtdomain.UUID(uuid.MustParse(machineUUID), test.name),
			}
			client, finished, served := openDomainFixture(t)
			text := `<domain><name>` + identity.Name + `</name>` + test.devices + `</domain>`
			require.NoError(t, client.Start(identity, text))
			client.Close()
			require.NoError(t, <-served)

			fixture := <-finished
			require.NotContains(t, fixture.records[identity.Name].xml, "<log ")
			require.Contains(t, fixture.records[identity.Name].xml, descriptionDeviceType(test.name))
		})
	}
}

func descriptionDeviceType(name string) string {
	switch name {
	case "file":
		return `serial type="file"`
	case "console-only":
		return `console type="pty"`
	default:
		return "<devices>"
	}
}
