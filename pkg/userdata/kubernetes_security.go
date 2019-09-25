/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// KubernetesSecurity represents the set of security options specific to
// Kubernetes.
type KubernetesSecurity struct {
	CA                     *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
	SA                     *x509.PEMEncodedCertificateAndKey `yaml:"sa"`
	FrontProxy             *x509.PEMEncodedCertificateAndKey `yaml:"frontproxy"`
	Etcd                   *x509.PEMEncodedCertificateAndKey `yaml:"etcd"`
	AESCBCEncryptionSecret string                            `yaml:"aescbcEncryptionSecret"`
}

// KubernetesSecurityCheck defines the function type for checks
type KubernetesSecurityCheck func(*KubernetesSecurity) error

// Validate triggers the specified validation checks to run
func (k *KubernetesSecurity) Validate(checks ...KubernetesSecurityCheck) error {
	var result *multierror.Error

	for _, check := range checks {
		result = multierror.Append(result, check(k))
	}

	return result.ErrorOrNil()
}

// CheckKubernetesCA verfies the KubernetesSecurity settings are valid
func CheckKubernetesCA() KubernetesSecurityCheck {
	return func(k *KubernetesSecurity) error {
		certs := []certTest{
			{
				Cert:     k.CA,
				Path:     "security.kubernetes.ca",
				Required: true,
			},
			{
				Cert: k.SA,
				Path: "security.kubernetes.sa",
			},
			{
				Cert: k.FrontProxy,
				Path: "security.kubernetes.frontproxy",
			},
			{
				Cert: k.Etcd,
				Path: "security.kubernetes.etcd",
			},
		}

		return checkCertKeyPair(certs)
	}
}
