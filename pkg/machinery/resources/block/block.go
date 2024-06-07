// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package block provides resources related to blockdevices, mounts, etc.
package block

import (
	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

//go:generate deep-copy -type DeviceSpec -type DiscoveredVolumeSpec -type DiskSpec -type SystemDiskSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// NamespaceName contains configuration resources.
const NamespaceName resource.Namespace = v1alpha1.NamespaceName
