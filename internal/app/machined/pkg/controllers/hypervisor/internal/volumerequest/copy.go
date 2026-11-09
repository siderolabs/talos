// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package volumerequest implements the shared immutable volume-copy request rules.
package volumerequest

import (
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// ErrOwner identifies a request owned by another producer, including an existing unowned request.
var ErrOwner = errors.New("different request owner")

// ErrDestination identifies an incompatible destination.
var ErrDestination = errors.New("different copy destination")

// ErrFormat reports that changing an existing copy's on-disk format is not supported.
var ErrFormat = errors.New("incompatible copy format")

// Destination names the pool resource destination, independently of its mounted backing.
type Destination struct {
	Pool     string
	Name     string
	Format   string
	Image    bool
	Capacity uint64
}

// Owned reports whether a producer may withdraw this request.
func Owned(current *storage.StoragePoolVolumeSpec, owner resource.Owner) bool {
	return current.Metadata().Owner() == owner
}

func matches(current *storage.StoragePoolVolumeSpec, destination Destination) bool {
	spec := current.TypedSpec()

	return spec.Pool == destination.Pool && spec.Name == destination.Name && spec.Format == destination.Format && spec.Copy != nil && spec.Copy.Image == destination.Image
}

// EnsureAdoption validates an existing image request before initializing source-free copy intent.
// Existing initialization intent is preserved; writable capacity follows resize requests.
func EnsureAdoption(current *storage.StoragePoolVolumeSpec, owner resource.Owner, destination Destination) error {
	if !current.Metadata().Version().Equal(resource.VersionUndefined) && !Owned(current, owner) {
		return ErrOwner
	}

	if err := checkAdoptionDestination(current, destination); err != nil {
		return err
	}

	if current.TypedSpec().Copy == nil {
		*current.TypedSpec() = storage.StoragePoolVolumeSpecSpec{
			Pool:   destination.Pool,
			Name:   destination.Name,
			Format: destination.Format,
			Copy:   &storage.StoragePoolVolumeCopySpec{Image: destination.Image},
		}
	}

	if destination.Image {
		current.TypedSpec().Capacity = destination.Capacity
	}

	return nil
}

func checkAdoptionDestination(current *storage.StoragePoolVolumeSpec, destination Destination) error {
	if current.TypedSpec().Pool == "" || matches(current, destination) {
		return nil
	}

	spec := current.TypedSpec()
	if spec.Pool == destination.Pool && spec.Name == destination.Name && spec.Format != destination.Format && spec.Copy != nil && spec.Copy.Image == destination.Image {
		return fmt.Errorf("%w: %w: format changes from %q to %q are not supported", ErrDestination, ErrFormat, spec.Format, destination.Format)
	}

	return ErrDestination
}

// ErrPublished rejects source replacement; retire the request and wait for its copy to drain.
var ErrPublished = errors.New("copy publication is not known to be absent")

// BindImage initializes a source once. Recovery withdraws the old request and waits for its drain.
// A destination never changes its digest; alias changes use a different request instead.
func BindImage(current *storage.StoragePoolVolumeSpec, owner resource.Owner, destination Destination, source, digest string) error {
	if err := checkImageBinding(current, owner, destination, digest); err != nil {
		return err
	}

	spec := current.TypedSpec()

	if spec.Copy.Source != "" && spec.Copy.Source != source {
		return ErrPublished
	}

	if spec.Copy.Source == "" {
		spec.Copy.Source = source
		spec.Copy.Digest = digest
	}

	return nil
}

func checkImageBinding(current *storage.StoragePoolVolumeSpec, owner resource.Owner, destination Destination, digest string) error {
	if !Owned(current, owner) {
		return ErrOwner
	}

	spec := current.TypedSpec()
	if spec.Copy == nil || spec.Pool != destination.Pool || spec.Name != destination.Name {
		return ErrDestination
	}

	if spec.Copy.Source != "" && spec.Copy.Digest != "" && spec.Copy.Digest != digest {
		return ErrDestination
	}

	return nil
}
