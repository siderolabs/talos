// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config_test

import (
	"bytes"
	"compress/gzip"
	"context"
	stderrors "errors"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	configctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/config"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	configresource "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

type AcquireSuite struct {
	ctest.DefaultSuite

	configPath     string
	platformConfig *platformConfigMock
	platformEvent  *platformEventMock
	configSetter   *configSetterMock
	eventPublisher *eventPublisherMock

	clusterName           string
	completeMachineConfig []byte
	partialMachineConfig  []byte
}

type platformConfigMock struct {
	configuration []byte
	err           error
}

func (p *platformConfigMock) Configuration(context.Context) ([]byte, error) {
	return p.configuration, p.err
}

func (p *platformConfigMock) Name() string {
	return "mock"
}

type platformEventMock struct {
	mu     sync.Mutex
	events []platform.Event
}

func (p *platformEventMock) FireEvent(_ context.Context, ev platform.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.events = append(p.events, ev)
}

func (p *platformEventMock) getEvents() []platform.Event {
	p.mu.Lock()
	defer p.mu.Unlock()

	return slices.Clone(p.events)
}

type configSetterMock struct {
	cfgCh chan config.Provider
}

func (c *configSetterMock) SetConfig(cfg config.Provider) error {
	c.cfgCh <- cfg

	return nil
}

type eventPublisherMock struct {
	mu     sync.Mutex
	events []proto.Message
}

func (e *eventPublisherMock) Publish(_ context.Context, ev proto.Message) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.events = append(e.events, ev)
}

func (e *eventPublisherMock) getEvents() []proto.Message {
	e.mu.Lock()
	defer e.mu.Unlock()

	return slices.Clone(e.events)
}

type validationModeMock struct{}

func (v validationModeMock) String() string {
	return "mock"
}

func (v validationModeMock) RequiresInstall() bool {
	return false
}

func (v validationModeMock) InContainer() bool {
	return false
}

func TestAcquireSuite(t *testing.T) {
	t.Parallel()

	s := &AcquireSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
		},
	}

	s.DefaultSuite.AfterSetup = func(*ctest.DefaultSuite) {
		tmpDir := s.T().TempDir()
		s.configPath = filepath.Join(tmpDir, "config.yaml")
		s.platformConfig = &platformConfigMock{
			err: errors.ErrNoConfigSource,
		}
		s.platformEvent = &platformEventMock{}
		s.configSetter = &configSetterMock{
			cfgCh: make(chan config.Provider, 1),
		}
		s.eventPublisher = &eventPublisherMock{}

		s.clusterName = fmt.Sprintf("cluster-%d", rand.Int31())
		input, err := generate.NewInput(s.clusterName, "https://localhost:6443", "")
		s.Require().NoError(err)

		cfg, err := input.Config(machine.TypeControlPlane)
		s.Require().NoError(err)

		s.completeMachineConfig, err = cfg.Bytes()
		s.Require().NoError(err)

		sideroLinkCfg := siderolink.NewConfigV1Alpha1()
		sideroLinkCfg.APIUrlConfig.URL = must(url.Parse("https://siderolink.api/?jointoken=secret&user=alice"))

		pCfg, err := container.New(sideroLinkCfg)
		s.Require().NoError(err)

		s.partialMachineConfig, err = pCfg.Bytes()
		s.Require().NoError(err)

		s.Require().NoError(s.Runtime().RegisterController(&configctrl.AcquireController{
			PlatformConfiguration: s.platformConfig,
			PlatformEvent:         s.platformEvent,
			ConfigSetter:          s.configSetter,
			EventPublisher:        s.eventPublisher,
			ValidationMode:        validationModeMock{},
			ConfigPath:            s.configPath,
		}))
	}

	suite.Run(t, s)
}

func (suite *AcquireSuite) triggerAcquire() {
	suite.Require().NoError(suite.State().Create(suite.Ctx(), v1alpha1.NewAcquireConfigSpec()))
}

func (suite *AcquireSuite) waitForConfig() config.Provider {
	var cfg config.Provider

	select {
	case cfg = <-suite.configSetter.cfgCh:
	case <-suite.Ctx().Done():
		suite.Require().Fail("timed out waiting for config")
	}

	status := v1alpha1.NewAcquireConfigStatus()
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{status.Metadata().ID()}, func(*v1alpha1.AcquireConfigStatus, *assert.Assertions) {})

	return cfg
}

