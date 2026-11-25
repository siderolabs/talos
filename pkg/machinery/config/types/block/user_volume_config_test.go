// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl,goconst
package block_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestUserVolumeConfigMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		filename string
		cfg      func(t *testing.T) *block.UserVolumeConfigV1Alpha1
	}{
		{
			name:     "disk selector",
			filename: "uservolumeconfig_diskselector.yaml",
			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = "ceph-data"

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.transport == "nvme" && !system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("100GiB")
				c.FilesystemSpec.FilesystemType = blockres.FilesystemTypeXFS

				return c
			},
		},
		{
			name:     "encrypted",
			filename: "uservolumeconfig_encrypted.yaml",
			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = "secret-store"

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`!system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.EncryptionSpec.EncryptionProvider = blockres.EncryptionProviderLUKS2
				c.EncryptionSpec.EncryptionCipher = "aes-xts-plain64"
				c.EncryptionSpec.EncryptionKeys = []block.EncryptionKey{
					{
						KeySlot: 0,
						KeyTPM:  &block.EncryptionKeyTPM{},
					},
					{
						KeySlot: 1,
						KeyStatic: &block.EncryptionKeyStatic{
							KeyData: "topsecret",
						},
					},
				}

				return c
			},
		},
		{
			name:     "prjquota",
			filename: "uservolumeconfig_prjquota.yaml",
			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = "secret-store"

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`!system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.FilesystemSpec.FilesystemType = blockres.FilesystemTypeXFS
				c.FilesystemSpec.ProjectQuotaSupportConfig = new(true)

				return c
			},
		},
		{
			name:     "xfs min allocation group size",
			filename: "uservolumeconfig_xfs.yaml",
			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = "build-cache"

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`!system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.FilesystemSpec.XFSSpec = &block.XFSSpec{
					MinAllocationGroupSizeConfig: block.MustByteSize("128GiB"),
				}

				return c
			},
		},
		{
			name:     "tmpfs",
			filename: "uservolumeconfig_tmpfs.yaml",
			cfg: func(*testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = "scratch"
				c.VolumeType = new(blockres.VolumeTypeTmpfs)
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("64MiB")

				return c
			},
		},
		{
			name:     "mount ownership",
			filename: "uservolumeconfig_ownership.yaml",
			cfg: func(*testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = "shared"
				c.VolumeType = new(blockres.VolumeTypeDirectory)
				c.MountSpec.MountOwner = new(uint32(999))
				c.MountSpec.MountGroup = new(uint32(2000))
				c.MountSpec.MountMode = new(block.UserVolumeMountMode(0o2770))

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

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

func TestUserVolumeConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(t *testing.T) *block.UserVolumeConfigV1Alpha1

		expectedErrors   string
		expectedWarnings []string
	}{
		{
			name: "no name",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 1u`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("2.5TiB")

				return c
			},

			expectedErrors: "name is required\nname must be between 1 and 34 characters long",
		},
		{
			name: "too long name",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = strings.Repeat("X", 35)

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 1u`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("2.5TiB")

				return c
			},

			expectedErrors: "name must be between 1 and 34 characters long",
		},
		{
			name: "invalid characters in name",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = "some/name"

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 1u`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("2.5TiB")

				return c
			},

			expectedErrors: "name can only contain lowercase and uppercase ASCII letters, digits, and hyphens",
		},
		{
			name: "invalid disk selector",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 120`)))

				return c
			},

			expectedErrors: "disk selector is invalid: ERROR: <input>:1:11: found no matching overload for '_>_' applied to '(uint, int)'\n | disk.size > 120\n | ..........^\nmin size or max size is required",
		},
		{
			name: "min size greater than max size",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("2.5TiB")
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("10GiB")

				return c
			},

			expectedErrors: "disk selector is required\nmin size is greater than max size",
		},
		{
			name: "unsupported filesystem type",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 120u * GiB`)))
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("2.5TiB")
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.FilesystemSpec.FilesystemType = blockres.FilesystemTypeISO9660

				return c
			},

			expectedErrors: "unsupported filesystem type: iso9660",
		},
		{
			name: "no encryption provider",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.EncryptionSpec.EncryptionKeys = []block.EncryptionKey{
					{
						KeySlot: 0,
						KeyTPM:  &block.EncryptionKeyTPM{},
					},
				}

				return c
			},

			expectedErrors: "unsupported encryption provider: none",
		},
		{
			name: "no encryption keys",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.EncryptionSpec.EncryptionProvider = blockres.EncryptionProviderLUKS2

				return c
			},

			expectedErrors: "encryption keys are required",
		},
		{
			name: "invalid encryption key slots",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.EncryptionSpec.EncryptionProvider = blockres.EncryptionProviderLUKS2
				c.EncryptionSpec.EncryptionKeys = []block.EncryptionKey{
					{
						KeySlot: 1,
						KeyTPM:  &block.EncryptionKeyTPM{},
					},
					{
						KeySlot: 0,
					},
					{
						KeySlot: 1,
						KeyTPM:  &block.EncryptionKeyTPM{},
					},
				}

				return c
			},

			expectedErrors: "at least one encryption key type must be specified for slot 0\nduplicate key slot 1",
		},
		{
			name: "prjquota not supported",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.FilesystemSpec.FilesystemType = blockres.FilesystemTypeEXT4
				c.FilesystemSpec.ProjectQuotaSupportConfig = new(true)

				return c
			},

			expectedErrors: "project quota support is only available for xfs filesystem",
		},
		{
			name: "xfs options not supported",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.FilesystemSpec.FilesystemType = blockres.FilesystemTypeEXT4
				c.FilesystemSpec.XFSSpec = &block.XFSSpec{
					MinAllocationGroupSizeConfig: block.MustByteSize("128GiB"),
				}

				return c
			},

			expectedErrors: "xfs options are only available for xfs filesystem",
		},
		{
			name: "provisioning spec for directory",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.VolumeType = new(blockres.VolumeTypeDirectory)

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")

				return c
			},

			expectedErrors: "provisioning spec is invalid for volumeType directory",
		},
		{
			name: "encryption spec for directory",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.VolumeType = new(blockres.VolumeTypeDirectory)

				c.EncryptionSpec.EncryptionProvider = blockres.EncryptionProviderLUKS2
				c.EncryptionSpec.EncryptionCipher = "aes-xts-plain64"
				c.EncryptionSpec.EncryptionKeys = []block.EncryptionKey{
					{
						KeySlot: 0,
						KeyTPM:  &block.EncryptionKeyTPM{},
					},
				}

				return c
			},

			expectedErrors: "encryption spec is invalid for volumeType directory",
		},
		{
			name: "size for disk",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.VolumeType = new(blockres.VolumeTypeDisk)

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 120u * GiB`)))
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("2.5TiB")
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.FilesystemSpec.FilesystemType = blockres.FilesystemTypeEXT4

				return c
			},

			expectedErrors: "min size and max size are not supported",
		},
		{
			name: "filesystem spec for directory",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.VolumeType = new(blockres.VolumeTypeDirectory)

				c.FilesystemSpec.FilesystemType = blockres.FilesystemTypeVFAT

				return c
			},

			expectedErrors: "filesystem spec is invalid for volumeType directory",
		},
		{
			name: "secure mount for directory",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.VolumeType = new(blockres.VolumeTypeDirectory)
				c.MountSpec.MountSecure = new(false)

				return c
			},
		},
		{
			name: "disable access time for directory",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.VolumeType = new(blockres.VolumeTypeDirectory)
				c.MountSpec.MountDisableAccessTime = new(true)

				return c
			},

			expectedErrors: "mount.disableAccessTime is invalid for volumeType directory",
		},
		{
			name: "mount ownership for directory",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.VolumeType = new(blockres.VolumeTypeDirectory)
				c.MountSpec.MountOwner = new(uint32(1000))
				c.MountSpec.MountGroup = new(uint32(2000))
				c.MountSpec.MountMode = new(block.UserVolumeMountMode(0o2770))

				return c
			},
		},
		{
			name: "mount ownership for tmpfs",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.VolumeType = new(blockres.VolumeTypeTmpfs)
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("64MiB")
				c.MountSpec.MountOwner = new(uint32(1000))
				c.MountSpec.MountMode = new(block.UserVolumeMountMode(0o700))

				return c
			},
		},
		{
			name: "invalid mount ownership",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.VolumeType = new(blockres.VolumeTypeDirectory)
				c.MountSpec.MountOwner = new(uint32(4294967295))
				c.MountSpec.MountGroup = new(uint32(4294967295))
				c.MountSpec.MountMode = new(block.UserVolumeMountMode(0o10755))

				return c
			},

			expectedErrors: "mount.owner must be at most 4294967294, got 4294967295\n" +
				"mount.group must be at most 4294967294, got 4294967295\n" +
				"mount.mode 0o10755 may only contain permission, setuid, setgid and sticky bits",
		},
		{
			name: "invalid volumeType",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.VolumeType = new(blockres.VolumeTypeOverlay)

				return c
			},

			expectedErrors: "unsupported volume type \"overlay\"",
		},
		{
			name: "tmpfs without size",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = "scratch"
				c.VolumeType = new(blockres.VolumeTypeTmpfs)

				return c
			},

			expectedErrors: "provisioning.maxSize is required for volumeType tmpfs",
		},
		{
			name: "tmpfs with negative size",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = "scratch"
				c.VolumeType = new(blockres.VolumeTypeTmpfs)
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("-64MiB")

				return c
			},

			expectedErrors: "provisioning.maxSize cannot be negative for volumeType tmpfs",
		},
		{
			name: "tmpfs with disk provisioning",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = "scratch"
				c.VolumeType = new(blockres.VolumeTypeTmpfs)

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10MiB")
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("64MiB")
				c.ProvisioningSpec.ProvisioningGrow = new(true)

				return c
			},

			expectedErrors: "provisioning.minSize is invalid for volumeType tmpfs\n" +
				"provisioning.grow is invalid for volumeType tmpfs\n" +
				"provisioning.diskSelector is invalid for volumeType tmpfs",
		},
		{
			name: "tmpfs with filesystem, encryption, trim and scrub",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = "scratch"
				c.VolumeType = new(blockres.VolumeTypeTmpfs)
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("64MiB")

				c.FilesystemSpec.FilesystemType = blockres.FilesystemTypeXFS
				c.EncryptionSpec.EncryptionProvider = blockres.EncryptionProviderLUKS2
				c.EncryptionSpec.EncryptionKeys = []block.EncryptionKey{
					{
						KeySlot: 0,
						KeyTPM:  &block.EncryptionKeyTPM{},
					},
				}
				c.TrimSpec = &block.TrimConfig{TrimEnabled: new(true)}
				c.ScrubSpec = &block.ScrubConfig{ScrubEnabled: new(true)}

				return c
			},

			expectedErrors: "encryption spec is invalid for volumeType tmpfs\n" +
				"filesystem spec is invalid for volumeType tmpfs\n" +
				"trim spec is invalid for volumeType tmpfs\n" +
				"scrub spec is invalid for volumeType tmpfs",
		},
		{
			name: "valid tmpfs",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = "scratch"
				c.VolumeType = new(blockres.VolumeTypeTmpfs)
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("64MiB")
				c.MountSpec.MountSecure = new(false)
				c.MountSpec.MountDisableAccessTime = new(true)

				return c
			},
		},
		{
			name: "valid tmpfs with a relative size",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = "scratch"
				c.VolumeType = new(blockres.VolumeTypeTmpfs)
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("10%")

				return c
			},
		},
		{
			name: "tmpfs at the kernel default size",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = "scratch"
				c.VolumeType = new(blockres.VolumeTypeTmpfs)
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("50%")

				return c
			},
		},
		{
			name: "tmpfs above the kernel default size",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = "scratch"
				c.VolumeType = new(blockres.VolumeTypeTmpfs)
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("100%")

				return c
			},

			expectedWarnings: []string{
				"provisioning.maxSize 100% for volumeType tmpfs is above 50% of the memory: tmpfs memory is not freed by the OOM killer and can only be swapped out, so it might exhaust the memory",
			},
		},
		{
			name: "valid",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 120u * GiB`)))
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("2.5TiB")
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.FilesystemSpec.FilesystemType = blockres.FilesystemTypeEXT4

				return c
			},
		},
		{
			name: "valid partition",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.VolumeType = new(blockres.VolumeTypePartition)

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 120u * GiB`)))
				c.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("2.5TiB")
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.FilesystemSpec.FilesystemType = blockres.FilesystemTypeEXT4

				return c
			},
		},
		{
			name: "valid directory",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.VolumeType = new(blockres.VolumeTypeDirectory)

				return c
			},
		},
		{
			name: "valid disk",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.VolumeType = new(blockres.VolumeTypeDisk)

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.size > 120u * GiB`)))
				c.FilesystemSpec.FilesystemType = blockres.FilesystemTypeEXT4

				return c
			},
		},
		{
			name: "valid encrypted",

			cfg: func(t *testing.T) *block.UserVolumeConfigV1Alpha1 {
				c := block.NewUserVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel

				require.NoError(t, c.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`system_disk`)))
				c.ProvisioningSpec.ProvisioningMinSize = block.MustByteSize("10GiB")
				c.EncryptionSpec.EncryptionProvider = blockres.EncryptionProviderLUKS2
				c.EncryptionSpec.EncryptionCipher = "aes-xts-plain64"
				c.EncryptionSpec.EncryptionKeys = []block.EncryptionKey{
					{
						KeySlot: 0,
						KeyTPM:  &block.EncryptionKeyTPM{},
					},
				}

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			warnings, err := cfg.Validate(validationMode{})

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedErrors == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)

				assert.EqualError(t, err, test.expectedErrors)
			}
		})
	}
}

