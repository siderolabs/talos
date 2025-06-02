// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imager_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/reporter"
)

func TestImager(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		prof profile.Profile

		expected string
	}{
		{
			name: "cmdline-pre1.8-amd64",

			prof: profile.Profile{
				BaseProfileName: "metal",
				Arch:            "amd64",
				Output: profile.Output{
					Kind:      profile.OutKindCmdline,
					OutFormat: profile.OutFormatRaw,
				},
				Version: "1.7.0",
			},

			expected: "talos.platform=metal console=ttyS0 console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512", //nolint:lll
		},
		{
			name: "cmdline-pre1.8-arm64",

			prof: profile.Profile{
				BaseProfileName: "metal",
				Arch:            "arm64",
				Output: profile.Output{
					Kind:      profile.OutKindCmdline,
					OutFormat: profile.OutFormatRaw,
				},
				Version: "1.7.0",
			},

			expected: "talos.platform=metal console=ttyAMA0 console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512", //nolint:lll
		},
		{
			name: "cmdline-1.8-amd64",

			prof: profile.Profile{
				BaseProfileName: "metal",
				Arch:            "amd64",
				Output: profile.Output{
					Kind:      profile.OutKindCmdline,
					OutFormat: profile.OutFormatRaw,
				},
				Version: "1.8.0",
			},

			expected: "talos.platform=metal console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512", //nolint:lll
		},
		{
			name: "cmdline-1.8-arm64",

			prof: profile.Profile{
				BaseProfileName: "metal",
				Arch:            "arm64",
				Output: profile.Output{
					Kind:      profile.OutKindCmdline,
					OutFormat: profile.OutFormatRaw,
				},
				Version: "1.8.0",
			},

			expected: "talos.platform=metal console=ttyAMA0 console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512", //nolint:lll
		},
		{
			name: "cmdline-1.10-amd64",

			prof: profile.Profile{
				BaseProfileName: "metal",
				Arch:            "amd64",
				Output: profile.Output{
					Kind:      profile.OutKindCmdline,
					OutFormat: profile.OutFormatRaw,
				},
				Version: "1.10.1",
			},

			expected: "talos.platform=metal console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512 selinux=1", //nolint:lll
		},
		{
			name: "cmdline-1.10-arm64",

			prof: profile.Profile{
				BaseProfileName: "metal",
				Arch:            "arm64",
				Output: profile.Output{
					Kind:      profile.OutKindCmdline,
					OutFormat: profile.OutFormatRaw,
				},
				Version: "1.10.1",
			},

			expected: "talos.platform=metal console=ttyAMA0 console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512 selinux=1", //nolint:lll
		},
		{
			name: "cmdline-1.11-amd64",

			prof: profile.Profile{
				BaseProfileName: "metal",
				Arch:            "amd64",
				Output: profile.Output{
					Kind:      profile.OutKindCmdline,
					OutFormat: profile.OutFormatRaw,
				},
				Version: "1.11.0",
			},

			expected: "talos.platform=metal console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on selinux=1", //nolint:lll
		},
		{
			name: "cmdline-1.11-arm64",

			prof: profile.Profile{
				BaseProfileName: "metal",
				Arch:            "arm64",
				Output: profile.Output{
					Kind:      profile.OutKindCmdline,
					OutFormat: profile.OutFormatRaw,
				},
				Version: "1.11.0",
			},

			expected: "talos.platform=metal console=ttyAMA0 console=tty0 init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on selinux=1", //nolint:lll
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			t.Cleanup(cancel)

			imgr, err := imager.New(test.prof)
			require.NoError(t, err)

			outPath := t.TempDir()

			outputPath, err := imgr.Execute(ctx, outPath, reporter.New())
			require.NoError(t, err)

			out, err := os.ReadFile(outputPath)
			require.NoError(t, err)

			assert.Equal(t, test.expected, string(out))
		})
	}
}
