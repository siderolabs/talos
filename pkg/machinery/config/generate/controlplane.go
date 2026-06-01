// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	v1alpha1 "github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

func (in *Input) controlPlane() ([]config.Document, error) {
	docs, err := in.init()
	if err != nil {
		return nil, err
	}

	docs[0].(*v1alpha1.Config).MachineConfig.MachineType = machine.TypeControlPlane.String()

	return docs, nil
}
