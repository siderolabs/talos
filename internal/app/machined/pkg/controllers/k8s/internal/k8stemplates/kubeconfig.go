// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// KubeconfigInClusterTemplate generates a ConfigMap containing the kubeconfig for in-cluster access.
func KubeconfigInClusterTemplate(spec *k8s.BootstrapManifestsConfigSpec) runtime.Object {
	cfg := clientcmdapi.Config{
		APIVersion: "v1",
		Clusters: map[string]*clientcmdapi.Cluster{
			"local": {
				Server:               spec.Server,
				CertificateAuthority: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"service-account": {
				TokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"local": {
				Cluster:  "local",
				AuthInfo: "service-account",
			},
		},
		CurrentContext: "local",
	}

	kubeconfig, err := clientcmd.Write(cfg)
	if err != nil {
		panic(err) // This should never happen, as the config is valid.
	}

	return &corev1.ConfigMap{
		TypeMeta: v1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "kubeconfig-in-cluster",
			Namespace: "kube-system",
		},
		Data: map[string]string{
			"kubeconfig": string(kubeconfig),
		},
	}
}
