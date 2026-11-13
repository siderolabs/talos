// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func (in *Input) generateWorkloadResourceConfigs(machineType machine.Type) []config.Document {
	if !in.Options.VersionContract.WorkloadResourceConfigEnabledByDefault() {
		return nil
	}

	reserved := constants.KubeletSystemReservedMemoryWorker

	if machineType.IsControlPlane() {
		reserved = constants.KubeletSystemReservedMemoryControlPlane
	}

	workloadResources := runtime.NewWorkloadResourceConfigV1Alpha1()
	workloadResources.ReservedConfig = &runtime.WorkloadReserved{MemoryConfig: meta.MustByteSize(reserved)}

	return []config.Document{workloadResources}
}
