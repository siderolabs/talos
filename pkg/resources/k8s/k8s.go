// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8s provides resources which interface with Kubernetes.
package k8s

import "github.com/talos-systems/os-runtime/pkg/resource"

// ControlPlaneNamespaceName contains resources supporting Kubernetes control plane.
const ControlPlaneNamespaceName resource.Namespace = "controlplane"

// ExtraNamespaceName contains extra resources related to Kubernnetes configuration.
const ExtraNamespaceName resource.Namespace = "extras"
