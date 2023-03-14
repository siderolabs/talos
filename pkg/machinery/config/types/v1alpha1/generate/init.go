// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"fmt"
	"net/url"

	"github.com/siderolabs/go-pointer"

	v1alpha1 "github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//nolint:gocyclo
func initUd(in *Input) (*v1alpha1.Config, error) {
	config := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ConfigDebug:   pointer.To(in.Debug),
		ConfigPersist: pointer.To(in.Persist),
	}

	networkConfig := &v1alpha1.NetworkConfig{}

	for _, opt := range in.NetworkConfigOptions {
		if err := opt(machine.TypeControlPlane, networkConfig); err != nil {
			return nil, err
		}
	}

	machine := &v1alpha1.MachineConfig{
		MachineType: machine.TypeInit.String(),
		MachineKubelet: &v1alpha1.KubeletConfig{
			KubeletImage: emptyIf(fmt.Sprintf("%s:v%s", constants.KubeletImage, in.KubernetesVersion), in.KubernetesVersion),
		},
		MachineNetwork:  networkConfig,
		MachineCA:       in.Certs.OS,
		MachineCertSANs: in.AdditionalMachineCertSANs,
		MachineToken:    in.TrustdInfo.Token,
		MachineInstall: &v1alpha1.InstallConfig{
			InstallDisk:            in.InstallDisk,
			InstallImage:           in.InstallImage,
			InstallBootloader:      pointer.To(true),
			InstallWipe:            pointer.To(false),
			InstallExtraKernelArgs: in.InstallExtraKernelArgs,
			InstallEphemeralSize:   in.InstallEphemeralSize,
		},
		MachineRegistries: v1alpha1.RegistriesConfig{
			RegistryMirrors: in.RegistryMirrors,
			RegistryConfig:  in.RegistryConfig,
		},
		MachineDisks:                in.MachineDisks,
		MachineSystemDiskEncryption: in.SystemDiskEncryptionConfig,
		MachineSysctls:              in.Sysctls,
		MachineFeatures:             &v1alpha1.FeaturesConfig{},
	}

	if in.VersionContract.SupportsRBACFeature() {
		machine.MachineFeatures.RBAC = pointer.To(true)
	}

	if in.VersionContract.StableHostnameEnabled() {
		machine.MachineFeatures.StableHostname = pointer.To(true)
	}

	if in.VersionContract.ApidExtKeyUsageCheckEnabled() {
		machine.MachineFeatures.ApidCheckExtKeyUsage = pointer.To(true)
	}

	if in.VersionContract.KubeletDefaultRuntimeSeccompProfileEnabled() {
		machine.MachineKubelet.KubeletDefaultRuntimeSeccompProfileEnabled = pointer.To(true)
	}

	if in.VersionContract.KubeletManifestsDirectoryDisabled() {
		machine.MachineKubelet.KubeletDisableManifestsDirectory = pointer.To(true)
	}

	certSANs := in.GetAPIServerSANs()

	controlPlaneURL, err := url.Parse(in.ControlPlaneEndpoint)
	if err != nil {
		return config, err
	}

	var admissionControlConfig []*v1alpha1.AdmissionPluginConfig

	if in.VersionContract.PodSecurityAdmissionEnabled() {
		admissionControlConfig = append(admissionControlConfig,
			&v1alpha1.AdmissionPluginConfig{
				PluginName: "PodSecurity",
				PluginConfiguration: v1alpha1.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "pod-security.admission.config.k8s.io/v1alpha1",
						"kind":       "PodSecurityConfiguration",
						"defaults": map[string]interface{}{
							"enforce":         "baseline",
							"enforce-version": "latest",
							"audit":           "restricted",
							"audit-version":   "latest",
							"warn":            "restricted",
							"warn-version":    "latest",
						},
						"exemptions": map[string]interface{}{
							"usernames":      []interface{}{},
							"runtimeClasses": []interface{}{},
							"namespaces":     []interface{}{"kube-system"},
						},
					},
				},
			},
		)
	}

	var auditPolicyConfig v1alpha1.Unstructured

	if in.VersionContract.APIServerAuditPolicySupported() {
		auditPolicyConfig = v1alpha1.APIServerDefaultAuditPolicy
	}

	cluster := &v1alpha1.ClusterConfig{
		ClusterID:     in.ClusterID,
		ClusterName:   in.ClusterName,
		ClusterSecret: in.ClusterSecret,
		ControlPlane: &v1alpha1.ControlPlaneConfig{
			Endpoint:           &v1alpha1.Endpoint{URL: controlPlaneURL},
			LocalAPIServerPort: in.LocalAPIServerPort,
		},
		APIServerConfig: &v1alpha1.APIServerConfig{
			CertSANs:               certSANs,
			ContainerImage:         emptyIf(fmt.Sprintf("%s:v%s", constants.KubernetesAPIServerImage, in.KubernetesVersion), in.KubernetesVersion),
			AdmissionControlConfig: admissionControlConfig,
			AuditPolicyConfig:      auditPolicyConfig,
		},
		ControllerManagerConfig: &v1alpha1.ControllerManagerConfig{
			ContainerImage: emptyIf(fmt.Sprintf("%s:v%s", constants.KubernetesControllerManagerImage, in.KubernetesVersion), in.KubernetesVersion),
		},
		ProxyConfig: &v1alpha1.ProxyConfig{
			ContainerImage: emptyIf(fmt.Sprintf("%s:v%s", constants.KubeProxyImage, in.KubernetesVersion), in.KubernetesVersion),
		},
		SchedulerConfig: &v1alpha1.SchedulerConfig{
			ContainerImage: emptyIf(fmt.Sprintf("%s:v%s", constants.KubernetesSchedulerImage, in.KubernetesVersion), in.KubernetesVersion),
		},
		EtcdConfig: &v1alpha1.EtcdConfig{
			RootCA: in.Certs.Etcd,
		},
		ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
			DNSDomain:     in.ServiceDomain,
			PodSubnet:     in.PodNet,
			ServiceSubnet: in.ServiceNet,
			CNI:           in.CNIConfig,
		},
		ClusterCA:              in.Certs.K8s,
		ClusterAggregatorCA:    in.Certs.K8sAggregator,
		ClusterServiceAccount:  in.Certs.K8sServiceAccount,
		BootstrapToken:         in.Secrets.BootstrapToken,
		ExtraManifests:         []string{},
		ClusterInlineManifests: v1alpha1.ClusterInlineManifests{},
	}

	if in.AllowSchedulingOnControlPlanes {
		if in.VersionContract.KubernetesAllowSchedulingOnControlPlanes() {
			cluster.AllowSchedulingOnControlPlanes = pointer.To(in.AllowSchedulingOnControlPlanes)
		} else {
			// backwards compatibility for Talos versions older than 1.2
			cluster.AllowSchedulingOnMasters = pointer.To(in.AllowSchedulingOnControlPlanes) //nolint:staticcheck
		}
	}

	if in.DiscoveryEnabled {
		cluster.ClusterDiscoveryConfig = &v1alpha1.ClusterDiscoveryConfig{
			DiscoveryEnabled: pointer.To(in.DiscoveryEnabled),
		}

		if in.VersionContract.KubernetesDiscoveryBackendDisabled() {
			cluster.ClusterDiscoveryConfig.DiscoveryRegistries.RegistryKubernetes.RegistryDisabled = pointer.To(true)
		}
	}

	if !in.VersionContract.PodSecurityPolicyEnabled() {
		cluster.APIServerConfig.DisablePodSecurityPolicyConfig = pointer.To(true)
	}

	if in.VersionContract.SecretboxEncryptionSupported() {
		cluster.ClusterSecretboxEncryptionSecret = in.Secrets.SecretboxEncryptionSecret
	} else {
		cluster.ClusterAESCBCEncryptionSecret = in.Secrets.AESCBCEncryptionSecret
	}

	if machine.MachineRegistries.RegistryMirrors == nil {
		machine.MachineRegistries.RegistryMirrors = map[string]*v1alpha1.RegistryMirrorConfig{}
	}

	if in.VersionContract.KubernetesAlternateImageRegistries() {
		if _, ok := machine.MachineRegistries.RegistryMirrors["k8s.gcr.io"]; !ok {
			machine.MachineRegistries.RegistryMirrors["k8s.gcr.io"] = &v1alpha1.RegistryMirrorConfig{
				MirrorEndpoints: []string{
					"https://registry.k8s.io",
					"https://k8s.gcr.io",
				},
			}
		}
	}

	config.MachineConfig = machine
	config.ClusterConfig = cluster

	return config, nil
}
