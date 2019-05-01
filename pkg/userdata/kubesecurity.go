/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"encoding/pem"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"golang.org/x/xerrors"
)

// KubernetesSecurity represents the set of security options specific to
// Kubernetes.
type KubernetesSecurity struct {
	CA *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
}

type KubeSecurityCheck func(*KubernetesSecurity) error

func (k *KubernetesSecurity) Validate(checks ...KubeSecurityCheck) error {
	var result *multierror.Error

	for _, check := range checks {
		result = multierror.Append(result, check(k))
	}

	return result.ErrorOrNil()
}

func CheckKubeCA() KubeSecurityCheck {
	return func(k *KubernetesSecurity) error {
		var result *multierror.Error

		// Verify the required sections are present
		if k.CA == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "security.kubernetes.ca", "", ErrRequiredSection))
		}

		// Bail early since we're already missing the required sections
		if result.ErrorOrNil() != nil {
			return result.ErrorOrNil()
		}

		if k.CA.Crt == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "security.kubernetes.ca.crt", "", ErrRequiredSection))
		}

		if k.CA.Key == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "security.kubernetes.ca.key", "", ErrRequiredSection))
		}

		// test if k.CA fields are present ( x509 package handles the b64 decode
		// during yaml unmarshal, so we have the bytes if it was successful )
		var block *pem.Block
		block, _ = pem.Decode(k.CA.Crt)
		if block == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "security.kubernetes.ca.crt", k.CA.Crt, ErrInvalidCert))
		} else {
			if block.Type != "CERTIFICATE" {
				result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "security.kubernetes.ca.crt", k.CA.Crt, ErrInvalidCertType))
			}
		}

		block, _ = pem.Decode(k.CA.Key)
		if block == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "security.kubernetes.ca.key", k.CA.Key, ErrInvalidCert))
		} else {
			if !strings.HasSuffix(block.Type, "PRIVATE KEY") {
				result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "security.kubernetes.ca.key", k.CA.Key, ErrInvalidCertType))
			}
		}

		return result.ErrorOrNil()
	}
}
