// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys

// KeyOption represents key option callback used in KeyHandler.GetKey func.
type KeyOption func(o *KeyOptions) error

// KeyOptions set of options to be used in KeyHandler.GetKey func.
type KeyOptions struct {
	PartitionLabel string
	NodeUUID       string
}

// WithPartitionLabel passes the partition label to the key handler.
func WithPartitionLabel(label string) KeyOption {
	return func(o *KeyOptions) error {
		o.PartitionLabel = label

		return nil
	}
}

// WithNodeUUID passes the node UUID to the key handler.
func WithNodeUUID(uuid string) KeyOption {
	return func(o *KeyOptions) error {
		o.NodeUUID = uuid

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
