// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"errors"
	"fmt"
	"slices"
)

// GetVmnetInterfaceName returns the name of the interface that will be assigned to the next vmnet interface.
// The name is assigned incrementing by one, starting from "bridge100".
func GetVmnetInterfaceName(allCurrentInterfaces []string) (string, error) {
	for i := 100; i < 200; i++ {
		interfaceName := fmt.Sprintf("bridge%d", i)
		if slices.Index(allCurrentInterfaces, interfaceName) == -1 {
			return interfaceName, nil
		}
	}

	return "", errors.New("all interface names seem to be already in use")
}
