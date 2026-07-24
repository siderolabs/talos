// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8s provides Kubernetes-related config documents.
package k8s

//go:generate go tool github.com/siderolabs/talos/tools/docgen -output k8s_doc.go admission_control.go aggregator_ca.go apiserver.go apiserver_ca.go audit_policy.go authentication.go authorization.go cluster.go common.go controller_manager.go coredns.go credential_provider.go etcd_encryption.go external_manifest.go flannel.go inline_manifest.go kubelet.go kubeprism.go network.go node.go proxy.go scheduler.go service_account.go static_pod.go talos_api_access.go

//go:generate go tool github.com/siderolabs/deep-copy -type KubeAdmissionControlConfigV1Alpha1 -type KubeAggregatorCAConfigV1Alpha1 -type KubeAPIServerCAConfigV1Alpha1 -type KubeAPIServerConfigV1Alpha1 -type KubeAuditPolicyConfigV1Alpha1 -type KubeAuthenticationConfigV1Alpha1 -type KubeAuthorizerConfigV1Alpha1 -type KubeClusterConfigV1Alpha1 -type KubeControllerManagerConfigV1Alpha1 -type KubeCoreDNSConfigV1Alpha1 -type KubeEtcdEncryptionConfigV1Alpha1 -type KubeExternalManifestConfigV1Alpha1 -type KubeFlannelCNIConfigV1Alpha1 -type KubeInlineManifestConfigV1Alpha1 -type KubePrismConfigV1Alpha1 -type KubeletConfigV1Alpha1 -type KubeNetworkConfigV1Alpha1 -type KubeNodeConfigV1Alpha1 -type KubeProxyConfigV1Alpha1 -type KubeSchedulerConfigV1Alpha1 -type KubeServiceAccountConfigV1Alpha1 -type KubeCredentialProviderConfigV1Alpha1 -type KubeStaticPodConfigV1Alpha1 -type KubeTalosAPIAccessConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
