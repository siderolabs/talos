// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

//docgen:jsonschema

import (
	"errors"
	"fmt"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// ContainerRunAs overrides the image's user and group.
type ContainerRunAs struct {
	//   description: |
	//     UID to run the container's entrypoint as.
	//
	//     Unset means use the image's own USER. There are no user namespaces, so uid 0 is host
	//     root.
	//   examples:
	//     - value: pointer.To[int32](65534)
	RunAsUID *int32 `yaml:"uid,omitempty"`
	//   description: |
	//     GID to run the container's entrypoint as.
	//
	//     Unset means use the image's own USER.
	//   examples:
	//     - value: pointer.To[int32](65534)
	RunAsGID *int32 `yaml:"gid,omitempty"`
}

// Check interfaces.
var _ config.ContainerRunAsConfig = &ContainerRunAs{}

// UID implements config.ContainerRunAsConfig interface.
func (r *ContainerRunAs) UID() optional.Optional[int32] {
	if r.RunAsUID == nil {
		return optional.None[int32]()
	}

	return optional.Some(*r.RunAsUID)
}

// GID implements config.ContainerRunAsConfig interface.
func (r *ContainerRunAs) GID() optional.Optional[int32] {
	if r.RunAsGID == nil {
		return optional.None[int32]()
	}

	return optional.Some(*r.RunAsGID)
}

// Validate checks the runAs settings.
func (r *ContainerRunAs) Validate() error {
	var validationErrors error

	if r.RunAsUID != nil && *r.RunAsUID < 0 {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("runAs.uid must be non-negative, got %d", *r.RunAsUID))
	}

	if r.RunAsGID != nil && *r.RunAsGID < 0 {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("runAs.gid must be non-negative, got %d", *r.RunAsGID))
	}

	return validationErrors
}