func (suite *AcquireSuite) injectViaMaintenance(cfg []byte) {
	_, err := suite.State().WatchFor(suite.Ctx(), runtime.NewMaintenanceServiceRequest().Metadata(), state.WithEventTypes(state.Created))
	suite.Require().NoError(err)

	mCfg, err := configloader.NewFromBytes(cfg)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), configresource.NewMachineConfigWithID(mCfg, configresource.MaintenanceID)))

	_, err = suite.State().WatchFor(suite.Ctx(), runtime.NewMaintenanceServiceRequest().Metadata(), state.WithEventTypes(state.Destroyed))
	suite.Require().NoError(err)
}

func (suite *AcquireSuite) TestFromDisk() {
	suite.Require().NoError(os.WriteFile(suite.configPath, suite.completeMachineConfig, 0o644))

	suite.triggerAcquire()

	cfg := suite.waitForConfig()
	suite.Require().Equal(cfg.Cluster().Name(), suite.clusterName)

	suite.Assert().Empty(suite.eventPublisher.getEvents())
	suite.Assert().Equal(
		[]platform.Event{
			{
				Type:    platform.EventTypeConfigLoaded,
				Message: "Talos machine config loaded successfully.",
			},
		},
		suite.platformEvent.getEvents(),
	)
}

func (suite *AcquireSuite) TestFromDiskFailure() {
	suite.Require().NoError(os.WriteFile(suite.configPath, append([]byte("aaa"), suite.completeMachineConfig...), 0o644))

	suite.triggerAcquire()

	suite.AssertWithin(time.Second, 10*time.Millisecond, func() error {
		if len(suite.platformEvent.getEvents()) == 0 || len(suite.eventPublisher.getEvents()) == 0 {
			return retry.ExpectedErrorf("no events received")
		}

		return nil
	})

	ev := suite.platformEvent.getEvents()[0]
	suite.Assert().Equal(platform.EventTypeFailure, ev.Type)
	suite.Assert().Equal("Error loading and validating Talos machine config.", ev.Message)
	suite.Assert().Equal("failed to load config from STATE: unknown keys found during decoding:\naaaversion: v1alpha1 # Indicates the schema used to decode the contents.\n", ev.Error.Error())

	suite.Assert().Equal(&machineapi.ConfigLoadErrorEvent{
		Error: "failed to load config from STATE: unknown keys found during decoding:\naaaversion: v1alpha1 # Indicates the schema used to decode the contents.\n",
	}, suite.eventPublisher.getEvents()[0])
}

func (suite *AcquireSuite) TestFromDiskToMaintenance() {
	suite.Require().NoError(os.WriteFile(suite.configPath, suite.partialMachineConfig, 0o644))

	suite.triggerAcquire()

	var cfg config.Provider

	select {
	case cfg = <-suite.configSetter.cfgCh:
	case <-suite.Ctx().Done():
		suite.Require().Fail("timed out waiting for config")
	}

	suite.Require().Equal(cfg.SideroLink().APIUrl().Host, "siderolink.api")

	suite.injectViaMaintenance(suite.completeMachineConfig)

	cfg = suite.waitForConfig()
	suite.Require().Equal(cfg.Cluster().Name(), suite.clusterName)

	suite.Assert().Equal(
		[]proto.Message{
			&machineapi.TaskEvent{
				Action: machineapi.TaskEvent_START,
				Task:   "runningMaintenance",
			},
			&machineapi.TaskEvent{
				Action: machineapi.TaskEvent_STOP,
				Task:   "runningMaintenance",
			},
		},
		suite.eventPublisher.getEvents(),
	)
	suite.Assert().Equal(
		[]platform.Event{
			{
				Type:    platform.EventTypeActivate,
				Message: "Talos booted into maintenance mode. Ready for user interaction.",
			},
			{
				Type:    platform.EventTypeConfigLoaded,
				Message: "Talos machine config loaded successfully.",
			},
		},
		suite.platformEvent.getEvents(),
	)
}

func (suite *AcquireSuite) TestFromPlatform() {
	suite.platformConfig.configuration = suite.completeMachineConfig
	suite.platformConfig.err = nil

	suite.triggerAcquire()

	cfg := suite.waitForConfig()
	suite.Require().Equal(cfg.Cluster().Name(), suite.clusterName)

	suite.Assert().Empty(suite.eventPublisher.getEvents())
	suite.Assert().Equal(
		[]platform.Event{
			{
				Type:    platform.EventTypeConfigLoaded,
				Message: "Talos machine config loaded successfully.",
			},
		},
		suite.platformEvent.getEvents(),
	)
}

