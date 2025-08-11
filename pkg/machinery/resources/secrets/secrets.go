// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package secrets provides resources which store secrets.
package secrets

import "github.com/cosi-project/runtime/pkg/resource"

// NamespaceName contains resources containing secret material.
const NamespaceName resource.Namespace = "secrets"

//go:generate go tool github.com/siderolabs/deep-copy -type APICertsSpec -type CertSANSpec -type EtcdCertsSpec -type EtcdRootSpec -type EncryptionSaltSpec -type KubeletSpec -type KubernetesCertsSpec -type KubernetesDynamicCertsSpec -type KubernetesRootSpec -type MaintenanceServiceCertsSpec -type MaintenanceRootSpec -type OSRootSpec -type TrustdCertsSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .
