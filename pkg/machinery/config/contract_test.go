// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/config"
)

func TestContractGreater(t *testing.T) {
	assert.True(t, config.TalosVersion1_1.Greater(config.TalosVersion1_0))
	assert.True(t, config.TalosVersionCurrent.Greater(config.TalosVersion1_2))
	assert.True(t, config.TalosVersionCurrent.Greater(config.TalosVersion1_3))

	assert.False(t, config.TalosVersion1_2.Greater(config.TalosVersion1_3))
	assert.False(t, config.TalosVersion1_2.Greater(config.TalosVersion1_2))
	assert.False(t, config.TalosVersionCurrent.Greater(config.TalosVersionCurrent))
}

func TestContractParseVersion(t *testing.T) {
	t.Parallel()

	for v, expected := range map[string]*config.VersionContract{
		"v1.5":           config.TalosVersion1_5,
		"v1.5.":          config.TalosVersion1_5,
		"v1.5.1":         config.TalosVersion1_5,
		"v1.88":          {1, 88},
		"v1.5.3-alpha.4": config.TalosVersion1_5,
		"1.6":            config.TalosVersion1_6,
	} {
		t.Run(v, func(t *testing.T) {
			t.Parallel()

			actual, err := config.ParseContractFromVersion(v)
			assert.NoError(t, err)
			assert.Equal(t, expected, actual)
		})
	}
}