func (suite *AcquireSuite) TestFromPlatformFailure() {
	suite.platformConfig.err = stderrors.New("mock error")

	suite.triggerAcquire()

	suite.AssertWithin(time.Second, 10*time.Millisecond, func() error {
		if len(suite.platformEvent.getEvents()) == 0 || len(suite.eventPublisher.getEvents()) == 0 {
			return retry.ExpectedErrorf("no events received")
		}

		return nil
	})

	ev := suite.platformEvent.getEvents()[0]
	suite.Assert().Equal(platform.EventTypeFailure, ev.Type)
	suite.Assert().Equal("Error loading and validating Talos machine config.", ev.Message)
	suite.Assert().Equal("error acquiring via platform mock: mock error", ev.Error.Error())

	suite.Assert().Equal(&machineapi.ConfigLoadErrorEvent{
		Error: "error acquiring via platform mock: mock error",
	}, suite.eventPublisher.getEvents()[0])
}

func (suite *AcquireSuite) TestFromPlatformGzip() {
	var buf bytes.Buffer

	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(suite.completeMachineConfig)
	suite.Require().NoError(err)
	suite.Require().NoError(gz.Close())

	suite.platformConfig.configuration = buf.Bytes()
	suite.platformConfig.err = nil

	suite.triggerAcquire()

	cfg := suite.waitForConfig()
	suite.Require().Equal(cfg.Cluster().Name(), suite.clusterName)

	suite.Assert().Empty(suite.eventPublisher.getEvents())
	suite.Assert().Equal(
		[]platform.Event{
			{
				Type:    platform.EventTypeConfigLoaded,
				Message: "Talos machine config loaded successfully.",
			},
		},
		suite.platformEvent.getEvents(),
	)
}

func (suite *AcquireSuite) TestFromPlatformToMaintenance() {
	suite.platformConfig.configuration = suite.partialMachineConfig
	suite.platformConfig.err = nil

	suite.triggerAcquire()

	var cfg config.Provider

	select {
	case cfg = <-suite.configSetter.cfgCh:
	case <-suite.Ctx().Done():
		suite.Require().Fail("timed out waiting for config")
	}

	suite.Require().Equal(cfg.SideroLink().APIUrl().Host, "siderolink.api")

	suite.injectViaMaintenance(suite.completeMachineConfig)

	cfg = suite.waitForConfig()
	suite.Require().Equal(cfg.Cluster().Name(), suite.clusterName)

	suite.Assert().Equal(
		[]proto.Message{
			&machineapi.TaskEvent{
				Action: machineapi.TaskEvent_START,
				Task:   "runningMaintenance",
			},
			&machineapi.TaskEvent{
				Action: machineapi.TaskEvent_STOP,
				Task:   "runningMaintenance",
			},
		},
		suite.eventPublisher.getEvents(),
	)
	suite.Assert().Equal(
		[]platform.Event{
			{
				Type:    platform.EventTypeActivate,
				Message: "Talos booted into maintenance mode. Ready for user interaction.",
			},
			{
				Type:    platform.EventTypeConfigLoaded,
				Message: "Talos machine config loaded successfully.",
			},
		},
		suite.platformEvent.getEvents(),
	)
}

func (suite *AcquireSuite) TestFromMaintenance() {
	suite.triggerAcquire()

	suite.injectViaMaintenance(suite.completeMachineConfig)

	cfg := suite.waitForConfig()
	suite.Require().Equal(cfg.Cluster().Name(), suite.clusterName)

	suite.Assert().Equal(
		[]proto.Message{
			&machineapi.TaskEvent{
				Action: machineapi.TaskEvent_START,
				Task:   "runningMaintenance",
			},
			&machineapi.TaskEvent{
				Action: machineapi.TaskEvent_STOP,
				Task:   "runningMaintenance",
			},
		},
		suite.eventPublisher.getEvents(),
	)
	suite.Assert().Equal(
		[]platform.Event{
			{
				Type:    platform.EventTypeActivate,
				Message: "Talos booted into maintenance mode. Ready for user interaction.",
			},
			{
				Type:    platform.EventTypeConfigLoaded,
				Message: "Talos machine config loaded successfully.",
			},
		},
		suite.platformEvent.getEvents(),
	)
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}
