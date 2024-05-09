// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"fmt"
	"net/url"

	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	v1alpha1 "github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//nolint:gocyclo,cyclop
func (in *Input) init() ([]config.Document, error) {
	v1alpha1Config := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ConfigDebug:   pointer.To(in.Options.Debug),
		ConfigPersist: pointer.To(in.Options.Persist),
	}

	networkConfig := &v1alpha1.NetworkConfig{}

	for _, opt := range in.Options.NetworkConfigOptions {
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
		MachineCA:       in.Options.SecretsBundle.Certs.OS,
		MachineCertSANs: in.AdditionalMachineCertSANs,
		MachineToken:    in.Options.SecretsBundle.TrustdInfo.Token,
		MachineInstall: &v1alpha1.InstallConfig{
			InstallDisk:            in.Options.InstallDisk,
			InstallImage:           in.Options.InstallImage,
			InstallWipe:            pointer.To(false),
			InstallExtraKernelArgs: in.Options.InstallExtraKernelArgs,
		},
		MachineRegistries: v1alpha1.RegistriesConfig{
			RegistryMirrors: in.Options.RegistryMirrors,
			RegistryConfig:  in.Options.RegistryConfig,
		},
		MachineDisks:                in.Options.MachineDisks,
		MachineSystemDiskEncryption: in.Options.SystemDiskEncryptionConfig,
		MachineSysctls:              in.Options.Sysctls,
		MachineFeatures:             &v1alpha1.FeaturesConfig{},
	}

	machine.MachineFeatures.RBAC = pointer.To(true)

	if in.Options.VersionContract.StableHostnameEnabled() {
		machine.MachineFeatures.StableHostname = pointer.To(true)
	}

	if in.Options.VersionContract.ApidExtKeyUsageCheckEnabled() {
		machine.MachineFeatures.ApidCheckExtKeyUsage = pointer.To(true)
	}

	if in.Options.VersionContract.DiskQuotaSupportEnabled() {
		machine.MachineFeatures.DiskQuotaSupport = pointer.To(true)
	}

	if kubePrismPort, optionSet := in.Options.KubePrismPort.Get(); optionSet { // default to enabled, but if set explicitly, allow it to be disabled
		if kubePrismPort > 0 {
			machine.MachineFeatures.KubePrismSupport = &v1alpha1.KubePrism{
				ServerEnabled: pointer.To(true),
				ServerPort:    kubePrismPort,
			}
		}
	} else if in.Options.VersionContract.KubePrismEnabled() {
		machine.MachineFeatures.KubePrismSupport = &v1alpha1.KubePrism{
			ServerEnabled: pointer.To(true),
			ServerPort:    constants.DefaultKubePrismPort,
		}
	}

	if in.Options.VersionContract.KubeletDefaultRuntimeSeccompProfileEnabled() {
		machine.MachineKubelet.KubeletDefaultRuntimeSeccompProfileEnabled = pointer.To(true)
	}

	if in.Options.VersionContract.KubeletManifestsDirectoryDisabled() {
		machine.MachineKubelet.KubeletDisableManifestsDirectory = pointer.To(true)
	}

	if in.Options.VersionContract.HostDNSEnabled() {
		machine.MachineFeatures.HostDNSSupport = &v1alpha1.HostDNSConfig{
			HostDNSEnabled:              pointer.To(true),
			HostDNSForwardKubeDNSToHost: in.Options.HostDNSForwardKubeDNSToHost.Ptr(),
		}
	}

	certSANs := in.GetAPIServerSANs()

	controlPlaneURL, err := url.Parse(in.ControlPlaneEndpoint)
	if err != nil {
		return nil, err
	}

	var admissionControlConfig []*v1alpha1.AdmissionPluginConfig

	if in.Options.VersionContract.PodSecurityAdmissionEnabled() {
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

	if in.Options.VersionContract.APIServerAuditPolicySupported() {
		auditPolicyConfig = v1alpha1.APIServerDefaultAuditPolicy
	}

	cluster := &v1alpha1.ClusterConfig{
		ClusterID:     in.Options.SecretsBundle.Cluster.ID,
		ClusterName:   in.ClusterName,
		ClusterSecret: in.Options.SecretsBundle.Cluster.Secret,
		ControlPlane: &v1alpha1.ControlPlaneConfig{
			Endpoint:           &v1alpha1.Endpoint{URL: controlPlaneURL},
			LocalAPIServerPort: in.Options.LocalAPIServerPort,
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
			RootCA: in.Options.SecretsBundle.Certs.Etcd,
		},
		ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
			DNSDomain:     in.Options.DNSDomain,
			PodSubnet:     in.PodNet,
			ServiceSubnet: in.ServiceNet,
			CNI:           in.Options.CNIConfig,
		},
		ClusterCA:              in.Options.SecretsBundle.Certs.K8s,
		ClusterAggregatorCA:    in.Options.SecretsBundle.Certs.K8sAggregator,
		ClusterServiceAccount:  in.Options.SecretsBundle.Certs.K8sServiceAccount,
		BootstrapToken:         in.Options.SecretsBundle.Secrets.BootstrapToken,
		ExtraManifests:         []string{},
		ClusterInlineManifests: v1alpha1.ClusterInlineManifests{},
	}

	if in.Options.AllowSchedulingOnControlPlanes {
		if in.Options.VersionContract.KubernetesAllowSchedulingOnControlPlanes() {
			cluster.AllowSchedulingOnControlPlanes = pointer.To(in.Options.AllowSchedulingOnControlPlanes)
		} else {
			// backwards compatibility for Talos versions older than 1.2
			cluster.AllowSchedulingOnMasters = pointer.To(in.Options.AllowSchedulingOnControlPlanes) //nolint:staticcheck
		}
	}

	if in.Options.DiscoveryEnabled != nil {
		cluster.ClusterDiscoveryConfig = &v1alpha1.ClusterDiscoveryConfig{
			DiscoveryEnabled: pointer.To(*in.Options.DiscoveryEnabled),
		}

		if in.Options.VersionContract.KubernetesDiscoveryBackendDisabled() {
			cluster.ClusterDiscoveryConfig.DiscoveryRegistries.RegistryKubernetes.RegistryDisabled = pointer.To(true)
		}
	}

	cluster.APIServerConfig.DisablePodSecurityPolicyConfig = pointer.To(true)

	if in.Options.VersionContract.SecretboxEncryptionSupported() {
		cluster.ClusterSecretboxEncryptionSecret = in.Options.SecretsBundle.Secrets.SecretboxEncryptionSecret
	} else {
		cluster.ClusterAESCBCEncryptionSecret = in.Options.SecretsBundle.Secrets.AESCBCEncryptionSecret
	}

	if machine.MachineRegistries.RegistryMirrors == nil {
		machine.MachineRegistries.RegistryMirrors = map[string]*v1alpha1.RegistryMirrorConfig{}
	}

	if in.Options.VersionContract.KubernetesAlternateImageRegistries() {
		if _, ok := machine.MachineRegistries.RegistryMirrors["k8s.gcr.io"]; !ok {
			machine.MachineRegistries.RegistryMirrors["k8s.gcr.io"] = &v1alpha1.RegistryMirrorConfig{
				MirrorEndpoints: []string{
					"https://registry.k8s.io",
					"https://k8s.gcr.io",
				},
			}
		}
	}

	v1alpha1Config.MachineConfig = machine
	v1alpha1Config.ClusterConfig = cluster

	return []config.Document{v1alpha1Config}, nil
}
