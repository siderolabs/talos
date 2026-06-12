// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cluster provides cluster configuration documents.
package cluster

//go:generate go tool github.com/siderolabs/talos/tools/docgen -output cluster_doc.go cluster.go discovery_service.go
//go:generate go tool github.com/siderolabs/deep-copy -type DiscoveryServiceConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
