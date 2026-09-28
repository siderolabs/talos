// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/utils/cpuset"
)

const (
	// MaxHostCPUID is the highest host logical CPU ID a pin may name.
	//
	// This is a software bound on the ID itself (1000 logical CPUs, SMT threads included), not a
	// statement about the host: whether the CPU exists and is available is only known on the host,
	// when the domain is started.
	MaxHostCPUID = 999

	// MaxHostNUMANodeID is the highest host NUMA node ID a nodeset may name.
	//
	// libvirt sizes every cpumask, the numatune nodeset included, at VIR_DOMAIN_CPUMASK_LEN = 16384
	// bits, so a higher ID could not be represented at all. As with MaxHostCPUID, this bounds the ID,
	// not the host's actual node inventory.
	MaxHostNUMANodeID = 16383
)

// ParseHostIDList parses a Linux cpulist-formatted set of host IDs ("0-3,8") and returns it in
// canonical form (sorted, deduplicated, ranges collapsed).
//
// Every numeric endpoint is bounded to [0, maxID] before the list is expanded, so an absurd range
// like "0-1000000000" is rejected without allocating for it. The grammar itself, including reversed
// ranges and duplicates, is left to the library.
func ParseHostIDList(list string, maxID int) (cpuset.CPUSet, error) {
	for r := range strings.SplitSeq(list, ",") {
		for endpoint := range strings.SplitSeq(r, "-") {
			// Anything the bound check does not understand (a sign, a stray character) is a grammar
			// error reported by the library below.
			id, err := strconv.Atoi(endpoint)
			if err != nil {
				continue
			}

			if id < 0 || id > maxID {
				return cpuset.New(), fmt.Errorf("%d is out of range, IDs must be between 0 and %d", id, maxID)
			}
		}
	}

	return cpuset.Parse(list)
}
