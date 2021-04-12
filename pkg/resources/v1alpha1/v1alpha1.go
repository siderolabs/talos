// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package v1alpha1 provides resources which implement "glue" code from v1alpha1 Talos init system.
package v1alpha1

import "github.com/cosi-project/runtime/pkg/resource"

// NamespaceName contains resources linking v1alpha2 components with v1alpha1 Talos runtime.
const NamespaceName resource.Namespace = "runtime"
