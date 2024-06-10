// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type HostnameSpecSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *HostnameSpecSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.Require().NoError(
		suite.runtime.RegisterController(
			&netctrl.HostnameSpecController{
				V1Alpha1Mode: v1alpha1runtime.ModeContainer, // run in container mode to skip _actually_ setting hostname
			},
		),
	)

	suite.startRuntime()
}

func (suite *HostnameSpecSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *HostnameSpecSuite) assertStatus(id string, fqdn string) error {
	r, err := suite.state.Get(
		suite.ctx,
		resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, id, resource.VersionUndefined),
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			return retry.ExpectedError(err)
		}

		return err
	}

	status := r.(*network.HostnameStatus) //nolint:errcheck,forcetypeassert

	if status.TypedSpec().FQDN() != fqdn {
		return retry.ExpectedErrorf("fqdn mismatch: %q != %q", status.TypedSpec().FQDN(), fqdn)
	}

	return nil
}

func (suite *HostnameSpecSuite) TestSpec() {
	spec := network.NewHostnameSpec(network.NamespaceName, "hostname")
	*spec.TypedSpec() = network.HostnameSpecSpec{
		Hostname:    "foo",
		Domainname:  "bar",
		ConfigLayer: network.ConfigDefault,
	}

	for _, res := range []resource.Resource{spec} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertStatus("hostname", "foo.bar")
			},
		),
	)
}

func (suite *HostnameSpecSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestHostnameSpecSuite(t *testing.T) {
	suite.Run(t, new(HostnameSpecSuite))
}
