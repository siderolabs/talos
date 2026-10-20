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

func TestProfileIndependentDefinitionDigest(t *testing.T) {
	owned := libvirtdomain.Domain{
		Name: "profile-digest",
		UUID: uuid.New(),
	}
	original := libvirtxml.Domain{
		Type: "kvm",
		Name: owned.Name,
		Memory: &libvirtxml.DomainMemory{
			Value: 128 << 20,
			Unit:  "bytes",
		},
		VCPU: &libvirtxml.DomainVCPU{
			Value: 2,
		},
		OS: &libvirtxml.DomainOS{
			Type: &libvirtxml.DomainOSType{
				Type:    "hvm",
				Arch:    "x86_64",
				Machine: "q35",
			},
		},
		CPU: &libvirtxml.DomainCPU{
			Mode: "host-model",
			Topology: &libvirtxml.DomainCPUTopology{
				Sockets: 1,
				Cores:   2,
				Threads: 1,
			},
		},
	}
	xml, err := original.Marshal()
	require.NoError(t, err)
	_, initial, err := libvirtdomain.DomainDefinition(owned, xml)
	require.NoError(t, err)

	for _, test := range []struct {
		name        string
		profileOnly bool
		mutate      func(*libvirtxml.Domain)
	}{
		{
			name:        "machine",
			profileOnly: true,
			mutate: func(definition *libvirtxml.Domain) {
				definition.OS.Type.Machine = "pc-q35-9.2"
			},
		},
		{
			name:        "cpu-model-and-features",
			profileOnly: true,
			mutate: func(definition *libvirtxml.Domain) {
				definition.CPU.Mode = "custom"
				definition.CPU.Match = "exact"
				definition.CPU.Model = &libvirtxml.DomainCPUModel{
					Value:    "Haswell-noTSX-IBRS",
					Fallback: "forbid",
				}
				definition.CPU.Features = []libvirtxml.DomainCPUFeature{
					{
						Name:   "avx2",
						Policy: "require",
					},
				}
			},
		},
		{
			name: "sizing",
			mutate: func(definition *libvirtxml.Domain) {
				definition.VCPU.Value = 4
			},
		},
		{
			name: "topology",
			mutate: func(definition *libvirtxml.Domain) {
				definition.CPU.Topology.Sockets = 2
				definition.CPU.Topology.Cores = 1
			},
		},
		{
			name: "memory",
			mutate: func(definition *libvirtxml.Domain) {
				definition.Memory.Value *= 2
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var changed libvirtxml.Domain
			require.NoError(t, changed.Unmarshal(xml))
			test.mutate(&changed)
			changedXML, err := changed.Marshal()
			require.NoError(t, err)
			_, next, err := libvirtdomain.DomainDefinition(owned, changedXML)
			require.NoError(t, err)
			require.NotEqual(t, initial.Full, next.Full)

			if test.profileOnly {
				require.Equal(t, initial.Configuration, next.Configuration)
			} else {
				require.NotEqual(t, initial.Configuration, next.Configuration)
			}
		})
	}
}
