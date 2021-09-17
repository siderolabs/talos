// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
)

// NamespaceName contains resources related to networking.
const NamespaceName resource.Namespace = "network"

// ConfigNamespaceName contains umerged resources related to networking generate from the configuration.
//
// Resources in the ConfigNamespaceName namespace are merged to produce final versions in the NamespaceName namespace.
const ConfigNamespaceName resource.Namespace = "network-config"

// LinkStatusType is type of LinkStatus resource.
const LinkStatusType = resource.Type("LinkStatuses.net.talos.dev")