func TestUserVolumeConfigUnmarshalMode(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		mode string

		expectedMode  block.UserVolumeMountMode
		expectedError string
	}{
		{mode: "0o2770", expectedMode: 0o2770},
		{mode: "02770", expectedMode: 0o2770},
		{mode: "0o755", expectedMode: 0o755},
		{mode: "0", expectedMode: 0},
		{mode: "2770", expectedError: `mount.mode must be an octal number such as 0o2770, got "2770"`},
		{mode: "755", expectedError: `mount.mode must be an octal number such as 0o2770, got "755"`},
		{mode: "0x1ed", expectedError: `mount.mode must be an octal number such as 0o2770, got "0x1ed"`},
		{mode: "0o9", expectedError: `mount.mode must be an octal number such as 0o2770, got "0o9"`},
		{mode: `"0o755"`, expectedError: `mount.mode must be an octal number such as 0o2770, got "0o755"`},
	} {
		t.Run(test.mode, func(t *testing.T) {
			t.Parallel()

			provider, err := configloader.NewFromBytes([]byte(`apiVersion: v1alpha1
kind: UserVolumeConfig
name: shared
volumeType: directory
mount:
    mode: ` + test.mode + "\n"))

			if test.expectedError != "" {
				require.ErrorContains(t, err, test.expectedError)

				return
			}

			require.NoError(t, err)

			docs := provider.Documents()
			require.Len(t, docs, 1)

			cfg, ok := docs[0].(*block.UserVolumeConfigV1Alpha1)
			require.True(t, ok)
			require.NotNil(t, cfg.MountSpec.MountMode)

			assert.Equal(t, test.expectedMode, *cfg.MountSpec.MountMode)
		})
	}
}

