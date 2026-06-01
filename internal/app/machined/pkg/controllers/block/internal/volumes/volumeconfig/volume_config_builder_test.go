// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig_test

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes/volumeconfig"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestNewVolumeConfigBuilder(t *testing.T) {
	t.Parallel()

	builder := volumeconfig.NewBuilder()
	require.NotNil(t, builder)

	spec := &block.VolumeConfigSpec{}
	err := builder.Apply(spec)
	require.NoError(t, err)
}

func TestVolumeConfigBuilder_EmptyBuilder(t *testing.T) {
	t.Parallel()

	spec := &block.VolumeConfigSpec{}
	builder := volumeconfig.NewBuilder()

	err := builder.Apply(spec)
	require.NoError(t, err)

	// spec should remain empty/default
	assert.Equal(t, block.VolumeType(0), spec.Type)
	assert.Empty(t, spec.ParentID)
	assert.Empty(t, spec.Mount)
	assert.Empty(t, spec.Provisioning)
}

func TestVolumeConfigBuilder_WithType(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		volumeType block.VolumeType
	}{
		{
			name:       "partition",
			volumeType: block.VolumeTypePartition,
		},
		{
			name:       "directory",
			volumeType: block.VolumeTypeDirectory,
		},
		{
			name:       "overlay",
			volumeType: block.VolumeTypeOverlay,
		},
		{
			name:       "symlink",
			volumeType: block.VolumeTypeSymlink,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec := &block.VolumeConfigSpec{}
			builder := volumeconfig.NewBuilder().WithType(tc.volumeType)

			err := builder.Apply(spec)
			require.NoError(t, err)

			assert.Equal(t, tc.volumeType, spec.Type)
		})
	}
}

func TestVolumeConfigBuilder_WithLocator(t *testing.T) {
	t.Parallel()

	match := cel.MustExpression(cel.ParseBooleanExpression(`volume.partition_label == "TEST"`, celenv.VolumeLocator()))
	spec := &block.VolumeConfigSpec{}

	builder := volumeconfig.NewBuilder().WithLocator(match)
	err := builder.Apply(spec)
	require.NoError(t, err)

	assert.Equal(t, match, spec.Locator.Match)
}

func TestVolumeConfigBuilder_WithProvisioning(t *testing.T) {
	t.Parallel()

	provisioning := block.ProvisioningSpec{
		Wave: block.WaveUserVolumes,
		DiskSelector: block.DiskSelector{
			Match: cel.MustExpression(cel.ParseBooleanExpression(`system_disk`, celenv.DiskLocator())),
		},
		PartitionSpec: block.PartitionSpec{
			MinSize:  100 * 1024 * 1024,
			MaxSize:  200 * 1024 * 1024,
			Grow:     true,
			Label:    "test-label",
			TypeUUID: partition.LinuxFilesystemData,
		},
		FilesystemSpec: block.FilesystemSpec{
			Type: block.FilesystemTypeXFS,
		},
	}
	spec := &block.VolumeConfigSpec{}
	builder := volumeconfig.NewBuilder().WithProvisioning(provisioning)

	err := builder.Apply(spec)
	require.NoError(t, err)

	assert.Equal(t, provisioning, spec.Provisioning)
}

func TestVolumeConfigBuilder_WithMount(t *testing.T) {
	t.Parallel()

	mount := block.MountSpec{
		TargetPath:   "/test/path",
		ParentID:     "parent-id",
		SelinuxLabel: "test_label",
		FileMode:     0o755,
		UID:          1000,
		GID:          2000,
	}
	spec := &block.VolumeConfigSpec{}
	builder := volumeconfig.NewBuilder().WithMount(mount)

	err := builder.Apply(spec)
	require.NoError(t, err)

	assert.Equal(t, mount, spec.Mount)
}

func TestVolumeConfigBuilder_WithEncryption(t *testing.T) {
	t.Parallel()

	encryption := block.EncryptionSpec{
		Provider: block.EncryptionProviderLUKS2,
		Keys: []block.EncryptionKey{
			{
				Slot: 0,
				Type: block.EncryptionKeyTPM,
			},
		},
	}
	spec := &block.VolumeConfigSpec{}
	builder := volumeconfig.NewBuilder().WithEncryption(encryption)

	err := builder.Apply(spec)
	require.NoError(t, err)

	assert.Equal(t, encryption, spec.Encryption)
}

func TestVolumeConfigBuilder_WithSymlink(t *testing.T) {
	t.Parallel()

	symlink := block.SymlinkProvisioningSpec{
		SymlinkTargetPath: "/target/path",
		Force:             true,
	}
	spec := &block.VolumeConfigSpec{}
	builder := volumeconfig.NewBuilder().WithSymlink(symlink)

	err := builder.Apply(spec)
	require.NoError(t, err)

	assert.Equal(t, symlink, spec.Symlink)
}

func TestVolumeConfigBuilder_WithParentID(t *testing.T) {
	t.Parallel()

	parentID := "parent-volume-id"
	spec := &block.VolumeConfigSpec{}
	builder := volumeconfig.NewBuilder().WithParentID(parentID)

	err := builder.Apply(spec)
	require.NoError(t, err)

	assert.Equal(t, parentID, spec.ParentID)
}

func TestVolumeConfigBuilder_WithFunc(t *testing.T) {
	t.Parallel()

	customValue := "custom-value"
	spec := &block.VolumeConfigSpec{}
	builder := volumeconfig.NewBuilder().
		WithFunc(func(s *block.VolumeConfigSpec) error {
			s.ParentID = customValue

			return nil
		})

	err := builder.Apply(spec)
	require.NoError(t, err)

	assert.Equal(t, customValue, spec.ParentID)
}

