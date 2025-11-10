// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cri provides container runtime interface related config documents.
package cri

//go:generate go tool github.com/siderolabs/talos/tools/docgen -output cri_doc.go registry_auth.go registry_mirror.go registry_tls.go

//go:generate go tool github.com/siderolabs/deep-copy -type RegistryAuthConfigV1Alpha1 -type RegistryMirrorConfigV1Alpha1 -type RegistryTLSConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
