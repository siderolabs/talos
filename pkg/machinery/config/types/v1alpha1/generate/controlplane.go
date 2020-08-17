// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	v1alpha1 "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

func controlPlaneUd(in *Input) (*v1alpha1.Config, error) {
	config, err := initUd(in)
	if err != nil {
		return nil, err
	}

	config.MachineConfig.MachineType = "controlplane"

	return config, nil
}
