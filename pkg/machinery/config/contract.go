// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"fmt"
	"regexp"
	"strconv"
)

// VersionContract describes Talos version to generate config for.
//
// Config generation only supports backwards compatibility (e.g. Talos 0.9 can generate configs for Talos 0.9 and 0.8).
// Matching version of the machinery package is required to generate configs for the current version of Talos.
//
// Nil value of *VersionContract always describes current version of Talos.
type VersionContract struct {
	Major int
	Minor int
}

// Well-known Talos version contracts.
var (
	TalosVersionCurrent = (*VersionContract)(nil)
	TalosVersion1_3     = &VersionContract{1, 3}
	TalosVersion1_2     = &VersionContract{1, 2}
	TalosVersion1_1     = &VersionContract{1, 1}
	TalosVersion1_0     = &VersionContract{1, 0}
	TalosVersion0_14    = &VersionContract{0, 14}
	TalosVersion0_13    = &VersionContract{0, 13}
	TalosVersion0_12    = &VersionContract{0, 12}
	TalosVersion0_11    = &VersionContract{0, 11}
	TalosVersion0_10    = &VersionContract{0, 10}
	TalosVersion0_9     = &VersionContract{0, 9}
	TalosVersion0_8     = &VersionContract{0, 8}
)

var versionRegexp = regexp.MustCompile(`^v(\d+)\.(\d+)($|\.)`)

// ParseContractFromVersion parses Talos version into VersionContract.
func ParseContractFromVersion(version string) (*VersionContract, error) {
	matches := versionRegexp.FindStringSubmatch(version)
	if len(matches) < 3 {
		return nil, fmt.Errorf("error parsing version %q", version)
	}

	var contract VersionContract

	contract.Major, _ = strconv.Atoi(matches[1]) //nolint:errcheck
	contract.Minor, _ = strconv.Atoi(matches[2]) //nolint:errcheck

	return &contract, nil
}

// Greater compares contract to another contract.
func (contract *VersionContract) Greater(other *VersionContract) bool {
	if contract == nil {
		return other != nil
	}

	if other == nil {
		return false
	}

	return contract.Major > other.Major || (contract.Major == other.Major && contract.Minor > other.Minor)
}

// SupportsECDSAKeys returns true if version of Talos supports ECDSA keys (vs. RSA keys).
func (contract *VersionContract) SupportsECDSAKeys() bool {
	return contract.Greater(TalosVersion0_8)
}

// SupportsAggregatorCA returns true if version of Talos supports AggregatorCA in the config.
func (contract *VersionContract) SupportsAggregatorCA() bool {
	return contract.Greater(TalosVersion0_8)
}

// SupportsServiceAccount returns true if version of Talos supports ServiceAccount in the config.
func (contract *VersionContract) SupportsServiceAccount() bool {
	return contract.Greater(TalosVersion0_8)
}

// SupportsRBACFeature returns true if version of Talos supports RBAC feature gate.
func (contract *VersionContract) SupportsRBACFeature() bool {
	return contract.Greater(TalosVersion0_10)
}

// SupportsDynamicCertSANs returns true if version of Talos supports dynamic certificate generation with SANs provided from resources.
func (contract *VersionContract) SupportsDynamicCertSANs() bool {
	return contract.Greater(TalosVersion0_12)
}

// SupportsECDSASHA256 returns true if version of Talos supports ECDSA-SHA256 for Kubernetes certificates.
func (contract *VersionContract) SupportsECDSASHA256() bool {
	return contract.Greater(TalosVersion0_12)
}

// ClusterDiscoveryEnabled returns true if cluster discovery should be enabled by default.
func (contract *VersionContract) ClusterDiscoveryEnabled() bool {
	return contract.Greater(TalosVersion0_13)
}

// PodSecurityPolicyEnabled returns true if pod security policy should be enabled by default.
func (contract *VersionContract) PodSecurityPolicyEnabled() bool {
	return !contract.Greater(TalosVersion0_14)
}

// PodSecurityAdmissionEnabled returns true if pod security admission should be enabled by default.
func (contract *VersionContract) PodSecurityAdmissionEnabled() bool {
	return contract.Greater(TalosVersion1_0)
}

// StableHostnameEnabled returns true if stable hostname generation should be enabled by default.
func (contract *VersionContract) StableHostnameEnabled() bool {
	return contract.Greater(TalosVersion1_1)
}

// KubeletDefaultRuntimeSeccompProfileEnabled returns true if kubelet seccomp profile should be enabled by default.
func (contract *VersionContract) KubeletDefaultRuntimeSeccompProfileEnabled() bool {
	return contract.Greater(TalosVersion1_1)
}

// KubernetesAlternateImageRegistries returns true if alternate image registries should be enabled by default.
// https://github.com/kubernetes/kubernetes/pull/109938
func (contract *VersionContract) KubernetesAlternateImageRegistries() bool {
	return contract.Greater(TalosVersion1_1)
}

// KubernetesAllowSchedulingOnControlPlanes returns true if scheduling on control planes should be enabled by default.
func (contract *VersionContract) KubernetesAllowSchedulingOnControlPlanes() bool {
	return contract.Greater(TalosVersion1_1)
}

// KubernetesDiscoveryBackendDisabled returns true if Kubernetes cluster discovery backend should be disabled by default.
func (contract *VersionContract) KubernetesDiscoveryBackendDisabled() bool {
	return contract.Greater(TalosVersion1_1)
}

// ApidExtKeyUsageCheckEnabled returns true if apid should check ext key usage of client certificates.
func (contract *VersionContract) ApidExtKeyUsageCheckEnabled() bool {
	return contract.Greater(TalosVersion1_2)
}

// APIServerAuditPolicySupported returns true if kube-apiserver custom audit policy is supported.
func (contract *VersionContract) APIServerAuditPolicySupported() bool {
	return contract.Greater(TalosVersion1_2)
}
