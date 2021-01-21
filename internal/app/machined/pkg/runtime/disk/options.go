// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package disk

// Option defines a function that can alter MachineState.Disk() method output.
type Option func(options *Options)

// Options contains disk selection options.
type Options struct {
	Label string
}

// WithPartitionLabel select a disk which has the partition labeled.
func WithPartitionLabel(label string) Option {
	return func(opts *Options) {
		opts.Label = label
	}
}
