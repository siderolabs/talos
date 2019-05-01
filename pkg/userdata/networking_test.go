package userdata

import (
	"github.com/hashicorp/go-multierror"
	"golang.org/x/xerrors"
)

func (suite *validateSuite) TestValidateDevice() {
	var err error

	// Test for missing required sections
	dev := &Device{}
	err = dev.Validate(CheckDeviceInterface())
	suite.Require().Error(err)
	// Embedding the check in suite.Assert().Equal(true, xerrors.Is had issues )
	if !xerrors.Is(err.(*multierror.Error).Errors[0], ErrRequiredSection) {
		suite.T().Errorf("%+v", err)

	}

	dev.Interface = "eth0"
	err = dev.Validate(CheckDeviceInterface())
	suite.Require().NoError(err)

	err = dev.Validate(CheckDeviceAddressing())
	suite.Require().Error(err)

	// Ensure only a single addressing scheme is specified
	dev.DHCP = true
	dev.CIDR = "1.0.0.1/32"
	err = dev.Validate(CheckDeviceAddressing())
	suite.Require().Error(err)

	dev.DHCP = false
	err = dev.Validate(CheckDeviceAddressing())
	suite.Require().NoError(err)

	dev.Routes = []Route{}
	err = dev.Validate(CheckDeviceRoutes())
	suite.Require().NoError(err)

	dev.Routes = []Route{Route{Gateway: "yolo"}}
	err = dev.Validate(CheckDeviceRoutes())
	suite.Require().Error(err)

	dev.Routes = []Route{Route{Gateway: "yolo", Network: "totes"}}
	err = dev.Validate(CheckDeviceRoutes())
	suite.Require().Error(err)

	dev.Routes = []Route{Route{Gateway: "192.168.1.1", Network: "192.168.1.0/24"}}
	err = dev.Validate(CheckDeviceRoutes())
	suite.Require().NoError(err)
}

/*
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
*/
