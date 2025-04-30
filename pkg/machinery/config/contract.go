// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
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
	TalosVersion1_11    = &VersionContract{1, 11}
	TalosVersion1_10    = &VersionContract{1, 10}
	TalosVersion1_9     = &VersionContract{1, 9}
	TalosVersion1_8     = &VersionContract{1, 8}
	TalosVersion1_7     = &VersionContract{1, 7}
	TalosVersion1_6     = &VersionContract{1, 6}
	TalosVersion1_5     = &VersionContract{1, 5}
	TalosVersion1_4     = &VersionContract{1, 4}
	TalosVersion1_3     = &VersionContract{1, 3}
	TalosVersion1_2     = &VersionContract{1, 2}
	TalosVersion1_1     = &VersionContract{1, 1}
	TalosVersion1_0     = &VersionContract{1, 0}
)

var versionRegexp = regexp.MustCompile(`^v(\d+)\.(\d+)($|\.)`)

// ParseContractFromVersion parses Talos version into VersionContract.
func ParseContractFromVersion(version string) (*VersionContract, error) {
	version = "v" + strings.TrimPrefix(version, "v")

	matches := versionRegexp.FindStringSubmatch(version)
	if len(matches) < 3 {
		return nil, fmt.Errorf("error parsing version %q", version)
	}

	var contract VersionContract

	contract.Major, _ = strconv.Atoi(matches[1]) //nolint:errcheck
	contract.Minor, _ = strconv.Atoi(matches[2]) //nolint:errcheck

	return &contract, nil
}

// String returns string representation of the contract.
func (contract *VersionContract) String() string {
	if contract == nil {
		return "current"
	}

	return fmt.Sprintf("v%d.%d", contract.Major, contract.Minor)
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
	return contract.Greater(TalosVersion1_1) && !contract.Greater(TalosVersion1_2)
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

// KubeletManifestsDirectoryDisabled returns true if the manifests directory flag is supported.
func (contract *VersionContract) KubeletManifestsDirectoryDisabled() bool {
	return contract.Greater(TalosVersion1_2)
}

// SecretboxEncryptionSupported returns true if encryption with secretbox is supported.
func (contract *VersionContract) SecretboxEncryptionSupported() bool {
	return contract.Greater(TalosVersion1_2)
}

// DiskQuotaSupportEnabled returns true if XFS filesystems should enable project quota.
func (contract *VersionContract) DiskQuotaSupportEnabled() bool {
	return contract.Greater(TalosVersion1_4)
}

// KubePrismEnabled returns true if KubePrism should be enabled by default.
func (contract *VersionContract) KubePrismEnabled() bool {
	return contract.Greater(TalosVersion1_5)
}

// HostDNSEnabled returns true if host dns router should be enabled by default.
func (contract *VersionContract) HostDNSEnabled() bool {
	return contract.Greater(TalosVersion1_6)
}

// UseRSAServiceAccountKey returns true if version of Talos should use RSA Service Account key for the kube-apiserver.
func (contract *VersionContract) UseRSAServiceAccountKey() bool {
	return contract.Greater(TalosVersion1_6)
}

// ClusterNameForWorkers returns true if version of Talos should put cluster name to the worker machine config.
func (contract *VersionContract) ClusterNameForWorkers() bool {
	return contract.Greater(TalosVersion1_7)
}

// HostDNSForwardKubeDNSToHost returns true if version of Talos forces host dns router to be used as upstream for Kubernetes CoreDNS pods.
func (contract *VersionContract) HostDNSForwardKubeDNSToHost() bool {
	return contract.Greater(TalosVersion1_7)
}

// AddExcludeFromExternalLoadBalancer returns true if the label 'node.kubernetes.io/exclude-from-external-load-balancers' is automatically added
// for controlplane nodes.
func (contract *VersionContract) AddExcludeFromExternalLoadBalancer() bool {
	return contract.Greater(TalosVersion1_7)
}

// SecureBootEnrollEnforcementSupported returns true if version of Talos supports SecureBoot enforcement on enroll.
func (contract *VersionContract) SecureBootEnrollEnforcementSupported() bool {
	return contract.Greater(TalosVersion1_7)
}