func TestVolumeConfigBuilder_WithFunc_Error(t *testing.T) {
	t.Parallel()

	testErr := errors.New("test error")
	spec := &block.VolumeConfigSpec{}
	builder := volumeconfig.NewBuilder().
		WithFunc(func(s *block.VolumeConfigSpec) error {
			return testErr
		})

	err := builder.Apply(spec)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "test error")
}

func TestVolumeConfigBuilder_WithFunc_MultipleErrors(t *testing.T) {
	t.Parallel()

	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	spec := &block.VolumeConfigSpec{}
	builder := volumeconfig.NewBuilder().
		WithFunc(func(s *block.VolumeConfigSpec) error {
			return err1
		}).
		WithFunc(func(s *block.VolumeConfigSpec) error {
			return err2
		})

	err := builder.Apply(spec)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error 1")
	assert.Contains(t, err.Error(), "error 2")
}

func TestVolumeConfigBuilder_Chaining(t *testing.T) {
	t.Parallel()

	spec := &block.VolumeConfigSpec{}
	builder := volumeconfig.NewBuilder().
		WithType(block.VolumeTypePartition).
		WithParentID("parent-id").
		WithMount(block.MountSpec{
			TargetPath: "/test/path",
			FileMode:   0o755,
		}).
		WithProvisioning(block.ProvisioningSpec{
			Wave: block.WaveUserVolumes,
			PartitionSpec: block.PartitionSpec{
				Label: "test-label",
			},
		})

	err := builder.Apply(spec)
	require.NoError(t, err)

	assert.Equal(t, block.VolumeTypePartition, spec.Type)
	assert.Equal(t, "parent-id", spec.ParentID)
	assert.Equal(t, "/test/path", spec.Mount.TargetPath)
	assert.Equal(t, fs.FileMode(0o755), spec.Mount.FileMode)
	assert.Equal(t, block.WaveUserVolumes, spec.Provisioning.Wave)
	assert.Equal(t, "test-label", spec.Provisioning.PartitionSpec.Label)
}

func TestVolumeConfigBuilder_Chaining_Overwrite(t *testing.T) {
	t.Parallel()

	spec := &block.VolumeConfigSpec{}
	builder := volumeconfig.NewBuilder().
		WithType(block.VolumeTypePartition).
		WithType(block.VolumeTypeDirectory).
		WithParentID("parent-1").
		WithParentID("parent-2")

	err := builder.Apply(spec)
	require.NoError(t, err)

	// Last call should win
	assert.Equal(t, block.VolumeTypeDirectory, spec.Type)
	assert.Equal(t, "parent-2", spec.ParentID)
}

func TestVolumeConfigBuilder_WriterFunc(t *testing.T) {
	t.Parallel()

	vc := block.NewVolumeConfig(block.NamespaceName, "test")
	builder := volumeconfig.NewBuilder().
		WithType(block.VolumeTypePartition).
		WithParentID("parent-id")

	writerFunc := builder.WriterFunc()
	require.NotNil(t, writerFunc)

	err := writerFunc(vc)
	require.NoError(t, err)

	assert.Equal(t, block.VolumeTypePartition, vc.TypedSpec().Type)
	assert.Equal(t, "parent-id", vc.TypedSpec().ParentID)
}

func TestVolumeConfigBuilder_MultipleWithFunc(t *testing.T) {
	t.Parallel()

	spec := &block.VolumeConfigSpec{}
	counter := 0

	builder := volumeconfig.NewBuilder().
		WithFunc(func(s *block.VolumeConfigSpec) error {
			counter++
			s.ParentID = "func1"

			return nil
		}).
		WithFunc(func(s *block.VolumeConfigSpec) error {
			counter++
			s.ParentID = "func2"

			return nil
		}).
		WithFunc(func(s *block.VolumeConfigSpec) error {
			counter++

			return nil
		})

	err := builder.Apply(spec)
	require.NoError(t, err)

	assert.Equal(t, 3, counter)
	assert.Equal(t, "func2", spec.ParentID)
}

func TestVolumeConfigBuilder_WithConvertEncryptionConfiguration_Nil(t *testing.T) {
	t.Parallel()

	builder := volumeconfig.NewBuilder().
		WithConvertEncryptionConfiguration(nil)

	vc := block.NewVolumeConfig(block.NamespaceName, "test")
	err := builder.Apply(vc.TypedSpec())
	require.NoError(t, err)

	assert.Empty(t, vc.TypedSpec().Encryption)
}

func TestVolumeConfigBuilder_WithConvertEncryptionConfiguration_WithConfig(t *testing.T) {
	t.Parallel()

	encryptionConfig := blockcfg.EncryptionSpec{
		EncryptionProvider: block.EncryptionProviderLUKS2,
		EncryptionKeys: []blockcfg.EncryptionKey{
			{
				KeySlot: 0,
				KeyTPM:  &blockcfg.EncryptionKeyTPM{},
			},
		},
	}
	builder := volumeconfig.NewBuilder().
		WithConvertEncryptionConfiguration(encryptionConfig)

	vc := block.NewVolumeConfig(block.NamespaceName, "test")
	err := builder.Apply(vc.TypedSpec())
	require.NoError(t, err)

	assert.Equal(t, block.EncryptionProviderLUKS2, vc.TypedSpec().Encryption.Provider)
	assert.Len(t, vc.TypedSpec().Encryption.Keys, 1)
	assert.Equal(t, 0, vc.TypedSpec().Encryption.Keys[0].Slot)
	assert.Equal(t, block.EncryptionKeyTPM, vc.TypedSpec().Encryption.Keys[0].Type)
}
