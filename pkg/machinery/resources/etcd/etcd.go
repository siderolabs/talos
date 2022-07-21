// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package etcd provides resources which interface with etcd.
package etcd

import "github.com/cosi-project/runtime/pkg/resource"

//go:generate deep-copy -type PKIStatusSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// NamespaceName contains resources supporting etcd service.
const NamespaceName resource.Namespace = "etcd"
