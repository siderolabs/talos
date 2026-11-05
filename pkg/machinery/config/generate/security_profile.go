// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
)

func (in *Input) generateSecurityProfileConfigs() []config.Document {
	if !in.Options.VersionContract.WorkloadIsolationEnabledByDefault() {
		return nil
	}

	securityProfile := runtime.NewSecurityProfileConfigV1Alpha1()
	securityProfile.WorkloadIsolationEnabled = new(true)

	return []config.Document{securityProfile}
}
