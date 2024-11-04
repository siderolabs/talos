// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	etcdctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

func TestMemberSuite(t *testing.T) {
	t.Parallel()

	ctrl := &etcdctrl.MemberController{}

	suite.Run(t, &MemberSuite{
		ctrl: ctrl,
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(ctrl))
			},
		},
	})
}

type MemberSuite struct {
	ctest.DefaultSuite

	ctrl *etcdctrl.MemberController
}

func (suite *MemberSuite) assertEtcdMember(member *etcd.Member) func() error {
	return func() error {
		r, err := ctest.Get[*etcd.Member](suite, member.Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				return retry.ExpectedError(err)
			}

			return err
		}

		spec := r.TypedSpec()
		expectedSpec := member.TypedSpec()

		suite.Require().Equal(expectedSpec.MemberID, spec.MemberID)

		return nil
	}
}

func (suite *MemberSuite) assertInexistentEtcdMember(member *etcd.Member) func() error {
	return func() error {
		_, err := suite.State().Get(suite.Ctx(), member.Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return retry.ExpectedError(err)
		}

		return retry.ExpectedErrorf("should not exist")
	}
}

func (suite *MemberSuite) TestEtcdRunning() {
	// given
	suite.ctrl.GetLocalMemberIDFunc = func(ctx context.Context) (uint64, error) {
		return 123, nil
	}
	etcdService := v1alpha1.NewService("etcd")
	etcdService.TypedSpec().Running = true
	etcdService.TypedSpec().Healthy = true

	// when
	suite.Require().NoError(suite.State().Create(suite.Ctx(), etcdService))

	// then
	expectedMember := etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID)
	expectedMember.TypedSpec().MemberID = "000000000000007b"

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertEtcdMember(expectedMember),
	),
	)
}

func (suite *MemberSuite) TestEtcdNotRunning() {
	// given
	suite.ctrl.GetLocalMemberIDFunc = func(ctx context.Context) (uint64, error) {
		return 123, nil
	}
	etcdService := v1alpha1.NewService("etcd")
	etcdService.TypedSpec().Running = false

	// when
	suite.Require().NoError(suite.State().Create(suite.Ctx(), etcdService))

	// then
	expectedMember := etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID)
	expectedMember.TypedSpec().MemberID = ""

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertInexistentEtcdMember(expectedMember),
	),
	)
}

func (suite *MemberSuite) TestCleanup() {
	// given
	suite.ctrl.GetLocalMemberIDFunc = func(ctx context.Context) (uint64, error) {
		return 123, nil
	}
	etcdService := v1alpha1.NewService("etcd")
	etcdService.TypedSpec().Running = true
	etcdService.TypedSpec().Healthy = true

	expectedMember := etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID)
	expectedMember.TypedSpec().MemberID = "000000000000007b"

	suite.Require().NoError(suite.State().Create(suite.Ctx(), etcdService))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertEtcdMember(expectedMember),
	),
	)

	// when
	okToDestroy, err := suite.State().Teardown(suite.Ctx(), etcdService.Metadata())
	suite.Require().NoError(err)
	suite.Require().True(okToDestroy)

	// then
	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertInexistentEtcdMember(expectedMember),
	))
}
