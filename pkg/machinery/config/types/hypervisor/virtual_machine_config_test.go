// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
)

//nolint:dupl
func TestVirtualMachineConfigMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		filename string
		cfg      func() *hypervisor.VirtualMachineConfigV1Alpha1
	}{
		{
			name:     "ballooning",
			filename: "virtualmachineconfig.yaml",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := hypervisor.NewVirtualMachineConfigV1Alpha1()
				c.MetaName = "vm1"
				c.CPUConfig.CPUCount = 4
				c.MemoryConfig.MemorySize = block.MustByteSize("4GiB")
				c.MemoryConfig.BallooningConfig = &hypervisor.VirtualMachineBallooning{
					BallooningEnabled: new(true),
				}

				return c
			},
		},
		{
			name:     "minimal",
			filename: "virtualmachineconfig_minimal.yaml",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := hypervisor.NewVirtualMachineConfigV1Alpha1()
				c.MetaName = "vm2"
				c.CPUConfig.CPUCount = 1
				c.MemoryConfig.MemorySize = block.MustByteSize("512MiB")

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg()

			warnings, err := cfg.Validate(validationMode{})
			require.NoError(t, err)
			require.Empty(t, warnings)

			marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
			require.NoError(t, err)

			t.Log(string(marshaled))

			expectedMarshaled, err := os.ReadFile(filepath.Join("testdata", test.filename))
			require.NoError(t, err)

			assert.Equal(t, string(expectedMarshaled), string(marshaled))

			provider, err := configloader.NewFromBytes(expectedMarshaled)
			require.NoError(t, err)

			docs := provider.Documents()
			require.Len(t, docs, 1)

			assert.Equal(t, cfg, docs[0])
		})
	}
}

func TestVirtualMachineConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func() *hypervisor.VirtualMachineConfigV1Alpha1

		expectedErrors string
	}{
		{
			name: "empty",
			cfg:  hypervisor.NewVirtualMachineConfigV1Alpha1,

			expectedErrors: "name is required\ncpu.count is required\nmemory.size is required",
		},
		{
			name: "invalid name",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.MetaName = "my vm"

				return c
			},

			expectedErrors: `name "my vm": name can only contain ASCII letters, digits and hyphens`,
		},
		{
			name: "name too long",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.MetaName = strings.Repeat("a", 64)

				return c
			},

			expectedErrors: fmt.Sprintf("name %q must be 63 characters or fewer", strings.Repeat("a", 64)),
		},
		{
			name: "no vCPUs",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.CPUConfig.CPUCount = 0

				return c
			},

			expectedErrors: "cpu.count is required",
		},
		{
			name: "no memory",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.MemoryConfig.MemorySize = block.ByteSize{}

				return c
			},

			expectedErrors: "memory.size is required",
		},
		{
			name: "zero memory",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.MemoryConfig.MemorySize = block.MustByteSize("0")

				return c
			},

			expectedErrors: "memory.size must be greater than zero",
		},
		{
			name: "negative memory",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.MemoryConfig.MemorySize = block.MustByteSize("-4GiB")

				return c
			},

			expectedErrors: "memory.size must not be negative",
		},
		{
			name: "valid",
			cfg:  validVirtualMachineConfig,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg()

			_, err := cfg.Validate(validationMode{})

			if test.expectedErrors == "" {
				require.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectedErrors)
			}
		})
	}
}

func validVirtualMachineConfig() *hypervisor.VirtualMachineConfigV1Alpha1 {
	c := hypervisor.NewVirtualMachineConfigV1Alpha1()
	c.MetaName = "vm1"
	c.CPUConfig.CPUCount = 4
	c.MemoryConfig.MemorySize = block.MustByteSize("4GiB")

	return c
}
