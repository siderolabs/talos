// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// WorkloadResourceConfig defines the interface to access aggregate workload resource limits.
//
// Every limit is in bytes; zero means the root is not limited.
type WorkloadResourceConfig interface {
	WorkloadResourceConfigSignal()

	KubepodsMemoryLimit() uint64
	TalosContainersMemoryLimit() uint64
	VirtualMachinesMemoryLimit() uint64
}
