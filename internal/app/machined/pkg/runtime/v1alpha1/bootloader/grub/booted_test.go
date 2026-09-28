// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub_test

import (
	"testing"

	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/kexec"
)

func newABConfig(t *testing.T, defaultLabel grub.BootLabel) *grub.Config {
	t.Helper()

	config := grub.NewConfig()
	require.NoError(t, config.Put(grub.BootA, "cmdline A", "v1.0.0"))
	require.NoError(t, config.Put(grub.BootB, "cmdline B", "v1.1.0"))

	config.Default = defaultLabel

	return config
}

func TestDetectBooted(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		cmdline string

		expected grub.BootLabel
	}{
		{
			name:     "A",
			cmdline:  "BOOT_IMAGE=/A/vmlinuz talos.platform=metal",
			expected: grub.BootA,
		},
		{
			name:     "B",
			cmdline:  "BOOT_IMAGE=/B/vmlinuz talos.platform=metal",
			expected: grub.BootB,
		},
		{
			name:     "device prefix",
			cmdline:  "BOOT_IMAGE=(hd0,gpt3)/B/vmlinuz talos.platform=metal",
			expected: grub.BootB,
		},
		{
			name:    "no BOOT_IMAGE",
			cmdline: "talos.platform=metal",
		},
		{
			name:    "unknown kernel",
			cmdline: "BOOT_IMAGE=/boot/vmlinuz talos.platform=metal",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			config := newABConfig(t, grub.BootA)
			config.DetectBooted(procfs.NewCmdline(test.cmdline))

			assert.Equal(t, test.expected, config.Booted)
		})
	}
}

func TestSelectUpgradeTarget(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		defaultLabel  grub.BootLabel
		bootedLabel   grub.BootLabel
		expectDefault grub.BootLabel
	}{
		{
			name:          "booted default A",
			defaultLabel:  grub.BootA,
			bootedLabel:   grub.BootA,
			expectDefault: grub.BootB,
		},
		{
			name:          "booted default B",
			defaultLabel:  grub.BootB,
			bootedLabel:   grub.BootB,
			expectDefault: grub.BootA,
		},
		{
			// failed upgrade to B, operator selected A manually in the GRUB menu
			name:          "manually booted A",
			defaultLabel:  grub.BootB,
			bootedLabel:   grub.BootA,
			expectDefault: grub.BootB,
		},
		{
			name:          "manually booted B",
			defaultLabel:  grub.BootA,
			bootedLabel:   grub.BootB,
			expectDefault: grub.BootA,
		},
		{
			name:          "booted unknown",
			defaultLabel:  grub.BootB,
			expectDefault: grub.BootA,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			config := newABConfig(t, test.defaultLabel)
			config.Booted = test.bootedLabel

			require.NoError(t, config.SelectUpgradeTarget(t.Logf))

			assert.Equal(t, test.expectDefault, config.Default)

			expectedFallback := test.bootedLabel
			if expectedFallback == "" {
				expectedFallback = test.defaultLabel
			}

			assert.Equal(t, expectedFallback, config.Fallback)
		})
	}
}

func TestDetectBootedKexec(t *testing.T) {
	t.Parallel()

	// the kexec command line is detected back as the kexec'ed entry
	config := newABConfig(t, grub.BootB)
	config.DetectBooted(procfs.NewCmdline(kexec.AppendBootImage(config.Entries[grub.BootB].Cmdline, config.Entries[grub.BootB].Linux)))

	assert.Equal(t, grub.BootB, config.Booted)
}
