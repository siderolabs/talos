// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"github.com/cosi-project/runtime/pkg/resource"
)

//go:generate deep-copy -type MemoryModuleSpec -type PCIDeviceSpec -type PCIDriverRebindConfigSpec -type PCIDriverRebindStatusSpec -type PCRStatusSpec -type ProcessorSpec -type SystemInformationSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// NamespaceName contains resources related to hardware as a whole.
const NamespaceName resource.Namespace = "hardware"
