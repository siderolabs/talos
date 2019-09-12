/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// OSSecurity represents the set of security options specific to the OS.
type OSSecurity struct {
	CA       *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
	Identity *x509.PEMEncodedCertificateAndKey `yaml:"identity"`
}

// OSSecurityCheck defines the function type for checks
type OSSecurityCheck func(*OSSecurity) error

// Validate triggers the specified validation checks to run
func (o *OSSecurity) Validate(checks ...OSSecurityCheck) error {
	var result *multierror.Error

	for _, check := range checks {
		result = multierror.Append(result, check(o))
	}

	return result.ErrorOrNil()
}

// CheckOSCA verfies the OSSecurity settings are valid
func CheckOSCA() OSSecurityCheck {
	return func(o *OSSecurity) error {
		certs := []certTest{
			{
				Cert:     o.CA,
				Path:     "security.os.ca",
				Required: true,
			},
		}

		return checkCertKeyPair(certs)
	}
}
