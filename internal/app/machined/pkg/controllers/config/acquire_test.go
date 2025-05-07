// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	stderrors "errors"
	"fmt"
	"math/rand/v2"
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
	"github.com/klauspost/compress/zstd"
	"github.com/siderolabs/go-procfs/procfs"
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
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	configresource "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

type AcquireSuite struct {
	ctest.DefaultSuite

	platformConfig *platformConfigMock
	platformEvent  *platformEventMock
	configSetter   *configSetterMock
	eventPublisher *eventPublisherMock
	cmdline        *cmdlineGetterMock

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
	cfgCh          chan config.Provider
	persistedCfgCh chan config.Provider
}

func (c *configSetterMock) SetConfig(cfg config.Provider) error {
	c.cfgCh <- cfg

	return nil
}

func (c *configSetterMock) SetPersistedConfig(cfg config.Provider) error {
	c.persistedCfgCh <- cfg

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

type cmdlineGetterMock struct {
	cmdline *procfs.Cmdline
}

func (c *cmdlineGetterMock) Getter() func() *procfs.Cmdline {
	return func() *procfs.Cmdline {
		return c.cmdline
	}
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
		s.platformConfig = &platformConfigMock{
			err: errors.ErrNoConfigSource,
		}
		s.platformEvent = &platformEventMock{}
		s.configSetter = &configSetterMock{
			cfgCh:          make(chan config.Provider, 1),
			persistedCfgCh: make(chan config.Provider, 1),
		}
		s.eventPublisher = &eventPublisherMock{}
		s.cmdline = &cmdlineGetterMock{
			procfs.NewCmdline(""),
		}

		s.clusterName = fmt.Sprintf("cluster-%d", rand.Int32())
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
			Mode:                  validationModeMock{},
			CmdlineGetter:         s.cmdline.Getter(),
			EventPublisher:        s.eventPublisher,
			ValidationMode:        validationModeMock{},
			ResourceState:         s.State(),
		}))
	}

	suite.Run(t, s)
}

func (suite *AcquireSuite) triggerAcquire() {
	suite.Require().NoError(suite.State().Create(suite.Ctx(), v1alpha1.NewAcquireConfigSpec()))
}

func (suite *AcquireSuite) waitForConfig(shouldPersist bool) config.Provider {
	var (
		appliedConfig   config.Provider
		persistedConfig config.Provider
	)

	for {
		select {
		case cfg := <-suite.configSetter.cfgCh:
			suite.Require().Nil(appliedConfig)

			appliedConfig = cfg
		case cfg := <-suite.configSetter.persistedCfgCh:
			suite.Require().Nil(persistedConfig)
			suite.Require().True(shouldPersist)

			persistedConfig = cfg
		case <-suite.Ctx().Done():
			suite.Require().Fail("timed out waiting for config: applied %v persisted %v", appliedConfig, persistedConfig)
		}

		if appliedConfig != nil && (persistedConfig != nil || !shouldPersist) {
			break
		}
	}

	if persistedConfig != nil {
		suite.Assert().Same(persistedConfig, appliedConfig)
	}

	status := v1alpha1.NewAcquireConfigStatus()
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{status.Metadata().ID()}, func(*v1alpha1.AcquireConfigStatus, *assert.Assertions) {})

	return appliedConfig
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

func (suite *AcquireSuite) noStateVolume() {
	volumeStatus := block.NewVolumeStatus(block.NamespaceName, constants.StatePartitionLabel)
	volumeStatus.TypedSpec().Phase = block.VolumePhaseMissing
	suite.Create(volumeStatus)
}

func (suite *AcquireSuite) presentStateVolume() {
	volumeStatus := block.NewVolumeStatus(block.NamespaceName, constants.StatePartitionLabel)
	volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
	suite.Create(volumeStatus)
}

