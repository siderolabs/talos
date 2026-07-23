// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package storage provides resources related to storage virtualization and orchestration.
package storage

import (
	"github.com/cosi-project/runtime/pkg/resource"
)

//go:generate go tool github.com/dmarkham/enumer -type=LVMLogicalVolumeType,MDLevel,MDMetadata,MDArrayPhase -linecomment -text

//go:generate go tool github.com/siderolabs/deep-copy -type LVMLogicalVolumeSpecSpec -type LVMLogicalVolumeStatusSpec -type LVMPhysicalVolumeSpecSpec -type LVMPhysicalVolumeStatusSpec -type LVMRefreshRequestSpec -type LVMValidationErrorSpec -type LVMVolumeGroupSpecSpec -type LVMVolumeGroupStatusSpec -type MDArraySpecSpec -type MDArrayStatusSpec -type MDRefreshRequestSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// NamespaceName contains storage resources.
const NamespaceName resource.Namespace = "storage"
