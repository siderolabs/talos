/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// Certificate represents the set of security options.
type Certificate struct {
	CA *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
}

// CertificateCheck defines the function type for checks
type CertificateCheck func(*Certificate) error

// Validate triggers the specified validation checks to run
func (o *Certificate) Validate(checks ...CertificateCheck) error {
	var result *multierror.Error

	for _, check := range checks {
		result = multierror.Append(result, check(o))
	}

	return result.ErrorOrNil()
}

// CheckCA verfies the Certificate settings are valid
func CheckCA(path string) CertificateCheck {
	return func(o *Certificate) error {
		certs := []certTest{
			{
				Cert:     o.CA,
				Path:     path,
				Required: true,
			},
		}

		return checkCertKeyPair(certs)
	}
}
