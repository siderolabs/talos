// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:revive
package security

import "github.com/cosi-project/runtime/pkg/resource"

//go:generate go tool github.com/siderolabs/deep-copy -type ImageVerificationRuleSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// NamespaceName is the namespace for security resources.
const NamespaceName resource.Namespace = "security"
