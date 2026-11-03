// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package mutationevidence defines invocation-owned mutation marker policy.
package mutationevidence

import libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"

// Merge accumulates evidence. Invalid evidence is unknown and unknown is sticky.
func Merge(previous, next libvirtstorage.OperationOutcome) libvirtstorage.OperationOutcome {
	switch previous {
	case libvirtstorage.NoMutationSubmitted, libvirtstorage.Finished:
	case libvirtstorage.Unknown:
		return libvirtstorage.Unknown
	default:
		return libvirtstorage.Unknown
	}

	switch next {
	case libvirtstorage.NoMutationSubmitted:
		return previous
	case libvirtstorage.Finished:
		return next
	case libvirtstorage.Unknown:
		return libvirtstorage.Unknown
	default:
		return libvirtstorage.Unknown
	}
}

// CanRelease requires replacement publication after any completed mutation.
func CanRelease(outcome libvirtstorage.OperationOutcome, published bool) bool {
	switch outcome {
	case libvirtstorage.NoMutationSubmitted:
		return true
	case libvirtstorage.Finished:
		return published
	case libvirtstorage.Unknown:
		return false
	default:
		return false
	}
}
