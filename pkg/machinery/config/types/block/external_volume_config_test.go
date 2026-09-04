// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl,goconst
package block_test

import (
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

func TestExternalVolumeConfigMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		filename string
		cfg      func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1
	}{
		{
			name:     "basic virtiofs",
			filename: "externalvolumeconfig_basicvirtiofs.yaml",
			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = "my-virtiofs-volume"
				c.FilesystemType = blockres.FilesystemTypeVirtiofs
				c.MountSpec.MountVirtiofs = new(block.VirtiofsMountSpec)
				c.MountSpec.MountVirtiofs.VirtiofsTag = "Data"

				return c
			},
		},
		{
			name:     "basic NFSv3",
			filename: "externalvolumeconfig_basicnfsv3.yaml",
			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				retransmissions := uint32(2)
				reservedPort := false

				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = "my-nfs-volume"
				c.FilesystemType = blockres.FilesystemTypeNFS
				c.MountSpec.MountNFS = &block.NFSMountSpec{
					NFSServer:          "10.5.0.1",
					NFSPath:            "/export",
					NFSVersion:         blockres.NFSVersion3,
					NFSPort:            12049,
					NFSTransport:       new(blockres.NFSTransportTCP),
					NFSMountPort:       12049,
					NFSMountTransport:  new(blockres.NFSTransportTCP),
					NFSLocking:         new(blockres.NFSLockingRemote),
					NFSRecovery:        new(blockres.NFSRecoverySoftError),
					NFSTimeout:         600,
					NFSRetransmissions: &retransmissions,
					NFSReadSize:        1048576,
					NFSWriteSize:       1048576,
					NFSConnections:     4,
					NFSReservedPort:    &reservedPort,
					NFSSecurity:        new(blockres.NFSSecuritySys),
				}

				return c
			},
		},
		{
			name:     "basic NFSv4",
			filename: "externalvolumeconfig_basicnfsv4.yaml",
			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = "my-nfs-volume"
				c.FilesystemType = blockres.FilesystemTypeNFS
				c.MountSpec.MountNFS = &block.NFSMountSpec{
					NFSServer:  "fd00::1",
					NFSPath:    "/export",
					NFSVersion: blockres.NFSVersion4Point1,
				}

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

func TestNFSMountSpec(t *testing.T) {
	t.Parallel()

	retransmissions := uint32(2)
	reservedPort := false

	spec := &block.NFSMountSpec{
		NFSServer:          "fd00::1",
		NFSPath:            "/export",
		NFSVersion:         blockres.NFSVersion3,
		NFSPort:            12049,
		NFSTransport:       new(blockres.NFSTransportTCP6),
		NFSMountPort:       12050,
		NFSMountTransport:  new(blockres.NFSTransportTCP6),
		NFSLocking:         new(blockres.NFSLockingRemote),
		NFSRecovery:        new(blockres.NFSRecoverySoftError),
		NFSTimeout:         600,
		NFSRetransmissions: &retransmissions,
		NFSReadSize:        1048576,
		NFSWriteSize:       1048576,
		NFSConnections:     4,
		NFSReservedPort:    &reservedPort,
		NFSSecurity:        new(blockres.NFSSecuritySys),
	}

	assert.Equal(t, "[fd00::1]:/export", spec.Source())
	parameters, err := spec.Parameters()
	require.NoError(t, err)
	assert.Equal(t, []blockres.ParameterSpec{
		blockres.NewStringParameter("vers", "3"),
		blockres.NewStringParameter("port", "12049"),
		blockres.NewStringParameter("proto", "tcp6"),
		blockres.NewStringParameter("mountport", "12050"),
		blockres.NewStringParameter("mountproto", "tcp6"),
		blockres.NewBooleanParameter("lock"),
		blockres.NewBooleanParameter("softerr"),
		blockres.NewStringParameter("timeo", "600"),
		blockres.NewStringParameter("retrans", "2"),
		blockres.NewStringParameter("rsize", "1048576"),
		blockres.NewStringParameter("wsize", "1048576"),
		blockres.NewStringParameter("nconnect", "4"),
		blockres.NewBooleanParameter("noresvport"),
		blockres.NewStringParameter("sec", "sys"),
	}, parameters)
}

func TestNFSVersions(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		version blockres.NFSVersion
		params  []blockres.ParameterSpec
	}{
		{
			version: blockres.NFSVersion3,
			params: []blockres.ParameterSpec{
				blockres.NewStringParameter("vers", "3"),
				blockres.NewBooleanParameter("nolock"),
			},
		},
		{
			version: blockres.NFSVersion4,
			params:  []blockres.ParameterSpec{blockres.NewStringParameter("vers", "4")},
		},
		{
			version: blockres.NFSVersion4Point1,
			params:  []blockres.ParameterSpec{blockres.NewStringParameter("vers", "4.1")},
		},
		{
			version: blockres.NFSVersion4Point2,
			params:  []blockres.ParameterSpec{blockres.NewStringParameter("vers", "4.2")},
		},
	} {
		t.Run(test.version.String(), func(t *testing.T) {
			t.Parallel()

			spec := &block.NFSMountSpec{
				NFSServer:  "nfs.example.com",
				NFSPath:    "/export",
				NFSVersion: test.version,
			}

			warnings, err := spec.Validate()
			require.NoError(t, err)
			require.Empty(t, warnings)

			params, err := spec.Parameters()
			require.NoError(t, err)
			assert.Equal(t, test.params, params)
		})
	}
}