func (suite *AcquireSuite) injectViaDisk(cfg []byte, wait bool) {
	statePath := suite.T().TempDir()
	mountID := (&configctrl.AcquireController{}).Name() + "-" + constants.StatePartitionLabel

	ctest.AssertResource(suite, mountID, func(mountRequest *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(constants.StatePartitionLabel, mountRequest.TypedSpec().VolumeID)
	})

	suite.Require().NoError(os.WriteFile(filepath.Join(statePath, constants.ConfigFilename), cfg, 0o644))

	volumeMountStatus := block.NewVolumeMountStatus(block.NamespaceName, mountID)
	volumeMountStatus.TypedSpec().Target = statePath
	suite.Create(volumeMountStatus)

	if wait {
		ctest.AssertNoResource[*block.VolumeMountRequest](suite, mountID)
		suite.Destroy(volumeMountStatus)
	}
}

func (suite *AcquireSuite) TestFromDisk() {
	suite.presentStateVolume()

	suite.triggerAcquire()

	suite.injectViaDisk(suite.completeMachineConfig, true)

	cfg := suite.waitForConfig(false)
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
	suite.presentStateVolume()

	suite.triggerAcquire()

	suite.injectViaDisk(slices.Concat([]byte("aaa"), suite.completeMachineConfig), false)

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
	suite.presentStateVolume()

	suite.triggerAcquire()

	suite.injectViaDisk(suite.partialMachineConfig, true)

	var cfg config.Provider

	select {
	case cfg = <-suite.configSetter.cfgCh:
	case <-suite.configSetter.persistedCfgCh:
		suite.Require().Fail("should not persist")
	case <-suite.Ctx().Done():
		suite.Require().Fail("timed out waiting for config")
	}

	suite.Require().Equal(cfg.SideroLink().APIUrl().Host, "siderolink.api")

	suite.injectViaMaintenance(suite.completeMachineConfig)

	cfg = suite.waitForConfig(true)
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
	suite.noStateVolume()
	suite.platformConfig.configuration = suite.completeMachineConfig
	suite.platformConfig.err = nil

	suite.triggerAcquire()

	cfg := suite.waitForConfig(true)
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
	suite.noStateVolume()
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

	suite.noStateVolume()
	suite.triggerAcquire()

	cfg := suite.waitForConfig(true)
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

	suite.noStateVolume()
	suite.triggerAcquire()

	var cfg config.Provider

	select {
	case cfg = <-suite.configSetter.cfgCh:
	case <-suite.Ctx().Done():
		suite.Require().Fail("timed out waiting for config")
	}

	select {
	case <-suite.configSetter.persistedCfgCh:
	case <-suite.Ctx().Done():
		suite.Require().Fail("timed out waiting for persisted config")
	}

	suite.Require().Equal(cfg.SideroLink().APIUrl().Host, "siderolink.api")

	suite.injectViaMaintenance(suite.completeMachineConfig)

	cfg = suite.waitForConfig(true)
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

func (suite *AcquireSuite) TestFromCmdlineToMaintenance() {
	var cfgCompressed bytes.Buffer

	zw, err := zstd.NewWriter(&cfgCompressed)
	suite.Require().NoError(err)

	_, err = zw.Write(suite.partialMachineConfig)
	suite.Require().NoError(err)

	suite.Require().NoError(zw.Close())

	cfgEncoded := base64.StdEncoding.EncodeToString(cfgCompressed.Bytes())

	suite.cmdline.cmdline = procfs.NewCmdline(fmt.Sprintf("%s=%s", constants.KernelParamConfigInline, cfgEncoded))

	suite.noStateVolume()
	suite.triggerAcquire()

	var cfg config.Provider

	select {
	case cfg = <-suite.configSetter.cfgCh:
	case <-suite.Ctx().Done():
		suite.Require().Fail("timed out waiting for config")
	}

	select {
	case <-suite.configSetter.persistedCfgCh:
	case <-suite.Ctx().Done():
		suite.Require().Fail("timed out waiting for persisted config")
	}

	suite.Require().Equal(cfg.SideroLink().APIUrl().Host, "siderolink.api")

	suite.injectViaMaintenance(suite.completeMachineConfig)

	cfg = suite.waitForConfig(true)
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
	suite.noStateVolume()
	suite.triggerAcquire()

	suite.injectViaMaintenance(suite.completeMachineConfig)

	cfg := suite.waitForConfig(true)
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
