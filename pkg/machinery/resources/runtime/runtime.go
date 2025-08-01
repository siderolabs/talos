// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

//go:generate deep-copy -type BootedEntrySpec -type DevicesStatusSpec -type DiagnosticSpec -type EventSinkConfigSpec -type ExtensionServiceConfigSpec -type ExtensionServiceConfigStatusSpec -type KernelCmdlineSpec -type KernelModuleSpecSpec -type KernelParamSpecSpec -type KernelParamStatusSpec -type KmsgLogConfigSpec -type LoadedKernelModuleSpec -type MaintenanceServiceConfigSpec -type MaintenanceServiceRequestSpec -type MachineResetSignalSpec -type MachineStatusSpec -type MetaKeySpec -type MountStatusSpec -type PlatformMetadataSpec -type SecurityStateSpec -type MetaLoadedSpec -type SBOMItemSpec -type UniqueMachineTokenSpec -type VersionSpec -type WatchdogTimerConfigSpec -type WatchdogTimerStatusSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// NamespaceName contains configuration resources.
const NamespaceName resource.Namespace = v1alpha1.NamespaceName
