// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package security provides security-related machine configuration documents.
package security

//go:generate go tool github.com/siderolabs/talos/tools/docgen -output security_doc.go security.go trusted_roots.go image_verification.go

//go:generate go tool github.com/siderolabs/deep-copy -type ImageVerificationConfigV1Alpha1 -type TrustedRootsConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
