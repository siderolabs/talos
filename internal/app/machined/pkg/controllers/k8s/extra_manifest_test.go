// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s_test

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	k8sadapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/k8s"
	k8sctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/generic/slices"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	"github.com/talos-systems/talos/pkg/machinery/resources/v1alpha1"
)

type ExtraManifestSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *ExtraManifestSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&k8sctrl.ExtraManifestController{}))

	suite.startRuntime()
}

func (suite *ExtraManifestSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

//nolint:dupl
func (suite *ExtraManifestSuite) assertExtraManifests(manifests []string) error {
	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.ManifestType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	ids := slices.Map(resources.Items, func(r resource.Resource) string { return r.Metadata().ID() })

	if !reflect.DeepEqual(manifests, ids) {
		return retry.ExpectedError(fmt.Errorf("expected %q, got %q", manifests, ids))
	}

	return nil
}

func (suite *ExtraManifestSuite) TestReconcileInlineManifests() {
	configExtraManifests := k8s.NewExtraManifestsConfig()
	*configExtraManifests.TypedSpec() = k8s.ExtraManifestsConfigSpec{
		ExtraManifests: []k8s.ExtraManifest{
			{
				Name:     "namespaces",
				Priority: "99",
				InlineManifest: strings.TrimSpace(
					`
apiVersion: v1
kind: Namespace
metadata:
    name: ci
---
apiVersion: v1
kind: Namespace
metadata:
    name: build
`,
				),
			},
		},
	}

	statusNetwork := network.NewStatus(network.NamespaceName, network.StatusID)
	statusNetwork.TypedSpec().AddressReady = true
	statusNetwork.TypedSpec().ConnectivityReady = true

	suite.Require().NoError(suite.state.Create(suite.ctx, configExtraManifests))
	suite.Require().NoError(suite.state.Create(suite.ctx, statusNetwork))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertExtraManifests(
					[]string{
						"99-namespaces",
					},
				)
			},
		),
	)

	r, err := suite.state.Get(
		suite.ctx,
		resource.NewMetadata(
			k8s.ControlPlaneNamespaceName,
			k8s.ManifestType,
			"99-namespaces",
			resource.VersionUndefined,
		),
	)
	suite.Require().NoError(err)

	manifest := r.(*k8s.Manifest) //nolint:errcheck,forcetypeassert

	suite.Assert().Len(k8sadapter.Manifest(manifest).Objects(), 2)
	suite.Assert().Equal("ci", k8sadapter.Manifest(manifest).Objects()[0].GetName())
	suite.Assert().Equal("build", k8sadapter.Manifest(manifest).Objects()[1].GetName())
}

func (suite *ExtraManifestSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	suite.Assert().NoError(suite.state.Create(context.Background(), v1alpha1.NewService("foo")))
}

func TestExtraManifestSuite(t *testing.T) {
	suite.Run(t, new(ExtraManifestSuite))
}
