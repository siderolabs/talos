// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig

import (
	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// Builder is a small utility to build spec-modifying functions
// that can be applied (inside `safe.WriterModify`) to a VolumeConfigSpec.
//
// The builder is just a wrapper around a slice `func(*VolumeConfigSpec) error`, and the
// `.WithXXX` methods only append to the slice. No modifications are made to the spec until
// `Apply()` is called, or the function returned by `WriterFunc()` is called.
//
// Errors that occur during application are collected into a multierror and returned
// by `Apply`/`WriterFunc`.
type Builder struct {
	opts []func(*block.VolumeConfigSpec) error
}

// NewBuilder creates a new VolumeConfigBuilder.
func NewBuilder() *Builder {
	return &Builder{
		opts: nil,
	}
}

// WithType sets VolumeConfigSpec.Type.
func (b *Builder) WithType(volumeType block.VolumeType) *Builder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		spec.Type = volumeType

		return nil
	})

	return b
}

// WithLocator sets VolumeConfigSpec.Locator.
func (b *Builder) WithLocator(match cel.Expression) *Builder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		spec.Locator = block.LocatorSpec{Match: match}

		return nil
	})

	return b
}

// WithProvisioning sets VolumeConfigSpec.Provisioning.
func (b *Builder) WithProvisioning(provisioning block.ProvisioningSpec) *Builder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		spec.Provisioning = provisioning

		return nil
	})

	return b
}

// WithMount sets VolumeConfigSpec.Mount.
func (b *Builder) WithMount(mount block.MountSpec) *Builder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		spec.Mount = mount

		return nil
	})

	return b
}

// WithSymlink sets VolumeConfigSpec.Symlink.
func (b *Builder) WithSymlink(symlink block.SymlinkProvisioningSpec) *Builder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		spec.Symlink = symlink

		return nil
	})

	return b
}

// WithParentID sets VolumeConfigSpec.ParentID.
func (b *Builder) WithParentID(parentID string) *Builder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		spec.ParentID = parentID

		return nil
	})

	return b
}

// WithEncryption sets VolumeConfigSpec.Encryption.
func (b *Builder) WithEncryption(encryption block.EncryptionSpec) *Builder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		spec.Encryption = encryption

		return nil
	})

	return b
}

// WithConvertEncryptionConfiguration sets VolumeConfigSpec.Encryption, converting the provided
// encryption config.
func (b *Builder) WithConvertEncryptionConfiguration(encryption configconfig.EncryptionConfig) *Builder {
	b.opts = append(b.opts, func(spec *block.VolumeConfigSpec) error {
		return volumes.ConvertEncryptionConfiguration(encryption, spec)
	})

	return b
}

// WithFunc adds an arbitraty spec-modifying `func(*block.VolumeConfigSpec) error` to the builder.
// Errors returned by the function are collected and returned by Apply/WriterFunc.
func (b *Builder) WithFunc(fn func(*block.VolumeConfigSpec) error) *Builder {
	b.opts = append(b.opts, fn)

	return b
}

// Apply applies all the changes to the provided VolumeConfigSpec.
// Returns a multierror containing all errors that occurred during application.
func (b *Builder) Apply(spec *block.VolumeConfigSpec) error {
	var errors *multierror.Error

	for _, opt := range b.opts {
		if err := opt(spec); err != nil {
			errors = multierror.Append(errors, err)
		}
	}

	return errors.ErrorOrNil()
}

// WriterFunc returns a function that applies all the changes to the provided VolumeConfig.
// The function returned is suitable for use in `safe.WriterModify`, and returns a multierror
// containing all errors that occurred during application.
func (b *Builder) WriterFunc() func(*block.VolumeConfig) error {
	return func(vc *block.VolumeConfig) error {
		return b.Apply(vc.TypedSpec())
	}
}
