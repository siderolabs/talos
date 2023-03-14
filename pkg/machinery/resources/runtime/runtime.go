// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//nolint:lll
//go:generate deep-copy -type KernelModuleSpecSpec -type KernelParamSpecSpec -type KernelParamStatusSpec -type MachineStatusSpec -type MetaKeySpec -type MountStatusSpec -type PlatformMetadataSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .
