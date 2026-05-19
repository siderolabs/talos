// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"net/url"
	"strings"
	"time"

	"github.com/siderolabs/crypto/x509"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func mustParseURL(uri string) *url.URL {
	u, err := url.Parse(uri)
	if err != nil {
		panic(err)
	}

	return u
}

// this is using custom type to avoid generating full example with all the nested structs.
func configExample() any {
	return struct {
		Version string `yaml:"version"`
		Machine *yaml.Node
		Cluster *yaml.Node
	}{
		Version: "v1alpha1",
		Machine: &yaml.Node{Kind: yaml.ScalarNode, LineComment: "..."},
		Cluster: &yaml.Node{Kind: yaml.ScalarNode, LineComment: "..."},
	}
}

func machineConfigExample() any {
	return struct {
		Type    string
		Install *InstallConfig
	}{
		Type:    machine.TypeControlPlane.String(),
		Install: machineInstallExample(),
	}
}

func pemEncodedCertificateExample() *x509.PEMEncodedCertificateAndKey {
	return &x509.PEMEncodedCertificateAndKey{
		Crt: []byte("--- EXAMPLE CERTIFICATE ---"),
		Key: []byte("--- EXAMPLE KEY ---"),
	}
}

func pemEncodedKeyExample() *x509.PEMEncodedKey {
	return &x509.PEMEncodedKey{
		Key: []byte("--- EXAMPLE KEY ---"),
	}
}

func machineKubeletExample() *KubeletConfig {
	return &KubeletConfig{
		KubeletImage: (&KubeletConfig{}).Image(),
		KubeletExtraArgs: meta.Args{
			"feature-gates": meta.NewArgValue("ServerSideApply=true", nil),
		},
	}
}

func kubeletImageExample() string {
	return (&KubeletConfig{}).Image()
}

func machineInstallExample() *InstallConfig {
	return &InstallConfig{
		InstallDisk:              "/dev/sda",
		InstallImage:             "factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:latest",
		InstallWipe:              new(false),
		InstallGrubUseUKICmdline: new(true),
	}
}

func machineInstallDiskSelectorExample() *InstallDiskSelector {
	return &InstallDiskSelector{
		Model: "WDC*",
		Size: &InstallDiskSizeMatcher{
			condition: ">= 1TB",
		},
	}
}

func machineInstallDiskSizeMatcherExamples0() *InstallDiskSizeMatcher {
	return &InstallDiskSizeMatcher{
		condition: "4GB",
	}
}

func machineInstallDiskSizeMatcherExamples1() *InstallDiskSizeMatcher {
	return &InstallDiskSizeMatcher{
		condition: "> 1TB",
	}
}

func machineInstallDiskSizeMatcherExamples2() *InstallDiskSizeMatcher {
	return &InstallDiskSizeMatcher{
		condition: "<= 2TB",
	}
}

func machineFilesExample() []*MachineFile {
	return []*MachineFile{
		{
			FileContent:     "...",
			FilePermissions: 0o666,
			FilePath:        "/tmp/file.txt",
			FileOp:          "append",
		},
	}
}

func machineSysctlsExample() map[string]string {
	return map[string]string{
		"kernel.domainname":                   "talos.dev",
		"net.ipv4.ip_forward":                 "0",
		"net/ipv6/conf/eth0.100/disable_ipv6": "1",
	}
}

func machineSysfsExample() map[string]string {
	return map[string]string{
		"devices.system.cpu.cpu0.cpufreq.scaling_governor": "performance",
	}
}

func machineFeaturesExample() *FeaturesConfig {
	return &FeaturesConfig{
		DiskQuotaSupport: new(true),
	}
}

func machineUdevExample() *UdevConfig {
	return &UdevConfig{
		UdevRules: []string{"SUBSYSTEM==\"drm\", KERNEL==\"renderD*\", GROUP=\"44\", MODE=\"0660\""},
	}
}

func clusterConfigExample() any {
	return struct {
		ControlPlane *ControlPlaneConfig   `yaml:"controlPlane"`
		ClusterName  string                `yaml:"clusterName"`
		Network      *ClusterNetworkConfig `yaml:"network"`
	}{
		ControlPlane: clusterControlPlaneExample(),
		ClusterName:  "talos.local",
		Network:      clusterNetworkExample(),
	}
}

func clusterControlPlaneExample() *ControlPlaneConfig {
	return &ControlPlaneConfig{
		Endpoint: &Endpoint{
			&url.URL{
				Host:   "1.2.3.4",
				Scheme: "https",
			},
		},
		LocalAPIServerPort: 443,
	}
}

func clusterNetworkExample() *ClusterNetworkConfig {
	return &ClusterNetworkConfig{
		CNI: &CNIConfig{
			CNIName: constants.FlannelCNI,
		},
		DNSDomain:     "cluster.local",
		PodSubnet:     []string{"10.244.0.0/16"},
		ServiceSubnet: []string{"10.96.0.0/12"},
	}
}

