// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import "github.com/cosi-project/runtime/pkg/resource"

// NamespaceName contains resources related to cluster as a whole.
const NamespaceName resource.Namespace = "cluster"

// RawNamespaceName contains raw resources which haven't gone through the merge phase yet.
const RawNamespaceName resource.Namespace = "cluster-raw"
