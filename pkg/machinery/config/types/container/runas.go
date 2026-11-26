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
	//
	//     Required when the image's USER names its user rather than giving a numeric uid
	//     (e.g. `app` or `app:app`): Talos does not read the image's /etc/passwd, and rejects
	//     such a container instead of running it as root.
	//   examples:
	//     - value: pointer.To[uint32](65534)
	//   schema:
	//     type: integer
	//     minimum: 0
	//     maximum: 4294967294
	RunAsUID *uint32 `yaml:"uid,omitempty"`
	//   description: |
	//     GID to run the container's entrypoint as.
	//
	//     Unset means use the image's own USER.
	//
	//     Required when the image's USER names its group (e.g. `app:app`), or names only a user
	//     (e.g. `app`), whose primary group would come from the image's /etc/passwd: Talos does
	//     not read it, and rejects such a container instead of running it as root.
	//   examples:
	//     - value: pointer.To[uint32](65534)
	//   schema:
	//     type: integer
	//     minimum: 0
	//     maximum: 4294967294
	RunAsGID *uint32 `yaml:"gid,omitempty"`
}

// runAsMaxID is the highest user or group ID a container can run as.
//
// 4294967295 ((uid_t)-1) is not a valid ID: the kernel reads it as "leave unchanged".
const runAsMaxID uint32 = 4294967294

// Check interfaces.
var _ config.ContainerRunAsConfig = &ContainerRunAs{}

// UID implements config.ContainerRunAsConfig interface.
func (r *ContainerRunAs) UID() optional.Optional[uint32] {
	if r.RunAsUID == nil {
		return optional.None[uint32]()
	}

	return optional.Some(*r.RunAsUID)
}

// GID implements config.ContainerRunAsConfig interface.
func (r *ContainerRunAs) GID() optional.Optional[uint32] {
	if r.RunAsGID == nil {
		return optional.None[uint32]()
	}

	return optional.Some(*r.RunAsGID)
}

// Validate checks the runAs settings.
func (r *ContainerRunAs) Validate() error {
	var validationErrors error

	if r.RunAsUID != nil && *r.RunAsUID > runAsMaxID {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("runAs.uid must be at most %d, got %d", runAsMaxID, *r.RunAsUID))
	}

	if r.RunAsGID != nil && *r.RunAsGID > runAsMaxID {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("runAs.gid must be at most %d, got %d", runAsMaxID, *r.RunAsGID))
	}

	return validationErrors
}
