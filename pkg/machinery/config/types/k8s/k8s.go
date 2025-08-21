// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8s provides k8s-related machine configuration documents.
package k8s

//go:generate go tool github.com/siderolabs/talos/tools/docgen -output k8s_doc.go k8s.go etcd_encryption.go

//go:generate go tool github.com/siderolabs/deep-copy -type EtcdEncryptionConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
