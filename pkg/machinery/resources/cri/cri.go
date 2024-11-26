// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cri contains resources related to the Container Runtime Interface (CRI).
package cri

import "github.com/cosi-project/runtime/pkg/resource"

//go:generate deep-copy -type ImageCacheConfigSpec -type SeccompProfileSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

//go:generate enumer -type=ImageCacheStatus -linecomment -text

// NamespaceName contains resources related to stats.
const NamespaceName resource.Namespace = "cri"
