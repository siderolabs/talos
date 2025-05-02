// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"fmt"
	"regexp"
	"strconv"
	"sync"
)

var vmnetInterfaceRegex = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`\bbridge(1\d\d)\b`) })

// GetVmnetInterfaceName returns the name of the interface that will be assigned to the next vmnet interface.
// The name is assigned incrementing by one, starting from "bridge100".
func GetVmnetInterfaceName(allCurrentInterfaces []string) (string, error) {
	vmnetInterfaceFound := false
	largestVmnetIfIndex := 100

	for _, iface := range allCurrentInterfaces {
		matches := vmnetInterfaceRegex().FindSubmatch([]byte(iface))
		if matches != nil {
			vmnetInterfaceFound = true

			index, err := strconv.Atoi(string(matches[1]))
			if err != nil {
				return "", fmt.Errorf("failed to parse interface name: %w", err)
			}

			if index > largestVmnetIfIndex {
				largestVmnetIfIndex = index
			}
		}
	}

	if !vmnetInterfaceFound {
		return "bridge100", nil
	}

	return "bridge" + strconv.Itoa(largestVmnetIfIndex+1), nil
}
