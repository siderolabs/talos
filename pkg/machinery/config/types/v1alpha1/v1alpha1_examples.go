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

func machineFeaturesExample() *FeaturesConfig {
	return &FeaturesConfig{
		DiskQuotaSupport: new(true),
	}
}

func clusterConfigExample() any {
	return struct {
		ControlPlane *ControlPlaneConfig `yaml:"controlPlane"`
		ClusterName  string              `yaml:"clusterName"`
	}{
		ControlPlane: clusterControlPlaneExample(),
		ClusterName:  "talos.local",
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
	}
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
