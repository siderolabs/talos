// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package etcd provides etcd-related config documents.
package etcd

//go:generate go tool github.com/siderolabs/talos/tools/docgen -output etcd_doc.go etcd.go ca.go member.go

//go:generate go tool github.com/siderolabs/deep-copy -type EtcdCAConfigV1Alpha1 -type EtcdMemberConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
