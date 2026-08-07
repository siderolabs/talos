// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package container provides container configuration documents.
package container

//go:generate go tool github.com/siderolabs/talos/tools/docgen -output container_doc.go container.go container_config.go mounts.go runas.go runtime.go security.go

//go:generate go tool github.com/siderolabs/deep-copy -type ContainerConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
