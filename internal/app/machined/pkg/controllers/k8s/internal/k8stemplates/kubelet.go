// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// KubeletBootstrapTokenSecret returns the kubelet bootstrap token secret.
func KubeletBootstrapTokenSecret(secrets *secrets.KubernetesRootSpec) runtime.Object {
	return &corev1.Secret{
		TypeMeta: v1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.Version,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "bootstrap-token-" + secrets.BootstrapTokenID,
			Namespace: "kube-system",
		},
		Type: corev1.SecretType("bootstrap.kubernetes.io/token"),
		StringData: map[string]string{
			"token-id":                       secrets.BootstrapTokenID,
			"token-secret":                   secrets.BootstrapTokenSecret,
			"usage-bootstrap-authentication": "true",
			"auth-extra-groups":              "system:bootstrappers:nodes",
		},
	}
}
