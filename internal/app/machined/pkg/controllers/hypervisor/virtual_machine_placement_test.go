// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	controller "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

func TestValidateDomainPlacement(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		xml      string
		topology *hardware.NUMATopologySpec
		invalid  string
		pending  bool
	}{
		{
			name: "no placement without topology",
			xml:  `<domain><name>vm</name><vcpu>2</vcpu></domain>`,
		},
		{
			name: "auto placement without topology",
			xml:  `<domain><name>vm</name><vcpu placement="auto" cpuset="900">2</vcpu><numatune><memory placement="auto" nodeset="900"/></numatune></domain>`,
		},
		{
			name:    "vcpu pin without topology",
			xml:     `<domain><name>vm</name><cputune><vcpupin vcpu="0" cpuset="0"/></cputune></domain>`,
			pending: true,
		},
		{
			name:    "nodeset without topology",
			xml:     `<domain><name>vm</name><numatune><memory mode="strict" nodeset="0"/></numatune></domain>`,
			pending: true,
		},
		{
			name:     "remote memory",
			xml:      `<domain><name>vm</name><cputune><vcpupin vcpu="0" cpuset="0-2"/><emulatorpin cpuset="4"/></cputune><numatune><memory mode="strict" nodeset="1-2"/></numatune></domain>`,
			topology: newNUMATopology().TypedSpec(),
		},
		{
			name:     "excluded offline CPU",
			xml:      `<domain><name>vm</name><vcpu cpuset="0-3,^3">2</vcpu></domain>`,
			topology: newNUMATopology().TypedSpec(),
		},
		{
			name:     "offline vCPU pin",
			xml:      `<domain><name>vm</name><cputune><vcpupin vcpu="1" cpuset="2-3"/></cputune></domain>`,
			topology: newNUMATopology().TypedSpec(),
			invalid:  `virtual machine "vm": invalid host placement: vCPU 1 pin names offline host CPU 3`,
		},
		{
			name:     "absent emulator CPU",
			xml:      `<domain><name>vm</name><cputune><emulatorpin cpuset="7"/></cputune></domain>`,
			topology: newNUMATopology().TypedSpec(),
			invalid:  `virtual machine "vm": invalid host placement: emulator pin names absent host CPU 7`,
		},
		{
			name:     "offline vcpu cpuset",
			xml:      `<domain><name>vm</name><vcpu cpuset="3">1</vcpu></domain>`,
			topology: newNUMATopology().TypedSpec(),
			invalid:  `virtual machine "vm": invalid host placement: vcpu cpuset names offline host CPU 3`,
		},
		{
			name:     "iothread pin",
			xml:      `<domain><name>vm</name><cputune><iothreadpin iothread="1" cpuset="3"/></cputune></domain>`,
			topology: newNUMATopology().TypedSpec(),
			invalid:  `virtual machine "vm": invalid host placement: iothread 1 pin names offline host CPU 3`,
		},
		{
			name:     "absent node",
			xml:      `<domain><name>vm</name><numatune><memory mode="strict" nodeset="4"/></numatune></domain>`,
			topology: newNUMATopology().TypedSpec(),
			invalid:  `virtual machine "vm": invalid host placement: memory nodeset names absent host NUMA node 4`,
		},
		{
			name:     "memoryless node",
			xml:      `<domain><name>vm</name><numatune><memory mode="preferred" nodeset="1,3"/></numatune></domain>`,
			topology: newNUMATopology().TypedSpec(),
			invalid:  `virtual machine "vm": invalid host placement: memory nodeset names host NUMA node 3 without memory`,
		},
		{
			name:     "memoryless memnode",
			xml:      `<domain><name>vm</name><numatune><memnode cellid="0" mode="strict" nodeset="3"/></numatune></domain>`,
			topology: newNUMATopology().TypedSpec(),
			invalid:  `virtual machine "vm": invalid host placement: memnode 0 nodeset names host NUMA node 3 without memory`,
		},
		{
			name:     "memory device nodemask",
			xml:      `<domain><name>vm</name><devices><memory model="dimm"><source><nodemask>5</nodemask></source></memory></devices></domain>`,
			topology: newNUMATopology().TypedSpec(),
			invalid:  `virtual machine "vm": invalid host placement: memory device 0 nodemask names absent host NUMA node 5`,
		},
		{
			name:     "CPU on memoryless node",
			xml:      `<domain><name>vm</name><cputune><vcpupin vcpu="0" cpuset="6"/></cputune><numatune><memory mode="strict" nodeset="2"/></numatune></domain>`,
			topology: newNUMATopology().TypedSpec(),
		},
		{
			name:     "unbounded range",
			xml:      `<domain><name>vm</name><vcpu cpuset="0-1000000000">1</vcpu></domain>`,
			topology: newNUMATopology().TypedSpec(),
			invalid:  `virtual machine "vm": invalid host placement: vcpu cpuset "0-1000000000": 1000000000 is out of range, IDs must be between 0 and 999`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := controller.ValidateDomainPlacement("vm", test.xml, test.topology)

			switch {
			case test.pending:
				require.ErrorIs(t, err, controller.ErrPlacementPending)
				require.True(t, controller.IsPlacementHeld(err))
			case test.invalid != "":
				require.ErrorIs(t, err, controller.ErrPlacementInvalid)
				require.True(t, controller.IsPlacementHeld(err))
				require.EqualError(t, err, test.invalid)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateDomainPlacementInvalidXML(t *testing.T) {
	t.Parallel()

	err := controller.ValidateDomainPlacement("vm", `<not-a-domain/>`, nil)
	require.Error(t, err)
	require.False(t, controller.IsPlacementHeld(err), "an invalid definition is an ordinary failure")
}
