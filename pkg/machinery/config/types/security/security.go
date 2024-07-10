// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package security provides security-related machine configuration documents.
package security

//go:generate docgen -output security_doc.go security.go trusted_roots.go

//go:generate deep-copy -type TrustedRootsConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
