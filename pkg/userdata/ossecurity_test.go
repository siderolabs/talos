package userdata

import (
	"github.com/hashicorp/go-multierror"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"golang.org/x/xerrors"
)

func (suite *validateSuite) TestValidateOSSecurity() {
	var err error

	// Test for missing required sections
	os := &OSSecurity{}
	err = os.Validate(CheckOSCA())
	suite.Require().Error(err)
	// Embedding the check in suite.Assert().Equal(true, xerrors.Is had issues )
	if !xerrors.Is(err.(*multierror.Error).Errors[0], ErrRequiredSection) {
		suite.T().Errorf("%+v", err)

	}

	os.CA = &x509.PEMEncodedCertificateAndKey{}
	err = os.Validate(CheckOSCA())
	suite.Require().Error(err)
	suite.Assert().Equal(4, len(err.(*multierror.Error).Errors))

	// Test for invalid certs
	os.CA.Crt = []byte("-----BEGIN Rubbish-----\n-----END Rubbish-----")
	os.CA.Key = []byte("-----BEGIN EC Fluffy KEY-----\n-----END EC Fluffy KEY-----")
	err = os.Validate(CheckOSCA())
	suite.Require().Error(err)

	// Successful test
	os.CA.Crt = []byte("-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----")
	os.CA.Key = []byte("-----BEGIN EC PRIVATE KEY-----\n-----END EC PRIVATE KEY-----")
	err = os.Validate(CheckOSCA())
	suite.Require().NoError(err)
}
