// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package volumerequest implements the shared immutable volume-copy request rules.
package volumerequest

import (
	"errors"

	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// ErrOwner identifies a request owned by another producer, including an existing unowned request.
var ErrOwner = errors.New("different request owner")

// ErrDestination identifies an incompatible destination.
var ErrDestination = errors.New("different copy destination")

// Destination names the pool resource destination, independently of its mounted backing.
type Destination struct {
	Pool   string
	Name   string
	Format string
}

// Owned reports whether a producer may withdraw this request.
func Owned(current *storage.StoragePoolVolumeSpec, owner resource.Owner) bool {
	return current.Metadata().Owner() == owner
}

func matches(current *storage.StoragePoolVolumeSpec, destination Destination) bool {
	spec := current.TypedSpec()

	return spec.Pool == destination.Pool && spec.Name == destination.Name && spec.Format == destination.Format && spec.Copy != nil
}

// EnsureAdoption validates an existing image request before initializing source-free copy intent.
// An existing copy request is preserved verbatim.
func EnsureAdoption(current *storage.StoragePoolVolumeSpec, owner resource.Owner, destination Destination) error {
	if !current.Metadata().Version().Equal(resource.VersionUndefined) && !Owned(current, owner) {
		return ErrOwner
	}

	if current.TypedSpec().Pool != "" && !matches(current, destination) {
		return ErrDestination
	}

	if current.TypedSpec().Copy == nil {
		*current.TypedSpec() = storage.StoragePoolVolumeSpecSpec{
			Pool:   destination.Pool,
			Name:   destination.Name,
			Format: destination.Format,
			Copy:   &storage.StoragePoolVolumeCopySpec{},
		}
	}

	return nil
}

// BindImage binds only an unset source. Existing source and digest are retained.
// Image binding intentionally does not recheck the format.
func BindImage(current *storage.StoragePoolVolumeSpec, owner resource.Owner, destination Destination, source, digest string) error {
	if !Owned(current, owner) {
		return ErrOwner
	}

	spec := current.TypedSpec()
	if spec.Copy == nil || spec.Pool != destination.Pool || spec.Name != destination.Name {
		return ErrDestination
	}

	if spec.Copy.Source == "" {
		spec.Copy.Source = source
		spec.Copy.Digest = digest
	}

	return nil
}
