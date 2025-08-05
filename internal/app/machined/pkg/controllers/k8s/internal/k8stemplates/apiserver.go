// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apiserverv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"

	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// APIServerEncryptionConfig returns the encryption configuration for the API server.
func APIServerEncryptionConfig(rootK8sSecrets *secrets.KubernetesRootSpec) runtime.Object {
	obj := apiserverv1.EncryptionConfiguration{
		TypeMeta: v1.TypeMeta{
			Kind:       "EncryptionConfig",
			APIVersion: apiserverv1.SchemeGroupVersion.Version,
		},
		Resources: []apiserverv1.ResourceConfiguration{
			{
				Resources: []string{"secrets"},
				Providers: []apiserverv1.ProviderConfiguration{},
			},
		},
	}

	if rootK8sSecrets.SecretboxEncryptionSecret != "" {
		obj.Resources[0].Providers = append(obj.Resources[0].Providers, apiserverv1.ProviderConfiguration{
			Secretbox: &apiserverv1.SecretboxConfiguration{
				Keys: []apiserverv1.Key{
					{
						Name:   "key2",
						Secret: rootK8sSecrets.SecretboxEncryptionSecret,
					},
				},
			},
		})
	}

	if rootK8sSecrets.AESCBCEncryptionSecret != "" {
		obj.Resources[0].Providers = append(obj.Resources[0].Providers, apiserverv1.ProviderConfiguration{
			AESCBC: &apiserverv1.AESConfiguration{
				Keys: []apiserverv1.Key{
					{
						Name:   "key1",
						Secret: rootK8sSecrets.AESCBCEncryptionSecret,
					},
				},
			},
		})
	}

	obj.Resources[0].Providers = append(obj.Resources[0].Providers, apiserverv1.ProviderConfiguration{
		Identity: &apiserverv1.IdentityConfiguration{},
	})

	return &obj
}