func resourcesConfigRequestsExample() meta.Unstructured {
	return meta.Unstructured{
		Object: map[string]any{
			"cpu":    1,
			"memory": "1Gi",
		},
	}
}

func resourcesConfigLimitsExample() meta.Unstructured {
	return meta.Unstructured{
		Object: map[string]any{
			"cpu":    2,
			"memory": "2500Mi",
		},
	}
}

func clusterAPIServerExample() *APIServerConfig {
	return &APIServerConfig{
		ContainerImage: (&APIServerConfig{}).Image(),
		ExtraArgsConfig: meta.Args{
			"feature-gates":                    meta.NewArgValue("ServerSideApply=true", nil),
			"http2-max-streams-per-connection": meta.NewArgValue("32", nil),
		},
		CertSANs: []string{
			"1.2.3.4",
			"4.5.6.7",
		},
	}
}

func clusterAPIServerImageExample() string {
	return (&APIServerConfig{}).Image()
}

func clusterControllerManagerExample() *ControllerManagerConfig {
	return &ControllerManagerConfig{
		ContainerImage: (&ControllerManagerConfig{}).Image(),
		ExtraArgsConfig: meta.Args{
			"feature-gates": meta.NewArgValue("ServerSideApply=true", nil),
		},
	}
}

func clusterControllerManagerImageExample() string {
	return (&ControllerManagerConfig{}).Image()
}

func clusterProxyExample() *ProxyConfig {
	return &ProxyConfig{
		ContainerImage: (&ProxyConfig{}).Image(),
		ExtraArgsConfig: meta.Args{
			"proxy-mode": meta.NewArgValue("iptables", nil),
		},
		ModeConfig: "ipvs",
	}
}

func clusterProxyImageExample() string {
	return (&ProxyConfig{}).Image()
}

func clusterEtcdExample() *EtcdConfig {
	return &EtcdConfig{
		ContainerImage: (&EtcdConfig{}).Image(),
		EtcdExtraArgs: meta.Args{
			"election-timeout": meta.NewArgValue("5000", nil),
		},
		RootCA: pemEncodedCertificateExample(),
	}
}

func clusterEtcdImageExample() string {
	return (&EtcdConfig{}).Image()
}

func clusterEtcdAdvertisedSubnetsExample() []string {
	return []string{"10.0.0.0/8"}
}

func clusterCoreDNSExample() *CoreDNS {
	return &CoreDNS{
		CoreDNSImage: (&CoreDNS{}).Image(),
	}
}

func clusterExternalCloudProviderConfigExample() *ExternalCloudProviderConfig {
	return &ExternalCloudProviderConfig{
		ExternalEnabled: new(true),
		ExternalManifests: []string{
			"https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/rbac.yaml",
			"https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/aws-cloud-controller-manager-daemonset.yaml",
		},
	}
}

func clusterAdminKubeconfigExample() *AdminKubeconfigConfig {
	return &AdminKubeconfigConfig{
		AdminKubeconfigCertLifetime: time.Hour,
	}
}

func machineSeccompExample() []*MachineSeccompProfile {
	return []*MachineSeccompProfile{
		{
			MachineSeccompProfileName: "audit.json",
			MachineSeccompProfileValue: meta.Unstructured{
				Object: map[string]any{
					"defaultAction": "SCMP_ACT_LOG",
				},
			},
		},
	}
}

func kubeletExtraMountsExample() []ExtraMount {
	return []ExtraMount{
		{
			Source:      "/var/lib/example",
			Destination: "/var/lib/example",
			Type:        "bind",
			Options: []string{
				"bind",
				"rshared",
				"rw",
			},
		},
	}
}

func clusterCustomCNIExample() *CNIConfig {
	return &CNIConfig{
		CNIName: constants.CustomCNI,
		CNIUrls: []string{
			"https://raw.githubusercontent.com/projectcalico/calico/v3.31.5/manifests/canal.yaml",
		},
	}
}

func clusterInlineManifestsExample() ClusterInlineManifests {
	return ClusterInlineManifests{
		{
			InlineManifestName: "namespace-ci",
			InlineManifestContents: strings.TrimSpace(`
apiVersion: v1
kind: Namespace
metadata:
	name: ci
`),
		},
	}
}

func clusterDiscoveryExample() ClusterDiscoveryConfig {
	return ClusterDiscoveryConfig{
		DiscoveryEnabled: new(true),
		DiscoveryRegistries: DiscoveryRegistriesConfig{
			RegistryService: RegistryServiceConfig{
				RegistryEndpoint: constants.DefaultDiscoveryServiceEndpoint,
			},
		},
	}
}

func kubeletNodeIPExample() *KubeletNodeIPConfig {
	return &KubeletNodeIPConfig{
		KubeletNodeIPValidSubnets: []string{
			"10.0.0.0/8",
			"!10.0.0.3/32",
			"fdc7::/16",
		},
	}
}

func kubeletExtraConfigExample() meta.Unstructured {
	return meta.Unstructured{
		Object: map[string]any{
			"serverTLSBootstrap": true,
		},
	}
}

