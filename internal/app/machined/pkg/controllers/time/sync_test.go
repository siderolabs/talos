// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package time_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	timectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/time"
	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	timeresource "github.com/siderolabs/talos/pkg/machinery/resources/time"
	v1alpha1resource "github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

type SyncSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	syncerMu sync.Mutex
	syncer   *mockSyncer
}

func (suite *SyncSuite) State() state.State { return suite.state }

func (suite *SyncSuite) Ctx() context.Context { return suite.ctx }

func (suite *SyncSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	// create fake device ready status
	deviceStatus := runtimeres.NewDevicesStatus(runtimeres.NamespaceName, runtimeres.DevicesID)
	deviceStatus.TypedSpec().Ready = true
	suite.Require().NoError(suite.state.Create(suite.ctx, deviceStatus))
}

func (suite *SyncSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *SyncSuite) assertTimeStatus(spec timeresource.StatusSpec) error {
	r, err := suite.state.Get(
		suite.ctx,
		resource.NewMetadata(
			v1alpha1resource.NamespaceName,
			timeresource.StatusType,
			timeresource.StatusID,
			resource.VersionUndefined,
		),
	)
	if err != nil {
		if state.IsNotFoundError(err) {
			return retry.ExpectedError(err)
		}

		return err
	}

	status := r.(*timeresource.Status) //nolint:forcetypeassert

	if *status.TypedSpec() != spec {
		return retry.ExpectedErrorf("time status doesn't match: %v != %v", *status.TypedSpec(), spec)
	}

	return nil
}

func (suite *SyncSuite) TestReconcileContainerMode() {
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&timectrl.SyncController{
				V1Alpha1Mode: v1alpha1runtime.ModeContainer,
				NewNTPSyncer: suite.newMockSyncer,
			},
		),
	)

	timeServers := network.NewTimeServerStatus(network.NamespaceName, network.TimeServerID)
	timeServers.TypedSpec().NTPServers = []string{constants.DefaultNTPServer}
	suite.Require().NoError(suite.state.Create(suite.ctx, timeServers))

	suite.startRuntime()

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertTimeStatus(
					timeresource.StatusSpec{
						Synced:       true,
						Epoch:        0,
						SyncDisabled: true,
					},
				)
			},
		),
	)
}

func (suite *SyncSuite) TestReconcileSyncDisabled() {
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&timectrl.SyncController{
				V1Alpha1Mode: v1alpha1runtime.ModeMetal,
				NewNTPSyncer: suite.newMockSyncer,
			},
		),
	)

	suite.startRuntime()

	timeServers := network.NewTimeServerStatus(network.NamespaceName, network.TimeServerID)
	timeServers.TypedSpec().NTPServers = []string{constants.DefaultNTPServer}
	suite.Require().NoError(suite.state.Create(suite.ctx, timeServers))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertTimeStatus(
					timeresource.StatusSpec{
						Synced:       false,
						Epoch:        0,
						SyncDisabled: false,
					},
				)
			},
		),
	)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineTime: &v1alpha1.TimeConfig{
						TimeDisabled: pointer.To(true),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{},
			},
		),
	)

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertTimeStatus(
					timeresource.StatusSpec{
						Synced:       true,
						Epoch:        0,
						SyncDisabled: true,
					},
				)
			},
		),
	)
}

func (suite *SyncSuite) TestReconcileSyncDefaultConfig() {
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&timectrl.SyncController{
				V1Alpha1Mode: v1alpha1runtime.ModeMetal,
				NewNTPSyncer: suite.newMockSyncer,
			},
		),
	)

	suite.startRuntime()

	timeServers := network.NewTimeServerStatus(network.NamespaceName, network.TimeServerID)
	timeServers.TypedSpec().NTPServers = []string{constants.DefaultNTPServer}
	suite.Require().NoError(suite.state.Create(suite.ctx, timeServers))

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
				ClusterConfig: &v1alpha1.ClusterConfig{},
			},
		),
	)

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertTimeStatus(
					timeresource.StatusSpec{
						Synced:       false,
						Epoch:        0,
						SyncDisabled: false,
					},
				)
			},
		),
	)
}

