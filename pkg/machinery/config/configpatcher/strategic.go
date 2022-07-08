// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configpatcher

import (
	"fmt"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/merge"
)

// StrategicMergePatch is a strategic merge config patch.
type StrategicMergePatch struct {
	config.Provider
}

// StrategicMerge performs strategic merge config patching.
func StrategicMerge(cfg config.Provider, patch StrategicMergePatch) (config.Provider, error) {
	left := cfg.Raw()
	right := patch.Raw()

	if err := merge.Merge(left, right); err != nil {
		return nil, err
	}

	result, ok := left.(config.Provider)
	if !ok {
		return nil, fmt.Errorf("strategic left is not config.Provider %T", left)
	}

	return result, nil
}
