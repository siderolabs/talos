// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s_test

import (
	"context"
	"io"
	"net/http"
	"strings"
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

	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type StaticPodListSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *StaticPodListSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&k8sctrl.StaticPodServerController{}))

	suite.startRuntime()
}

func (suite *StaticPodListSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *StaticPodListSuite) assertResource(
	md resource.Metadata,
	check func(res resource.Resource) error,
) func() error {
	return func() error {
		r, err := suite.state.Get(suite.ctx, md)
		if err != nil {
			if state.IsNotFoundError(err) {
				return retry.ExpectedError(err)
			}

			return err
		}

		return check(r)
	}
}

func (suite *StaticPodListSuite) getResource(
	md resource.Metadata,
) resource.Resource {
	var ret resource.Resource

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
			r, err := suite.state.Get(suite.ctx, md)
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			ret = r

			return nil
		}))

	return ret
}

func newTestPod(name string) *k8s.StaticPod {
	testPod := k8s.NewStaticPod(k8s.NamespaceName, name)

	testPod.TypedSpec().Pod = map[string]any{
		"metadata": name,
		"spec":     "testSpec",
	}

	return testPod
}

func (suite *StaticPodListSuite) TestCreatesStaticPodServerStatus() {
	// given
	testPod := newTestPod("testPod")

	// when
	suite.Require().NoError(suite.state.Create(suite.ctx, testPod))

	// then
	expectedPodListURL := k8s.NewStaticPodServerStatus(k8s.NamespaceName, k8s.StaticPodServerStatusResourceID)

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertResource(*expectedPodListURL.Metadata(), func(res resource.Resource) error {
				suite.Require().True(strings.HasPrefix(
					res.(*k8s.StaticPodServerStatus).TypedSpec().URL,
					"http://127.0.0.1:",
				),
				)

				return nil
			},
			),
		),
	)
}

func (suite *StaticPodListSuite) TestServesStaticPodList() {
	// given
	testPod1 := newTestPod("testPod1")
	testPod2 := newTestPod("testPod2")

	// when
	suite.Require().NoError(suite.state.Create(suite.ctx, testPod1))
	suite.Require().NoError(suite.state.Create(suite.ctx, testPod2))

	// then
	expectedPodListURL := k8s.NewStaticPodServerStatus(k8s.NamespaceName, k8s.StaticPodServerStatusResourceID)

	podListURL := suite.getResource(*expectedPodListURL.Metadata())

	suite.Require().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
			resp, err := http.Get(podListURL.(*k8s.StaticPodServerStatus).TypedSpec().URL) //nolint:noctx
			if err != nil {
				return retry.ExpectedError(err)
			}

			defer resp.Body.Close() //nolint:errcheck

			content, err := io.ReadAll(resp.Body)
			suite.Assert().NoError(err)

			suite.Require().Equal("kind: PodList\nitems:\n    - metadata: testPod1\n      spec: testSpec\n    - metadata: testPod2\n      spec: testSpec\napiversion: v1\n", string(content))

			return nil
		}),
	)
}

func (suite *StaticPodListSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestStaticPodListSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(StaticPodListSuite))
}