func kubeletCredentialProviderConfigExample() meta.Unstructured {
	return meta.Unstructured{
		Object: map[string]any{
			"apiVersion": "kubelet.config.k8s.io/v1",
			"kind":       "CredentialProviderConfig",
			"providers": []any{
				map[string]any{
					"name":       "ecr-credential-provider",
					"apiVersion": "credentialprovider.kubelet.k8s.io/v1",
					"matchImages": []any{
						"*.dkr.ecr.*.amazonaws.com",
						"*.dkr.ecr.*.amazonaws.com.cn",
						"*.dkr.ecr-fips.*.amazonaws.com",
						"*.dkr.ecr.us-iso-east-1.c2s.ic.gov",
						"*.dkr.ecr.us-isob-east-1.sc2s.sgov.gov",
					},
					"defaultCacheDuration": "12h",
				},
			},
		},
	}
}

func machineLoggingExample1() LoggingConfig {
	return LoggingConfig{
		LoggingDestinations: []LoggingDestination{
			{
				LoggingEndpoint: &Endpoint{
					mustParseURL("tcp://1.2.3.4:12345"),
				},
				LoggingFormat: constants.LoggingFormatJSONLines,
			},
		},
	}
}

func machineLoggingExample2() LoggingConfig {
	return LoggingConfig{
		LoggingDestinations: []LoggingDestination{
			{
				LoggingEndpoint: &Endpoint{
					mustParseURL("udp://127.0.0.1:12345"),
				},
				LoggingFormat: constants.LoggingFormatJSONLines,
				LoggingExtraTags: map[string]string{
					"machine": "worker-1",
				},
			},
		},
	}
}

func machineKernelExample() *KernelConfig {
	return &KernelConfig{
		KernelModules: []*KernelModuleConfig{
			{
				ModuleName: "btrfs",
			},
		},
	}
}

func machinePodsExample() []meta.Unstructured {
	return []meta.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "pod",
				"metadata": map[string]any{
					"name": "nginx",
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "nginx",
							"image": "nginx",
						},
					},
				},
			},
		},
	}
}

func admissionControlConfigExample() []*AdmissionPluginConfig {
	return []*AdmissionPluginConfig{
		{
			PluginName: "PodSecurity",
			PluginConfiguration: meta.Unstructured{
				Object: map[string]any{
					"apiVersion": "pod-security.admission.config.k8s.io/v1alpha1",
					"kind":       "PodSecurityConfiguration",
					"defaults": map[string]any{
						"enforce":         "baseline",
						"enforce-version": "latest",
						"audit":           "restricted",
						"audit-version":   "latest",
						"warn":            "restricted",
						"warn-version":    "latest",
					},
					"exemptions": map[string]any{
						"usernames":      []any{},
						"runtimeClasses": []any{},
						"namespaces":     []any{"kube-system"},
					},
				},
			},
		},
	}
}

func authorizationConfigExample() []*AuthorizationConfigAuthorizerConfig {
	return []*AuthorizationConfigAuthorizerConfig{
		{
			AuthorizerType: "Webhook",
			AuthorizerName: "webhook",
			AuthorizerWebhook: meta.Unstructured{
				Object: map[string]any{
					"timeout":                    "3s",
					"subjectAccessReviewVersion": "v1",
					"matchConditionSubjectAccessReviewVersion": "v1",
					"failurePolicy": "Deny",
					"connectionInfo": map[string]any{
						"type": "InClusterConfig",
					},
					"matchConditions": []map[string]any{
						{
							"expression": "has(request.resourceAttributes)",
						},
						{
							"expression": "!(\\'system:serviceaccounts:kube-system\\' in request.groups)",
						},
					},
				},
			},
		},
		{
			AuthorizerType: "Webhook",
			AuthorizerName: "in-cluster-authorizer",
			AuthorizerWebhook: meta.Unstructured{
				Object: map[string]any{
					"timeout":                    "3s",
					"subjectAccessReviewVersion": "v1",
					"matchConditionSubjectAccessReviewVersion": "v1",
					"failurePolicy": "NoOpinion",
					"connectionInfo": map[string]any{
						"type": "InClusterConfig",
					},
				},
			},
		},
	}
}

func kubernetesTalosAPIAccessConfigExample() *KubernetesTalosAPIAccessConfig {
	return &KubernetesTalosAPIAccessConfig{
		AccessEnabled: new(true),
		AccessAllowedRoles: []string{
			"os:reader",
		},
		AccessAllowedKubernetesNamespaces: []string{
			"kube-system",
		},
	}
}

func machineBaseRuntimeSpecOverridesExample() meta.Unstructured {
	return meta.Unstructured{
		Object: map[string]any{
			"process": map[string]any{
				"rlimits": []map[string]any{
					{
						"type": "RLIMIT_NOFILE",
						"hard": 1024,
						"soft": 1024,
					},
				},
			},
		},
	}
}
