/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// nolint: dupl
package userdata

import (
	"github.com/hashicorp/go-multierror"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"golang.org/x/xerrors"
)

func (suite *validateSuite) TestValidateKubernetesSecurity() {
	var err error

	// Test for missing required sections
	kube := &KubernetesSecurity{}
	err = kube.Validate(CheckKubernetesCA())
	suite.Require().Error(err)
	// Embedding the check in suite.Assert().Equal(true, xerrors.Is had issues )
	if !xerrors.Is(err.(*multierror.Error).Errors[0], ErrRequiredSection) {
		suite.T().Errorf("%+v", err)
	}

	kube.CA = &x509.PEMEncodedCertificateAndKey{}
	err = kube.Validate(CheckKubernetesCA())
	suite.Require().Error(err)
	suite.Assert().Equal(4, len(err.(*multierror.Error).Errors))

	// Test for invalid certs
	kube.CA.Crt = []byte("-----BEGIN Rubbish-----\n-----END Rubbish-----")
	kube.CA.Key = []byte("-----BEGIN EC Fluffy KEY-----\n-----END EC Fluffy KEY-----")
	err = kube.Validate(CheckKubernetesCA())
	suite.Require().Error(err)

	// Successful test
	kube.CA = &x509.PEMEncodedCertificateAndKey{
		Crt: []byte("-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----"),
		Key: []byte("-----BEGIN EC PRIVATE KEY-----\n-----END EC PRIVATE KEY-----"),
	}
	kube.SA = &x509.PEMEncodedCertificateAndKey{
		Crt: []byte("-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----"),
		Key: []byte("-----BEGIN EC PRIVATE KEY-----\n-----END EC PRIVATE KEY-----"),
	}
	kube.FrontProxy = &x509.PEMEncodedCertificateAndKey{
		Crt: []byte("-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----"),
		Key: []byte("-----BEGIN EC PRIVATE KEY-----\n-----END EC PRIVATE KEY-----"),
	}
	kube.Etcd = &x509.PEMEncodedCertificateAndKey{
		Crt: []byte("-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----"),
		Key: []byte("-----BEGIN EC PRIVATE KEY-----\n-----END EC PRIVATE KEY-----"),
	}
	err = kube.Validate(CheckKubernetesCA())
	suite.Require().NoError(err)
}
