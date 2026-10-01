// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	hypervisorapi "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestVirtualMachineSpecRoundTrip(t *testing.T) {
	t.Parallel()

	res := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest")
	*res.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU: hypervisor.VirtualMachineCPUSpec{
			Count:       3,
			Pins:        []hypervisor.VirtualMachineVCPUPinSpec{{VCPU: 0, CPUs: "4-5"}},
			EmulatorPin: "6",
			Topology:    &hypervisor.VirtualMachineCPUTopologySpec{Sockets: 1, Cores: 3, Threads: 1},
			Slice:       "database",
		},
		PowerState: "suspended",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi", SecureBoot: true},
		Console:    hypervisor.VirtualMachineConsoleSpec{Serial: true, VNC: true},
		Disks: []hypervisor.VirtualMachineDiskSpec{
			{
				Name:      "system",
				Pool:      "pool1",
				Size:      20 << 30,
				Format:    "qcow2",
				Bus:       "virtio",
				Type:      "disk",
				BootOrder: 1,
				Provision: hypervisor.VirtualMachineDiskProvisionSpec{
					FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{
						Library: "images",
						File:    "system.qcow2",
						Digest:  "sha256:abc",
						Mode:    "linked",
					},
				},
			},
		},
		Memory: hypervisor.VirtualMachineMemorySpec{
			Size:       4 << 30,
			Ballooning: hypervisor.VirtualMachineMemoryBallooningSpec{Enabled: true},
			NUMA:       &hypervisor.VirtualMachineMemoryNUMASpec{Mode: "strict", Nodes: "1"},
		},
	}
	encoded, err := protobuf.FromResource(res)
	require.NoError(t, err)
	wire, err := encoded.Marshal()
	require.NoError(t, err)

	var spec hypervisorapi.VirtualMachineSpecSpec

	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	assert.Equal(t, res.TypedSpec().CPU.Count, spec.GetCpu().GetCount())
	assert.Equal(t, "database", spec.GetCpu().GetSlice())
	assert.Equal(t, res.TypedSpec().Memory.Size, spec.GetMemory().GetSize())
	assert.Equal(t, res.TypedSpec().Memory.Ballooning.Enabled, spec.GetMemory().GetBallooning().GetEnabled())
	assert.Equal(t, res.TypedSpec().PowerState, spec.GetPowerState())
	assert.Equal(t, res.TypedSpec().Firmware.Type, spec.GetFirmware().GetType())
	assert.Equal(t, res.TypedSpec().Firmware.SecureBoot, spec.GetFirmware().GetSecureBoot())
	assert.Equal(t, res.TypedSpec().Console.Serial, spec.GetConsole().GetSerial())
	assert.Equal(t, res.TypedSpec().Console.VNC, spec.GetConsole().GetVnc())
	assert.Equal(t, res.TypedSpec().Disks[0].Provision.FromImage.Digest, spec.GetDisks()[0].GetProvision().GetFromImage().GetDigest())

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)
	roundTrip, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.IsType(t, res, roundTrip)
	assert.Equal(t, res.TypedSpec(), roundTrip.(*hypervisor.VirtualMachineSpec).TypedSpec())

	clone := res.DeepCopy().(*hypervisor.VirtualMachineSpec)
	clone.TypedSpec().CPU.Count++
	clone.TypedSpec().CPU.Topology.Cores = 4
	clone.TypedSpec().CPU.Pins[0].CPUs = "7"
	clone.TypedSpec().CPU.EmulatorPin = "8"
	clone.TypedSpec().CPU.Slice = "changed"
	clone.TypedSpec().Memory.NUMA.Nodes = "0"
	assert.Equal(t, uint32(3), res.TypedSpec().CPU.Topology.Cores)
	assert.Equal(t, "4-5", res.TypedSpec().CPU.Pins[0].CPUs)
	assert.Equal(t, "6", res.TypedSpec().CPU.EmulatorPin)
	assert.Equal(t, "database", res.TypedSpec().CPU.Slice)
	assert.Equal(t, "1", res.TypedSpec().Memory.NUMA.Nodes)
	clone.TypedSpec().Memory.Size++
	clone.TypedSpec().Memory.Ballooning.Enabled = false
	clone.TypedSpec().Disks[0].Provision.FromImage.Digest = "changed"
	assert.Equal(t, "sha256:abc", res.TypedSpec().Disks[0].Provision.FromImage.Digest)
	assert.NotEqual(t, clone.TypedSpec().CPU.Count, res.TypedSpec().CPU.Count)
	assert.NotEqual(t, clone.TypedSpec().Memory.Size, res.TypedSpec().Memory.Size)
	assert.NotEqual(t, clone.TypedSpec().Memory.Ballooning.Enabled, res.TypedSpec().Memory.Ballooning.Enabled)
	assert.Equal(t, hypervisor.NamespaceName, res.ResourceDefinition().DefaultNamespace)

	marshaled, err := yaml.Marshal(res.TypedSpec())
	require.NoError(t, err)
	assert.Equal(t, `cpu:
    count: 3
    pins:
        - vcpu: 0
          cpus: 4-5
    emulatorPin: "6"
    topology:
        sockets: 1
        cores: 3
        threads: 1
    slice: database
memory:
    size: 4294967296
    ballooning:
        enabled: true
    numa:
        mode: strict
        nodes: "1"
powerState: suspended
firmware:
    type: uefi
    secureBoot: true
console:
    serial: true
    vnc: true
disks:
    - name: system
      pool: pool1
      size: 21474836480
      format: qcow2
      bus: virtio
      type: disk
      bootOrder: 1
      provision:
        blank: false
        fromImage:
            library: images
            file: system.qcow2
            digest: sha256:abc
            mode: linked
`, string(marshaled))

	var yamlSpec hypervisor.VirtualMachineSpecSpec

	require.NoError(t, yaml.Unmarshal(marshaled, &yamlSpec))
	assert.Equal(t, *res.TypedSpec(), yamlSpec)
}

func TestVirtualMachineDomainSpecRoundTrip(t *testing.T) {
	t.Parallel()

	res := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "guest")
	res.TypedSpec().DomainXML = `<domain type="kvm"><name>guest</name></domain>`
	res.TypedSpec().PowerState = "running"
	encoded, err := protobuf.FromResource(res)
	require.NoError(t, err)
	wire, err := encoded.Marshal()
	require.NoError(t, err)

	var spec hypervisorapi.VirtualMachineDomainSpecSpec

	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	assert.Equal(t, res.TypedSpec().DomainXML, spec.GetDomainXml())
	assert.Equal(t, res.TypedSpec().PowerState, spec.GetPowerState())

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)
	roundTrip, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.IsType(t, res, roundTrip)
	assert.Equal(t, res.TypedSpec(), roundTrip.(*hypervisor.VirtualMachineDomainSpec).TypedSpec())

	clone := res.DeepCopy().(*hypervisor.VirtualMachineDomainSpec)
	clone.TypedSpec().DomainXML = "changed"
	assert.NotEqual(t, clone.TypedSpec().DomainXML, res.TypedSpec().DomainXML)
	assert.Equal(t, hypervisor.NamespaceName, res.ResourceDefinition().DefaultNamespace)
}
