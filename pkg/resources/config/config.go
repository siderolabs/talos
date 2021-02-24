// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package config provides resources which hold Talos node configuration.
package config

import "github.com/talos-systems/os-runtime/pkg/resource"

// NamespaceName contains configuration resources.
const NamespaceName resource.Namespace = "config"

// Type represents short resource alias.
const Type resource.Type = "config"