func TestContractCurrent(t *testing.T) {
	contract := config.TalosVersionCurrent

	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.True(t, contract.StableHostnameEnabled())
	assert.True(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.True(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.True(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.True(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.True(t, contract.APIServerAuditPolicySupported())
	assert.True(t, contract.KubeletManifestsDirectoryDisabled())
	assert.True(t, contract.SecretboxEncryptionSupported())
	assert.True(t, contract.DiskQuotaSupportEnabled())
	assert.True(t, contract.KubePrismEnabled())
	assert.True(t, contract.HostDNSEnabled())
	assert.True(t, contract.UseRSAServiceAccountKey())
	assert.True(t, contract.ClusterNameForWorkers())
	assert.True(t, contract.HostDNSForwardKubeDNSToHost())
	assert.True(t, contract.AddExcludeFromExternalLoadBalancer())
	assert.True(t, contract.SecureBootEnrollEnforcementSupported())
}

func TestContract1_11(t *testing.T) {
	contract := config.TalosVersion1_11

	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.True(t, contract.StableHostnameEnabled())
	assert.True(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.True(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.True(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.True(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.True(t, contract.APIServerAuditPolicySupported())
	assert.True(t, contract.KubeletManifestsDirectoryDisabled())
	assert.True(t, contract.SecretboxEncryptionSupported())
	assert.True(t, contract.DiskQuotaSupportEnabled())
	assert.True(t, contract.KubePrismEnabled())
	assert.True(t, contract.HostDNSEnabled())
	assert.True(t, contract.UseRSAServiceAccountKey())
	assert.True(t, contract.ClusterNameForWorkers())
	assert.True(t, contract.HostDNSForwardKubeDNSToHost())
	assert.True(t, contract.AddExcludeFromExternalLoadBalancer())
	assert.True(t, contract.SecureBootEnrollEnforcementSupported())
}

func TestContract1_10(t *testing.T) {
	contract := config.TalosVersion1_10

	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.True(t, contract.StableHostnameEnabled())
	assert.True(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.True(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.True(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.True(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.True(t, contract.APIServerAuditPolicySupported())
	assert.True(t, contract.KubeletManifestsDirectoryDisabled())
	assert.True(t, contract.SecretboxEncryptionSupported())
	assert.True(t, contract.DiskQuotaSupportEnabled())
	assert.True(t, contract.KubePrismEnabled())
	assert.True(t, contract.HostDNSEnabled())
	assert.True(t, contract.UseRSAServiceAccountKey())
	assert.True(t, contract.ClusterNameForWorkers())
	assert.True(t, contract.HostDNSForwardKubeDNSToHost())
	assert.True(t, contract.AddExcludeFromExternalLoadBalancer())
	assert.True(t, contract.SecureBootEnrollEnforcementSupported())
}

func TestContract1_9(t *testing.T) {
	contract := config.TalosVersion1_9

	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.True(t, contract.StableHostnameEnabled())
	assert.True(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.True(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.True(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.True(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.True(t, contract.APIServerAuditPolicySupported())
	assert.True(t, contract.KubeletManifestsDirectoryDisabled())
	assert.True(t, contract.SecretboxEncryptionSupported())
	assert.True(t, contract.DiskQuotaSupportEnabled())
	assert.True(t, contract.KubePrismEnabled())
	assert.True(t, contract.HostDNSEnabled())
	assert.True(t, contract.UseRSAServiceAccountKey())
	assert.True(t, contract.ClusterNameForWorkers())
	assert.True(t, contract.HostDNSForwardKubeDNSToHost())
	assert.True(t, contract.AddExcludeFromExternalLoadBalancer())
	assert.True(t, contract.SecureBootEnrollEnforcementSupported())
}

func TestContract1_8(t *testing.T) {
	contract := config.TalosVersion1_8

	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.True(t, contract.StableHostnameEnabled())
	assert.True(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.True(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.True(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.True(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.True(t, contract.APIServerAuditPolicySupported())
	assert.True(t, contract.KubeletManifestsDirectoryDisabled())
	assert.True(t, contract.SecretboxEncryptionSupported())
	assert.True(t, contract.DiskQuotaSupportEnabled())
	assert.True(t, contract.KubePrismEnabled())
	assert.True(t, contract.HostDNSEnabled())
	assert.True(t, contract.UseRSAServiceAccountKey())
	assert.True(t, contract.ClusterNameForWorkers())
	assert.True(t, contract.HostDNSForwardKubeDNSToHost())
	assert.True(t, contract.AddExcludeFromExternalLoadBalancer())
	assert.True(t, contract.SecureBootEnrollEnforcementSupported())
}

func TestContract1_7(t *testing.T) {
	contract := config.TalosVersion1_7

	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.True(t, contract.StableHostnameEnabled())
	assert.True(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.True(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.True(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.True(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.True(t, contract.APIServerAuditPolicySupported())
	assert.True(t, contract.KubeletManifestsDirectoryDisabled())
	assert.True(t, contract.SecretboxEncryptionSupported())
	assert.True(t, contract.DiskQuotaSupportEnabled())
	assert.True(t, contract.KubePrismEnabled())
	assert.True(t, contract.HostDNSEnabled())
	assert.True(t, contract.UseRSAServiceAccountKey())
	assert.False(t, contract.ClusterNameForWorkers())
	assert.False(t, contract.HostDNSForwardKubeDNSToHost())
	assert.False(t, contract.AddExcludeFromExternalLoadBalancer())
	assert.False(t, contract.SecureBootEnrollEnforcementSupported())
}

func TestContract1_6(t *testing.T) {
	contract := config.TalosVersion1_6

	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.True(t, contract.StableHostnameEnabled())
	assert.True(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.True(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.True(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.True(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.True(t, contract.APIServerAuditPolicySupported())
	assert.True(t, contract.KubeletManifestsDirectoryDisabled())
	assert.True(t, contract.SecretboxEncryptionSupported())
	assert.True(t, contract.DiskQuotaSupportEnabled())
	assert.True(t, contract.KubePrismEnabled())
	assert.False(t, contract.HostDNSEnabled())
	assert.False(t, contract.UseRSAServiceAccountKey())
	assert.False(t, contract.ClusterNameForWorkers())
	assert.False(t, contract.HostDNSForwardKubeDNSToHost())
	assert.False(t, contract.AddExcludeFromExternalLoadBalancer())
	assert.False(t, contract.SecureBootEnrollEnforcementSupported())
}

func TestContract1_5(t *testing.T) {
	contract := config.TalosVersion1_5

	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.True(t, contract.StableHostnameEnabled())
	assert.True(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.True(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.True(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.True(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.True(t, contract.APIServerAuditPolicySupported())
	assert.True(t, contract.KubeletManifestsDirectoryDisabled())
	assert.True(t, contract.SecretboxEncryptionSupported())
	assert.True(t, contract.DiskQuotaSupportEnabled())
	assert.False(t, contract.KubePrismEnabled())
	assert.False(t, contract.HostDNSEnabled())
	assert.False(t, contract.UseRSAServiceAccountKey())
	assert.False(t, contract.ClusterNameForWorkers())
	assert.False(t, contract.HostDNSForwardKubeDNSToHost())
	assert.False(t, contract.AddExcludeFromExternalLoadBalancer())
	assert.False(t, contract.SecureBootEnrollEnforcementSupported())
}

func TestContract1_4(t *testing.T) {
	contract := config.TalosVersion1_4

	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.True(t, contract.StableHostnameEnabled())
	assert.True(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.True(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.True(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.True(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.True(t, contract.APIServerAuditPolicySupported())
	assert.True(t, contract.KubeletManifestsDirectoryDisabled())
	assert.True(t, contract.SecretboxEncryptionSupported())
	assert.False(t, contract.DiskQuotaSupportEnabled())
	assert.False(t, contract.KubePrismEnabled())
	assert.False(t, contract.HostDNSEnabled())
	assert.False(t, contract.UseRSAServiceAccountKey())
	assert.False(t, contract.ClusterNameForWorkers())
	assert.False(t, contract.HostDNSForwardKubeDNSToHost())
	assert.False(t, contract.AddExcludeFromExternalLoadBalancer())
	assert.False(t, contract.SecureBootEnrollEnforcementSupported())
}

func TestContract1_3(t *testing.T) {
	contract := config.TalosVersion1_3

	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.True(t, contract.StableHostnameEnabled())
	assert.True(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.True(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.True(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.True(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.True(t, contract.APIServerAuditPolicySupported())
	assert.True(t, contract.KubeletManifestsDirectoryDisabled())
	assert.True(t, contract.SecretboxEncryptionSupported())
	assert.False(t, contract.DiskQuotaSupportEnabled())
	assert.False(t, contract.KubePrismEnabled())
	assert.False(t, contract.HostDNSEnabled())
	assert.False(t, contract.UseRSAServiceAccountKey())
	assert.False(t, contract.ClusterNameForWorkers())
	assert.False(t, contract.HostDNSForwardKubeDNSToHost())
	assert.False(t, contract.AddExcludeFromExternalLoadBalancer())
	assert.False(t, contract.SecureBootEnrollEnforcementSupported())
}

func TestContract1_2(t *testing.T) {
	contract := config.TalosVersion1_2

	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.True(t, contract.StableHostnameEnabled())
	assert.True(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.True(t, contract.KubernetesAlternateImageRegistries())
	assert.True(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.True(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.False(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.False(t, contract.APIServerAuditPolicySupported())
	assert.False(t, contract.KubeletManifestsDirectoryDisabled())
	assert.False(t, contract.SecretboxEncryptionSupported())
	assert.False(t, contract.DiskQuotaSupportEnabled())
	assert.False(t, contract.KubePrismEnabled())
	assert.False(t, contract.HostDNSEnabled())
	assert.False(t, contract.UseRSAServiceAccountKey())
	assert.False(t, contract.ClusterNameForWorkers())
	assert.False(t, contract.HostDNSForwardKubeDNSToHost())
	assert.False(t, contract.AddExcludeFromExternalLoadBalancer())
	assert.False(t, contract.SecureBootEnrollEnforcementSupported())
}

func TestContract1_1(t *testing.T) {
	contract := config.TalosVersion1_1

	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.False(t, contract.StableHostnameEnabled())
	assert.False(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.False(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.False(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.False(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.False(t, contract.APIServerAuditPolicySupported())
	assert.False(t, contract.KubeletManifestsDirectoryDisabled())
	assert.False(t, contract.SecretboxEncryptionSupported())
	assert.False(t, contract.DiskQuotaSupportEnabled())
	assert.False(t, contract.KubePrismEnabled())
	assert.False(t, contract.HostDNSEnabled())
	assert.False(t, contract.UseRSAServiceAccountKey())
	assert.False(t, contract.ClusterNameForWorkers())
	assert.False(t, contract.HostDNSForwardKubeDNSToHost())
	assert.False(t, contract.AddExcludeFromExternalLoadBalancer())
	assert.False(t, contract.SecureBootEnrollEnforcementSupported())
}

func TestContract1_0(t *testing.T) {
	contract := config.TalosVersion1_0

	assert.False(t, contract.PodSecurityAdmissionEnabled())
	assert.False(t, contract.StableHostnameEnabled())
	assert.False(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.False(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.False(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.False(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.False(t, contract.APIServerAuditPolicySupported())
	assert.False(t, contract.KubeletManifestsDirectoryDisabled())
	assert.False(t, contract.SecretboxEncryptionSupported())
	assert.False(t, contract.DiskQuotaSupportEnabled())
	assert.False(t, contract.KubePrismEnabled())
	assert.False(t, contract.HostDNSEnabled())
	assert.False(t, contract.UseRSAServiceAccountKey())
	assert.False(t, contract.ClusterNameForWorkers())
	assert.False(t, contract.HostDNSForwardKubeDNSToHost())
	assert.False(t, contract.AddExcludeFromExternalLoadBalancer())
	assert.False(t, contract.SecureBootEnrollEnforcementSupported())
}