func TestExternalVolumeConfigDeepCopy(t *testing.T) {
	t.Parallel()

	cfg := block.NewExternalVolumeConfigV1Alpha1()
	cfg.MountSpec.MountNFS = &block.NFSMountSpec{
		NFSLocking:  new(blockres.NFSLockingRemote),
		NFSRecovery: new(blockres.NFSRecoverySoft),
		NFSSecurity: new(blockres.NFSSecuritySys),
	}

	copied := cfg.DeepCopy()

	require.NotSame(t, cfg.MountSpec.MountNFS, copied.MountSpec.MountNFS)
	require.NotSame(t, cfg.MountSpec.MountNFS.NFSLocking, copied.MountSpec.MountNFS.NFSLocking)
	require.NotSame(t, cfg.MountSpec.MountNFS.NFSRecovery, copied.MountSpec.MountNFS.NFSRecovery)
	require.NotSame(t, cfg.MountSpec.MountNFS.NFSSecurity, copied.MountSpec.MountNFS.NFSSecurity)

	*copied.MountSpec.MountNFS.NFSLocking = blockres.NFSLockingLocal
	*copied.MountSpec.MountNFS.NFSRecovery = blockres.NFSRecoveryHard
	*copied.MountSpec.MountNFS.NFSSecurity = blockres.NFSSecurityNone

	assert.Equal(t, blockres.NFSLockingRemote, *cfg.MountSpec.MountNFS.NFSLocking)
	assert.Equal(t, blockres.NFSRecoverySoft, *cfg.MountSpec.MountNFS.NFSRecovery)
	assert.Equal(t, blockres.NFSSecuritySys, *cfg.MountSpec.MountNFS.NFSSecurity)
}

func TestExternalVolumeConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1

		expectedErrors string
	}{
		{
			name: "no name",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.FilesystemType = blockres.FilesystemTypeVirtiofs
				c.MountSpec.MountVirtiofs = new(block.VirtiofsMountSpec)
				c.MountSpec.MountVirtiofs.VirtiofsTag = "Data"

				return c
			},

			expectedErrors: "name is required\nname must be between 1 and 34 characters long",
		},
		{
			name: "invalid characters in name",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = "some/name"
				c.FilesystemType = blockres.FilesystemTypeVirtiofs
				c.MountSpec.MountVirtiofs = new(block.VirtiofsMountSpec)
				c.MountSpec.MountVirtiofs.VirtiofsTag = "Data"

				return c
			},

			expectedErrors: "name can only contain lowercase and uppercase ASCII letters, digits, and hyphens",
		},
		{
			name: "no mount spec",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.FilesystemType = blockres.FilesystemTypeVirtiofs

				return c
			},

			expectedErrors: "virtiofs mount spec is required",
		},
		{
			name: "invalid type",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.FilesystemType = blockres.FilesystemTypeEXT4
				c.MountSpec.MountVirtiofs = new(block.VirtiofsMountSpec)
				c.MountSpec.MountVirtiofs.VirtiofsTag = "Data"

				return c
			},

			expectedErrors: "invalid filesystem type: ext4",
		},
		{
			name: "empty type",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.MountSpec.MountVirtiofs = new(block.VirtiofsMountSpec)
				c.MountSpec.MountVirtiofs.VirtiofsTag = "Data"

				return c
			},

			expectedErrors: "invalid filesystem type: none",
		},
		{
			name: "valid virtiofs",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = constants.EphemeralPartitionLabel
				c.FilesystemType = blockres.FilesystemTypeVirtiofs
				c.MountSpec.MountVirtiofs = new(block.VirtiofsMountSpec)
				c.MountSpec.MountVirtiofs.VirtiofsTag = "Data"

				return c
			},
		},
		{
			name: "NFS spec is required",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = "nfs"
				c.FilesystemType = blockres.FilesystemTypeNFS

				return c
			},

			expectedErrors: "NFS mount spec is required",
		},
		{
			name: "invalid NFS spec",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = "nfs"
				c.FilesystemType = blockres.FilesystemTypeNFS
				c.MountSpec.MountNFS = &block.NFSMountSpec{NFSVersion: blockres.NFSVersion(99)}

				return c
			},

			expectedErrors: "NFS server is required\nNFS path is required\nNFS version must be one of 3, 4, 4.1, or 4.2",
		},
		{
			name: "invalid typed NFS parameters",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = "nfs4"
				c.FilesystemType = blockres.FilesystemTypeNFS
				c.MountSpec.MountNFS = &block.NFSMountSpec{
					NFSServer:         "nfs.example.com",
					NFSPath:           "/export",
					NFSVersion:        blockres.NFSVersion4Point1,
					NFSTransport:      new(blockres.NFSTransportUDP),
					NFSMountPort:      12049,
					NFSMountTransport: new(blockres.NFSTransportTCP),
					NFSLocking:        new(blockres.NFSLockingRemote),
					NFSRecovery:       new(blockres.NFSRecovery(99)),
					NFSReadSize:       1000,
					NFSWriteSize:      1048577,
					NFSConnections:    17,
					NFSSecurity:       new(blockres.NFSSecurity(99)),
				}

				return c
			},

			expectedErrors: strings.Join([]string{
				"NFSv4 transport must be tcp or tcp6",
				"NFS mount port is only valid with NFSv3",
				"NFS mount transport is only valid with NFSv3",
				"NFS locking is only configurable with NFSv3",
				"NFS recovery must be one of hard, soft, or soft-error",
				"NFS read size must be a multiple of 1024 between 1024 and 1048576",
				"NFS write size must be a multiple of 1024 between 1024 and 1048576",
				"NFS connections must be between 1 and 16",
				"NFS connections require TCP transport",
				"NFS security must be either none or sys",
			}, "\n"),
		},
		{
			name: "mismatched NFS transport address families",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = "nfs3"
				c.FilesystemType = blockres.FilesystemTypeNFS
				c.MountSpec.MountNFS = &block.NFSMountSpec{
					NFSServer:         "nfs.example.com",
					NFSPath:           "/export",
					NFSVersion:        blockres.NFSVersion3,
					NFSTransport:      new(blockres.NFSTransportTCP),
					NFSMountTransport: new(blockres.NFSTransportTCP6),
				}

				return c
			},

			expectedErrors: "NFS mount transport address family must match NFS transport address family",
		},
		{
			name: "valid NFSv3",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = "nfs3"
				c.FilesystemType = blockres.FilesystemTypeNFS
				c.MountSpec.MountNFS = &block.NFSMountSpec{NFSServer: "10.5.0.1", NFSPath: "/export", NFSVersion: blockres.NFSVersion3}

				return c
			},
		},
		{
			name: "valid NFSv4",

			cfg: func(t *testing.T) *block.ExternalVolumeConfigV1Alpha1 {
				c := block.NewExternalVolumeConfigV1Alpha1()
				c.MetaName = "nfs4"
				c.FilesystemType = blockres.FilesystemTypeNFS
				c.MountSpec.MountNFS = &block.NFSMountSpec{NFSServer: "nfs.example.com", NFSPath: "/export", NFSVersion: blockres.NFSVersion4}

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			_, err := cfg.Validate(validationMode{})

			if test.expectedErrors == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)

				assert.EqualError(t, err, test.expectedErrors)
			}
		})
	}
}
