// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package minimal provides the minimal/recommended limits for different machine types.
package minimal

import (
	"fmt"

	"github.com/dustin/go-humanize"

	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

// Memory returns the minimal/recommended amount of memory required to run the node.
func Memory(typ machine.Type) (minimum, recommended uint64, err error) {
	// Account for memory unavailable to MemTotal, including kernel
	// and firmware reservations.
	switch typ { //nolint:exhaustive
	case machine.TypeControlPlane, machine.TypeInit:
		const memoryOverhead = 150 * humanize.MiByte

		minimum = 1848*humanize.MiByte - memoryOverhead
		recommended = 4*humanize.GiByte - memoryOverhead

	case machine.TypeWorker:
		const memoryOverhead = 200 * humanize.MiByte

		minimum = 1*humanize.GiByte - memoryOverhead
		recommended = 2*humanize.GiByte - memoryOverhead

	default:
		return 0, 0, fmt.Errorf("unknown machine type %q", typ)
	}

	return minimum, recommended, nil
}

// DiskSize returns the minimal/recommended amount of disk space required to run the node.
func DiskSize() uint64 {
	return 6 * humanize.GiByte
}
