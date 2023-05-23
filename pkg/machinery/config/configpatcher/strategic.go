// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configpatcher

import (
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
)

// StrategicMergePatch is a strategic merge config patch.
type StrategicMergePatch struct {
	config.Provider
}

// StrategicMerge performs strategic merge config patching.
func StrategicMerge(cfg config.Provider, patch StrategicMergePatch) (config.Provider, error) {
	left := cfg.RawV1Alpha1()
	right := patch.RawV1Alpha1()

	if err := merge.Merge(left, right); err != nil {
		return nil, err
	}

	return left, nil
}
