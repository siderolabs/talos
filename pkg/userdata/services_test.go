/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"github.com/hashicorp/go-multierror"
	"golang.org/x/xerrors"
)

func (suite *validateSuite) TestValidateServices() {
	var err error

	// Test for missing required sections
	svc := &Services{}
	err = svc.Validate(CheckServices())
	suite.Require().Error(err)
	//  services.kubeadm
	if !xerrors.Is(err.(*multierror.Error).Errors[0], ErrRequiredSection) {
		suite.T().Errorf("%+v", err)
	}
	//  services.trustd
	if !xerrors.Is(err.(*multierror.Error).Errors[1], ErrRequiredSection) {
		suite.T().Errorf("%+v", err)
	}
}

func (suite *validateSuite) TestValidateTrustd() {
	var err error

	svc := &Services{}
	svc.Trustd = &Trustd{}
	err = svc.Trustd.Validate(CheckTrustdAuth(), CheckTrustdEndpoints())
	suite.Require().Error(err)
	suite.Assert().Equal(2, len(err.(*multierror.Error).Errors))

	svc.Trustd.Endpoints = []string{"1.2.3.4"}
	err = svc.Trustd.Validate(CheckTrustdEndpoints())
	suite.Require().NoError(err)

	svc.Trustd.Token = "yolo"
	err = svc.Trustd.Validate(CheckTrustdAuth())
	suite.Require().NoError(err)

	svc.Trustd.Token = ""
	svc.Trustd.Username = "bob"
	svc.Trustd.Password = "burger"
	err = svc.Trustd.Validate(CheckTrustdAuth())
	suite.Require().NoError(err)
}

func (suite *validateSuite) TestValidateInit() {
	var err error

	svc := &Services{}
	svc.Init = &Init{}
	err = svc.Init.Validate(CheckInitCNI())
	suite.Require().Error(err)
	if !xerrors.Is(err.(*multierror.Error).Errors[0], ErrUnsupportedCNI) {
		suite.T().Errorf("%+v", err)
	}

	svc.Init.CNI = "calico"
	err = svc.Init.Validate(CheckInitCNI())
	suite.Require().NoError(err)
}
