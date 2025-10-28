// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type volumeConfigBuilder struct {
	opts []func(*block.VolumeConfigSpec) error
}

func newVolumeConfigBuilder() *volumeConfigBuilder {
	return &volumeConfigBuilder{
		opts: nil,
	}
}

func (b *volumeConfigBuilder) WithType(volumeType block.VolumeType) *volumeConfigBuilder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		spec.Type = volumeType

		return nil
	})

	return b
}

func (b *volumeConfigBuilder) WithLocator(match cel.Expression) *volumeConfigBuilder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		spec.Locator = block.LocatorSpec{Match: match}

		return nil
	})

	return b
}

func (b *volumeConfigBuilder) WithProvisioning(provisioning block.ProvisioningSpec) *volumeConfigBuilder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		spec.Provisioning = provisioning

		return nil
	})

	return b
}

func (b *volumeConfigBuilder) WithMount(mount block.MountSpec) *volumeConfigBuilder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		spec.Mount = mount

		return nil
	})

	return b
}

func (b *volumeConfigBuilder) WithEncryption(encryption block.EncryptionSpec) *volumeConfigBuilder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		spec.Encryption = encryption

		return nil
	})

	return b
}

func (b *volumeConfigBuilder) WithSymlink(symlink block.SymlinkProvisioningSpec) *volumeConfigBuilder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		spec.Symlink = symlink

		return nil
	})

	return b
}

func (b *volumeConfigBuilder) WithParentID(parentID string) *volumeConfigBuilder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		spec.ParentID = parentID

		return nil
	})

	return b
}

func (b *volumeConfigBuilder) WithConvertEncryptionConfiguration(encryption configconfig.EncryptionConfig) *volumeConfigBuilder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		return convertEncryptionConfiguration(encryption, spec)
	})

	return b
}

func (b *volumeConfigBuilder) WithFunc(fn func(*block.VolumeConfigSpec) error) *volumeConfigBuilder {
	b.opts = append(b.opts, fn)

	return b
}

func (b *volumeConfigBuilder) Apply(spec *block.VolumeConfigSpec) error {
	var errors *multierror.Error

	for _, opt := range b.opts {
		if err := opt(spec); err != nil {
			errors = multierror.Append(errors, err)
		}
	}

	return errors.ErrorOrNil()
}

func (b *volumeConfigBuilder) WriterFunc() func(*block.VolumeConfig) error {
	return func(vc *block.VolumeConfig) error {
		return b.Apply(vc.TypedSpec())
	}
}
