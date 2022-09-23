// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/machinery/config"
)

func TestContractGreater(t *testing.T) {
	assert.True(t, config.TalosVersion0_9.Greater(config.TalosVersion0_8))
	assert.True(t, config.TalosVersionCurrent.Greater(config.TalosVersion0_8))
	assert.True(t, config.TalosVersionCurrent.Greater(config.TalosVersion0_9))

	assert.False(t, config.TalosVersion0_8.Greater(config.TalosVersion0_9))
	assert.False(t, config.TalosVersion0_8.Greater(config.TalosVersion0_8))
	assert.False(t, config.TalosVersionCurrent.Greater(config.TalosVersionCurrent))
}

func TestContractParseVersion(t *testing.T) {
	t.Parallel()

	for v, expected := range map[string]*config.VersionContract{
		"v0.8":           config.TalosVersion0_8,
		"v0.8.":          config.TalosVersion0_8,
		"v0.8.1":         config.TalosVersion0_8,
		"v0.88":          {0, 88},
		"v0.8.3-alpha.4": config.TalosVersion0_8,
	} {
		v, expected := v, expected
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

	assert.True(t, contract.SupportsAggregatorCA())
	assert.True(t, contract.SupportsECDSAKeys())
	assert.True(t, contract.SupportsServiceAccount())
	assert.True(t, contract.SupportsRBACFeature())
	assert.True(t, contract.SupportsDynamicCertSANs())
	assert.True(t, contract.SupportsECDSASHA256())
	assert.True(t, contract.ClusterDiscoveryEnabled())
	assert.False(t, contract.PodSecurityPolicyEnabled())
	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.True(t, contract.StableHostnameEnabled())
	assert.True(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.True(t, contract.KubernetesAlternateImageRegistries())
	assert.True(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.True(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.True(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.True(t, contract.APIServerAuditPolicySupported())
}

func TestContract1_3(t *testing.T) {
	contract := config.TalosVersion1_3

	assert.True(t, contract.SupportsAggregatorCA())
	assert.True(t, contract.SupportsECDSAKeys())
	assert.True(t, contract.SupportsServiceAccount())
	assert.True(t, contract.SupportsRBACFeature())
	assert.True(t, contract.SupportsDynamicCertSANs())
	assert.True(t, contract.SupportsECDSASHA256())
	assert.True(t, contract.ClusterDiscoveryEnabled())
	assert.False(t, contract.PodSecurityPolicyEnabled())
	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.True(t, contract.StableHostnameEnabled())
	assert.True(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.True(t, contract.KubernetesAlternateImageRegistries())
	assert.True(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.True(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.True(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.True(t, contract.APIServerAuditPolicySupported())
}

func TestContract1_2(t *testing.T) {
	contract := config.TalosVersion1_2

	assert.True(t, contract.SupportsAggregatorCA())
	assert.True(t, contract.SupportsECDSAKeys())
	assert.True(t, contract.SupportsServiceAccount())
	assert.True(t, contract.SupportsRBACFeature())
	assert.True(t, contract.SupportsDynamicCertSANs())
	assert.True(t, contract.SupportsECDSASHA256())
	assert.True(t, contract.ClusterDiscoveryEnabled())
	assert.False(t, contract.PodSecurityPolicyEnabled())
	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.True(t, contract.StableHostnameEnabled())
	assert.True(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.True(t, contract.KubernetesAlternateImageRegistries())
	assert.True(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.True(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.False(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.False(t, contract.APIServerAuditPolicySupported())
}

func TestContract1_1(t *testing.T) {
	contract := config.TalosVersion1_1

	assert.True(t, contract.SupportsAggregatorCA())
	assert.True(t, contract.SupportsECDSAKeys())
	assert.True(t, contract.SupportsServiceAccount())
	assert.True(t, contract.SupportsRBACFeature())
	assert.True(t, contract.SupportsDynamicCertSANs())
	assert.True(t, contract.SupportsECDSASHA256())
	assert.True(t, contract.ClusterDiscoveryEnabled())
	assert.False(t, contract.PodSecurityPolicyEnabled())
	assert.True(t, contract.PodSecurityAdmissionEnabled())
	assert.False(t, contract.StableHostnameEnabled())
	assert.False(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.False(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.False(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.False(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.False(t, contract.APIServerAuditPolicySupported())
}

func TestContract1_0(t *testing.T) {
	contract := config.TalosVersion1_0

	assert.True(t, contract.SupportsAggregatorCA())
	assert.True(t, contract.SupportsECDSAKeys())
	assert.True(t, contract.SupportsServiceAccount())
	assert.True(t, contract.SupportsRBACFeature())
	assert.True(t, contract.SupportsDynamicCertSANs())
	assert.True(t, contract.SupportsECDSASHA256())
	assert.True(t, contract.ClusterDiscoveryEnabled())
	assert.False(t, contract.PodSecurityPolicyEnabled())
	assert.False(t, contract.PodSecurityAdmissionEnabled())
	assert.False(t, contract.StableHostnameEnabled())
	assert.False(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.False(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.False(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.False(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.False(t, contract.APIServerAuditPolicySupported())
}

func TestContract0_14(t *testing.T) {
	contract := config.TalosVersion0_14

	assert.True(t, contract.SupportsAggregatorCA())
	assert.True(t, contract.SupportsECDSAKeys())
	assert.True(t, contract.SupportsServiceAccount())
	assert.True(t, contract.SupportsRBACFeature())
	assert.True(t, contract.SupportsDynamicCertSANs())
	assert.True(t, contract.SupportsECDSASHA256())
	assert.True(t, contract.ClusterDiscoveryEnabled())
	assert.True(t, contract.PodSecurityPolicyEnabled())
	assert.False(t, contract.PodSecurityAdmissionEnabled())
	assert.False(t, contract.StableHostnameEnabled())
	assert.False(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.False(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.False(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.False(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.False(t, contract.APIServerAuditPolicySupported())
}

func TestContract0_13(t *testing.T) {
	contract := config.TalosVersion0_13

	assert.True(t, contract.SupportsAggregatorCA())
	assert.True(t, contract.SupportsECDSAKeys())
	assert.True(t, contract.SupportsServiceAccount())
	assert.True(t, contract.SupportsRBACFeature())
	assert.True(t, contract.SupportsDynamicCertSANs())
	assert.True(t, contract.SupportsECDSASHA256())
	assert.False(t, contract.ClusterDiscoveryEnabled())
	assert.True(t, contract.PodSecurityPolicyEnabled())
	assert.False(t, contract.PodSecurityAdmissionEnabled())
	assert.False(t, contract.StableHostnameEnabled())
	assert.False(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.False(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.False(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.False(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.False(t, contract.APIServerAuditPolicySupported())
}

func TestContract0_12(t *testing.T) {
	contract := config.TalosVersion0_12

	assert.True(t, contract.SupportsAggregatorCA())
	assert.True(t, contract.SupportsECDSAKeys())
	assert.True(t, contract.SupportsServiceAccount())
	assert.True(t, contract.SupportsRBACFeature())
	assert.False(t, contract.SupportsDynamicCertSANs())
	assert.False(t, contract.SupportsECDSASHA256())
	assert.False(t, contract.ClusterDiscoveryEnabled())
	assert.True(t, contract.PodSecurityPolicyEnabled())
	assert.False(t, contract.PodSecurityAdmissionEnabled())
	assert.False(t, contract.StableHostnameEnabled())
	assert.False(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.False(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.False(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.False(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.False(t, contract.APIServerAuditPolicySupported())
}

func TestContract0_11(t *testing.T) {
	contract := config.TalosVersion0_11

	assert.True(t, contract.SupportsAggregatorCA())
	assert.True(t, contract.SupportsECDSAKeys())
	assert.True(t, contract.SupportsServiceAccount())
	assert.True(t, contract.SupportsRBACFeature())
	assert.False(t, contract.SupportsDynamicCertSANs())
	assert.False(t, contract.SupportsECDSASHA256())
	assert.False(t, contract.ClusterDiscoveryEnabled())
	assert.True(t, contract.PodSecurityPolicyEnabled())
	assert.False(t, contract.PodSecurityAdmissionEnabled())
	assert.False(t, contract.StableHostnameEnabled())
	assert.False(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.False(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.False(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.False(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.False(t, contract.APIServerAuditPolicySupported())
}

func TestContract0_10(t *testing.T) {
	contract := config.TalosVersion0_10

	assert.True(t, contract.SupportsAggregatorCA())
	assert.True(t, contract.SupportsECDSAKeys())
	assert.True(t, contract.SupportsServiceAccount())
	assert.False(t, contract.SupportsRBACFeature())
	assert.False(t, contract.SupportsDynamicCertSANs())
	assert.False(t, contract.SupportsECDSASHA256())
	assert.False(t, contract.ClusterDiscoveryEnabled())
	assert.True(t, contract.PodSecurityPolicyEnabled())
	assert.False(t, contract.PodSecurityAdmissionEnabled())
	assert.False(t, contract.StableHostnameEnabled())
	assert.False(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.False(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.False(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.False(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.False(t, contract.APIServerAuditPolicySupported())
}

func TestContract0_9(t *testing.T) {
	contract := config.TalosVersion0_9

	assert.True(t, contract.SupportsAggregatorCA())
	assert.True(t, contract.SupportsECDSAKeys())
	assert.True(t, contract.SupportsServiceAccount())
	assert.False(t, contract.SupportsRBACFeature())
	assert.False(t, contract.SupportsDynamicCertSANs())
	assert.False(t, contract.SupportsECDSASHA256())
	assert.False(t, contract.ClusterDiscoveryEnabled())
	assert.True(t, contract.PodSecurityPolicyEnabled())
	assert.False(t, contract.PodSecurityAdmissionEnabled())
	assert.False(t, contract.StableHostnameEnabled())
	assert.False(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.False(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.False(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.False(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.False(t, contract.APIServerAuditPolicySupported())
}

func TestContract0_8(t *testing.T) {
	contract := config.TalosVersion0_8

	assert.False(t, contract.SupportsAggregatorCA())
	assert.False(t, contract.SupportsECDSAKeys())
	assert.False(t, contract.SupportsServiceAccount())
	assert.False(t, contract.SupportsRBACFeature())
	assert.False(t, contract.SupportsDynamicCertSANs())
	assert.False(t, contract.SupportsECDSASHA256())
	assert.False(t, contract.ClusterDiscoveryEnabled())
	assert.True(t, contract.PodSecurityPolicyEnabled())
	assert.False(t, contract.PodSecurityAdmissionEnabled())
	assert.False(t, contract.StableHostnameEnabled())
	assert.False(t, contract.KubeletDefaultRuntimeSeccompProfileEnabled())
	assert.False(t, contract.KubernetesAlternateImageRegistries())
	assert.False(t, contract.KubernetesAllowSchedulingOnControlPlanes())
	assert.False(t, contract.KubernetesDiscoveryBackendDisabled())
	assert.False(t, contract.ApidExtKeyUsageCheckEnabled())
	assert.False(t, contract.APIServerAuditPolicySupported())
}