func TestUserMountSpecOwnership(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		spec block.UserMountSpec

		expectedOwner uint32
		expectedGroup uint32
		expectedMode  fs.FileMode
	}{
		{
			name: "defaults",

			expectedMode: 0o755,
		},
		{
			name: "permission bits",

			spec: block.UserMountSpec{
				MountOwner: new(uint32(1000)),
				MountGroup: new(uint32(2000)),
				MountMode:  new(block.UserVolumeMountMode(0o750)),
			},

			expectedOwner: 1000,
			expectedGroup: 2000,
			expectedMode:  0o750,
		},
		{
			name: "IDs above 2^31",

			spec: block.UserMountSpec{
				MountOwner: new(uint32(3000000000)),
				MountGroup: new(uint32(4294967294)),
			},

			expectedOwner: 3000000000,
			expectedGroup: 4294967294,
			expectedMode:  0o755,
		},
		{
			name: "setgid",

			spec: block.UserMountSpec{
				MountMode: new(block.UserVolumeMountMode(0o2770)),
			},

			expectedMode: fs.ModeSetgid | 0o770,
		},
		{
			name: "setuid and sticky",

			spec: block.UserMountSpec{
				MountMode: new(block.UserVolumeMountMode(0o5777)),
			},

			expectedMode: fs.ModeSetuid | fs.ModeSticky | 0o777,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expectedOwner, test.spec.Owner())
			assert.Equal(t, test.expectedGroup, test.spec.Group())
			assert.Equal(t, test.expectedMode, test.spec.Mode())
		})
	}
}