func (suite *SyncSuite) TestReconcileSyncChangeConfig() {
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&timectrl.SyncController{
				V1Alpha1Mode: v1alpha1runtime.ModeMetal,
				NewNTPSyncer: suite.newMockSyncer,
			},
		),
	)

	suite.startRuntime()

	timeServers := network.NewTimeServerStatus(network.NamespaceName, network.TimeServerID)
	timeServers.TypedSpec().NTPServers = []string{constants.DefaultNTPServer}
	suite.Require().NoError(suite.state.Create(suite.ctx, timeServers))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertTimeStatus(
					timeresource.StatusSpec{
						Synced:       false,
						Epoch:        0,
						SyncDisabled: false,
					},
				)
			},
		),
	)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
				ClusterConfig: &v1alpha1.ClusterConfig{},
			},
		),
	)

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	var mockSyncer *mockSyncer

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				mockSyncer = suite.getMockSyncer()

				if mockSyncer == nil {
					return retry.ExpectedErrorf("syncer not created yet")
				}

				return nil
			},
		),
	)

	suite.Assert().Equal([]string{constants.DefaultNTPServer}, mockSyncer.getTimeServers())

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertTimeStatus(
					timeresource.StatusSpec{
						Synced:       false,
						Epoch:        0,
						SyncDisabled: false,
					},
				)
			},
		),
	)

	close(mockSyncer.syncedCh)

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertTimeStatus(
					timeresource.StatusSpec{
						Synced:       true,
						Epoch:        0,
						SyncDisabled: false,
					},
				)
			},
		),
	)

	ctest.UpdateWithConflicts(suite, timeServers, func(r *network.TimeServerStatus) error {
		r.TypedSpec().NTPServers = []string{"127.0.0.1"}

		return nil
	})

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				if !slices.Equal(mockSyncer.getTimeServers(), []string{"127.0.0.1"}) {
					return retry.ExpectedErrorf("time servers not updated yet")
				}

				return nil
			},
		),
	)

	mockSyncer.epochCh <- struct{}{}

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertTimeStatus(
					timeresource.StatusSpec{
						Synced:       true,
						Epoch:        1,
						SyncDisabled: false,
					},
				)
			},
		),
	)

	ctest.UpdateWithConflicts(suite, cfg, func(r *config.MachineConfig) error {
		r.Container().RawV1Alpha1().MachineConfig.MachineTime = &v1alpha1.TimeConfig{
			TimeDisabled: pointer.To(true),
		}

		return nil
	})

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertTimeStatus(
					timeresource.StatusSpec{
						Synced:       true,
						Epoch:        1,
						SyncDisabled: true,
					},
				)
			},
		),
	)
}

func (suite *SyncSuite) TestReconcileSyncBootTimeout() {
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&timectrl.SyncController{
				V1Alpha1Mode: v1alpha1runtime.ModeMetal,
				NewNTPSyncer: suite.newMockSyncer,
			},
		),
	)

	suite.startRuntime()

	timeServers := network.NewTimeServerStatus(network.NamespaceName, network.TimeServerID)
	timeServers.TypedSpec().NTPServers = []string{constants.DefaultNTPServer}
	suite.Require().NoError(suite.state.Create(suite.ctx, timeServers))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertTimeStatus(
					timeresource.StatusSpec{
						Synced:       false,
						Epoch:        0,
						SyncDisabled: false,
					},
				)
			},
		),
	)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineTime: &v1alpha1.TimeConfig{
						TimeBootTimeout: 5 * time.Second,
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{},
			},
		),
	)

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertTimeStatus(
					timeresource.StatusSpec{
						Synced:       true,
						Epoch:        0,
						SyncDisabled: false,
					},
				)
			},
		),
	)
}

func (suite *SyncSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func (suite *SyncSuite) newMockSyncer(logger *zap.Logger, servers []string) timectrl.NTPSyncer {
	suite.syncerMu.Lock()
	defer suite.syncerMu.Unlock()

	suite.syncer = newMockSyncer(logger, servers)

	return suite.syncer
}

func (suite *SyncSuite) getMockSyncer() *mockSyncer {
	suite.syncerMu.Lock()
	defer suite.syncerMu.Unlock()

	return suite.syncer
}

func TestSyncSuite(t *testing.T) {
	suite.Run(t, new(SyncSuite))
}

type mockSyncer struct {
	mu sync.Mutex

	timeServers []string
	syncedCh    chan struct{}
	epochCh     chan struct{}
}

func (mock *mockSyncer) Run(ctx context.Context) {
	<-ctx.Done()
}

func (mock *mockSyncer) Synced() <-chan struct{} {
	return mock.syncedCh
}

func (mock *mockSyncer) EpochChange() <-chan struct{} {
	return mock.epochCh
}

func (mock *mockSyncer) getTimeServers() (servers []string) {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	return slices.Clone(mock.timeServers)
}

func (mock *mockSyncer) SetTimeServers(servers []string) {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	mock.timeServers = slices.Clone(servers)
}

func newMockSyncer(_ *zap.Logger, servers []string) *mockSyncer {
	return &mockSyncer{
		timeServers: slices.Clone(servers),
		syncedCh:    make(chan struct{}, 1),
		epochCh:     make(chan struct{}, 1),
	}
}
