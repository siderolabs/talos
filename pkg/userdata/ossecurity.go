package userdata

import (
	"encoding/pem"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"golang.org/x/xerrors"
)

// OSSecurity represents the set of security options specific to the OS.
type OSSecurity struct {
	CA       *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
	Identity *x509.PEMEncodedCertificateAndKey `yaml:"identity"`
}

type OSSecurityCheck func(*OSSecurity) error

func (o *OSSecurity) Validate(checks ...OSSecurityCheck) error {
	var result *multierror.Error

	for _, check := range checks {
		result = multierror.Append(result, check(o))
	}

	return result.ErrorOrNil()
}

func CheckOSCA() OSSecurityCheck {
	return func(o *OSSecurity) error {
		var result *multierror.Error

		// Verify the required sections are present
		if o.CA == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "security.os.ca", "", ErrRequiredSection))
		}

		// Bail early since we're already missing the required sections
		if result.ErrorOrNil() != nil {
			return result.ErrorOrNil()
		}

		if o.CA.Crt == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "security.os.ca.crt", "", ErrRequiredSection))
		}

		if o.CA.Key == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "security.os.ca.key", "", ErrRequiredSection))
		}

		// test if o.CA fields are present ( x509 package handles the b64 decode
		// during yaml unmarshal, so we have the bytes if it was successful )
		var block *pem.Block
		block, _ = pem.Decode(o.CA.Crt)
		if block == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "security.os.ca.crt", o.CA.Crt, ErrInvalidCert))
		} else {
			if block.Type != "CERTIFICATE" {
				result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "security.os.ca.crt", o.CA.Crt, ErrInvalidCertType))
			}
		}

		block, _ = pem.Decode(o.CA.Key)
		if block == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "security.os.ca.key", o.CA.Key, ErrInvalidCert))
		} else {
			if !strings.HasSuffix(block.Type, "PRIVATE KEY") {
				result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", "security.os.ca.key", o.CA.Key, ErrInvalidCertType))
			}
		}

		return result.ErrorOrNil()
	}
}
