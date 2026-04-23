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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

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
)

type SyncSuite struct {
	ctest.DefaultSuite

	syncerMu sync.Mutex
	syncer   *mockSyncer
}

func (suite *SyncSuite) assertTimeStatus(spec timeresource.StatusSpec) {
	ctest.AssertResource(suite, timeresource.StatusID, func(r *timeresource.Status, asrt *assert.Assertions) {
		asrt.Equal(spec, *r.TypedSpec())
	})
}

func (suite *SyncSuite) registerSyncController(mode v1alpha1runtime.Mode) {
	suite.Require().NoError(suite.Runtime().RegisterController(&timectrl.SyncController{
		V1Alpha1Mode: mode,
		NewNTPSyncer: suite.newMockSyncer,
	}))
}

func (suite *SyncSuite) createDefaultTimeServers() *network.TimeServerStatus {
	timeServers := network.NewTimeServerStatus(network.NamespaceName, network.TimeServerID)
	timeServers.TypedSpec().NTPServers = []string{constants.DefaultNTPServer}

	suite.Create(timeServers)

	return timeServers
}

func (suite *SyncSuite) TestReconcileContainerMode() {
	suite.registerSyncController(v1alpha1runtime.ModeContainer)
	suite.createDefaultTimeServers()

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       true,
		Epoch:        0,
		SyncDisabled: true,
	})
}

func (suite *SyncSuite) TestReconcileSyncDisabled() {
	suite.registerSyncController(v1alpha1runtime.ModeMetal)
	suite.createDefaultTimeServers()

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       false,
		Epoch:        0,
		SyncDisabled: false,
	})

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineTime: &v1alpha1.TimeConfig{
						TimeDisabled: new(true),
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{},
			},
		),
	)

	suite.Create(cfg)

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       true,
		Epoch:        0,
		SyncDisabled: true,
	})
}

func (suite *SyncSuite) TestReconcileSyncDefaultConfig() {
	suite.registerSyncController(v1alpha1runtime.ModeMetal)
	suite.createDefaultTimeServers()

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
				ClusterConfig: &v1alpha1.ClusterConfig{},
			},
		),
	)

	suite.Create(cfg)

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       false,
		Epoch:        0,
		SyncDisabled: false,
	})
}

func (suite *SyncSuite) TestReconcileSyncChangeConfig() {
	suite.registerSyncController(v1alpha1runtime.ModeMetal)
	timeServers := suite.createDefaultTimeServers()

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       false,
		Epoch:        0,
		SyncDisabled: false,
	})

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
				ClusterConfig: &v1alpha1.ClusterConfig{},
			},
		),
	)

	suite.Create(cfg)

	var mockSyncer *mockSyncer

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		mockSyncer = suite.getMockSyncer()
		assert.NotNil(collect, mockSyncer, "syncer not created yet")
	}, 10*time.Second, 100*time.Millisecond)

	suite.Assert().Equal([]string{constants.DefaultNTPServer}, mockSyncer.getTimeServers())

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       false,
		Epoch:        0,
		SyncDisabled: false,
	})

	close(mockSyncer.syncedCh)

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       true,
		Epoch:        0,
		SyncDisabled: false,
	})

	ctest.UpdateWithConflicts(suite, timeServers, func(r *network.TimeServerStatus) error {
		r.TypedSpec().NTPServers = []string{"127.0.0.1"}

		return nil
	})

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		assert.Equal(collect, []string{"127.0.0.1"}, mockSyncer.getTimeServers(), "time servers not updated yet")
	}, 10*time.Second, 100*time.Millisecond)

	mockSyncer.epochCh <- struct{}{}

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       true,
		Epoch:        1,
		SyncDisabled: false,
	})

	ctest.UpdateWithConflicts(suite, cfg, func(r *config.MachineConfig) error {
		r.Container().RawV1Alpha1().MachineConfig.MachineTime = &v1alpha1.TimeConfig{ //nolint:staticcheck
			TimeDisabled: new(true),
		}

		return nil
	})

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       true,
		Epoch:        1,
		SyncDisabled: true,
	})
}

func (suite *SyncSuite) TestReconcileSyncBootTimeout() {
	suite.registerSyncController(v1alpha1runtime.ModeMetal)
	suite.createDefaultTimeServers()

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       false,
		Epoch:        0,
		SyncDisabled: false,
	})

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

	suite.Create(cfg)

	suite.assertTimeStatus(timeresource.StatusSpec{
		Synced:       true,
		Epoch:        0,
		SyncDisabled: false,
	})
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
	t.Parallel()

	syncSuite := &SyncSuite{}
	syncSuite.DefaultSuite = ctest.DefaultSuite{
		Timeout: 30 * time.Second,
		AfterSetup: func(s *ctest.DefaultSuite) {
			// reset mock syncer tracker between tests
			syncSuite.syncerMu.Lock()
			syncSuite.syncer = nil
			syncSuite.syncerMu.Unlock()

			// create fake device ready status
			deviceStatus := runtimeres.NewDevicesStatus(runtimeres.NamespaceName, runtimeres.DevicesID)
			deviceStatus.TypedSpec().Ready = true
			s.Require().NoError(s.State().Create(s.Ctx(), deviceStatus))
		},
	}

	suite.Run(t, syncSuite)
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
