// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package time_test

import (
	"context"
	"fmt"
	"log"
	"reflect"
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

	timectrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/time"
	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/resources/config"
	timeresource "github.com/talos-systems/talos/pkg/resources/time"
	v1alpha1resource "github.com/talos-systems/talos/pkg/resources/v1alpha1"
)

type SyncSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc

	syncerMu sync.Mutex
	syncer   *mockSyncer
}

func (suite *SyncSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	logger := log.New(log.Writer(), "controller-runtime: ", log.Flags())

	suite.runtime, err = runtime.NewRuntime(suite.state, logger)
	suite.Require().NoError(err)
}

func (suite *SyncSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *SyncSuite) assertTimeStatus(spec timeresource.StatusSpec) error {
	r, err := suite.state.Get(suite.ctx, resource.NewMetadata(v1alpha1resource.NamespaceName, timeresource.StatusType, timeresource.StatusID, resource.VersionUndefined))
	if err != nil {
		if state.IsNotFoundError(err) {
			return retry.ExpectedError(err)
		}

		return retry.UnexpectedError(err)
	}

	status := r.(*timeresource.Status) //nolint:errcheck,forcetypeassert

	if status.Status() != spec {
		return retry.ExpectedError(fmt.Errorf("time status doesn't match: %v != %v", status.Status(), spec))
	}

	return nil
}

func (suite *SyncSuite) TestReconcileContainerMode() {
	suite.Require().NoError(suite.runtime.RegisterController(&timectrl.SyncController{
		V1Alpha1Mode: v1alpha1runtime.ModeContainer,
		NewNTPSyncer: suite.newMockSyncer,
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertTimeStatus(
				timeresource.StatusSpec{
					Synced:       true,
					Epoch:        0,
					SyncDisabled: true,
				},
			)
		},
	))
}

func (suite *SyncSuite) TestReconcileSyncDisabled() {
	suite.Require().NoError(suite.runtime.RegisterController(&timectrl.SyncController{
		V1Alpha1Mode: v1alpha1runtime.ModeMetal,
		NewNTPSyncer: suite.newMockSyncer,
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertTimeStatus(
				timeresource.StatusSpec{
					Synced:       false,
					Epoch:        0,
					SyncDisabled: false,
				},
			)
		},
	))

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineTime: &v1alpha1.TimeConfig{
				TimeDisabled: true,
			},
		},
		ClusterConfig: &v1alpha1.ClusterConfig{},
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertTimeStatus(
				timeresource.StatusSpec{
					Synced:       true,
					Epoch:        0,
					SyncDisabled: true,
				},
			)
		},
	))
}

func (suite *SyncSuite) TestReconcileSyncDefaultConfig() {
	suite.Require().NoError(suite.runtime.RegisterController(&timectrl.SyncController{
		V1Alpha1Mode: v1alpha1runtime.ModeMetal,
		NewNTPSyncer: suite.newMockSyncer,
	}))

	suite.startRuntime()

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
		ClusterConfig: &v1alpha1.ClusterConfig{},
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertTimeStatus(
				timeresource.StatusSpec{
					Synced:       false,
					Epoch:        0,
					SyncDisabled: false,
				},
			)
		},
	))
}

func (suite *SyncSuite) TestReconcileSyncChangeConfig() {
	suite.Require().NoError(suite.runtime.RegisterController(&timectrl.SyncController{
		V1Alpha1Mode: v1alpha1runtime.ModeMetal,
		NewNTPSyncer: suite.newMockSyncer,
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertTimeStatus(
				timeresource.StatusSpec{
					Synced:       false,
					Epoch:        0,
					SyncDisabled: false,
				},
			)
		},
	))

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
		ClusterConfig: &v1alpha1.ClusterConfig{},
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	var mockSyncer *mockSyncer

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			mockSyncer = suite.getMockSyncer()

			if mockSyncer == nil {
				return retry.ExpectedError(fmt.Errorf("syncer not created yet"))
			}

			return nil
		},
	))

	suite.Assert().Equal([]string{constants.DefaultNTPServer}, mockSyncer.getTimeServers())

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertTimeStatus(
				timeresource.StatusSpec{
					Synced:       false,
					Epoch:        0,
					SyncDisabled: false,
				},
			)
		},
	))

	close(mockSyncer.syncedCh)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertTimeStatus(
				timeresource.StatusSpec{
					Synced:       true,
					Epoch:        0,
					SyncDisabled: false,
				},
			)
		},
	))

	_, err := suite.state.UpdateWithConflicts(suite.ctx, cfg.Metadata(), func(r resource.Resource) error {
		r.(*config.MachineConfig).Config().(*v1alpha1.Config).MachineConfig.MachineTime = &v1alpha1.TimeConfig{
			TimeServers: []string{"127.0.0.1"},
		}

		return nil
	})
	suite.Require().NoError(err)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			if !reflect.DeepEqual(mockSyncer.getTimeServers(), []string{"127.0.0.1"}) {
				return retry.ExpectedError(fmt.Errorf("time servers not updated yet"))
			}

			return nil
		},
	))

	mockSyncer.epochCh <- struct{}{}

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertTimeStatus(
				timeresource.StatusSpec{
					Synced:       true,
					Epoch:        1,
					SyncDisabled: false,
				},
			)
		},
	))

	_, err = suite.state.UpdateWithConflicts(suite.ctx, cfg.Metadata(), func(r resource.Resource) error {
		r.(*config.MachineConfig).Config().(*v1alpha1.Config).MachineConfig.MachineTime = &v1alpha1.TimeConfig{
			TimeDisabled: true,
		}

		return nil
	})
	suite.Require().NoError(err)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertTimeStatus(
				timeresource.StatusSpec{
					Synced:       true,
					Epoch:        1,
					SyncDisabled: true,
				},
			)
		},
	))
}

func (suite *SyncSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	err := suite.state.Create(context.Background(), config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
	}))
	if state.IsConflictError(err) {
		err = suite.state.Destroy(context.Background(), config.NewMachineConfig(nil).Metadata())
	}

	suite.Assert().NoError(err)
}

func (suite *SyncSuite) newMockSyncer(logger *log.Logger, servers []string) timectrl.NTPSyncer {
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

	return append([]string(nil), mock.timeServers...)
}

func (mock *mockSyncer) SetTimeServers(servers []string) {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	mock.timeServers = append([]string(nil), servers...)
}

func newMockSyncer(_ *log.Logger, servers []string) *mockSyncer {
	return &mockSyncer{
		timeServers: append([]string(nil), servers...),
		syncedCh:    make(chan struct{}, 1),
		epochCh:     make(chan struct{}, 1),
	}
}
