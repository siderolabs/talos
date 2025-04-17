// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys

import "github.com/siderolabs/talos/internal/pkg/encryption/helpers"

// KeyOption represents key option callback used in KeyHandler.GetKey func.
type KeyOption func(o *KeyOptions) error

// KeyOptions set of options to be used in KeyHandler.GetKey func.
type KeyOptions struct {
	VolumeID             string
	GetSystemInformation helpers.SystemInformationGetter
	TPMLocker            helpers.TPMLockFunc
}

// WithVolumeID passes the partition label to the key handler.
func WithVolumeID(label string) KeyOption {
	return func(o *KeyOptions) error {
		o.VolumeID = label

		return nil
	}
}

// WithSystemInformationGetter passes the node UUID to the key handler.
func WithSystemInformationGetter(getter helpers.SystemInformationGetter) KeyOption {
	return func(o *KeyOptions) error {
		o.GetSystemInformation = getter

		return nil
	}
}

// WithTPMLocker passes the TPM locker to the key handler.
func WithTPMLocker(locker helpers.TPMLockFunc) KeyOption {
	return func(o *KeyOptions) error {
		o.TPMLocker = locker

		return nil
	}
}

// NewDefaultOptions creates new KeyOptions.
func NewDefaultOptions(options []KeyOption) (*KeyOptions, error) {
	var opts KeyOptions

	for _, o := range options {
		err := o(&opts)
		if err != nil {
			return nil, err
		}
	}

	return &opts, nil
}
