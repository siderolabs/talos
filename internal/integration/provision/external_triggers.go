// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_provision

package provision

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// ExternalTriggerSuite ...
type ExternalTriggerSuite struct {
	BaseSuite

	track int
}

// SuiteName ...
func (suite *ExternalTriggerSuite) SuiteName() string {
	return fmt.Sprintf("provision.UpgradeSuite.ExternalTrigger-TR%d", suite.track)
}

// TestTriggers verifies that external triggers like Ctrl+Alt+Del and ACPI power off are handled correctly.
//
//nolint:dupl,gocyclo
func (suite *ExternalTriggerSuite) TestTriggers() {
	const (
		numControlplanes = 2
	)

	suite.setupCluster(clusterOptions{
		ClusterName: "external-trigger",

		ControlplaneNodes: numControlplanes,

		SourceKernelPath:    helpers.ArtifactPath(constants.KernelAssetWithArch),
		SourceInitramfsPath: helpers.ArtifactPath(constants.InitramfsAssetWithArch),
		SourceInstallerImage: fmt.Sprintf(
			"%s/%s:%s",
			DefaultSettings.TargetInstallImageRegistry,
			images.DefaultInstallerImageName, //nolint:staticcheck // legacy is only used in tests
			DefaultSettings.CurrentVersion,
		),
		SourceVersion:    DefaultSettings.CurrentVersion,
		SourceK8sVersion: constants.DefaultKubernetesVersion,

		WithSkipInjectingConfig: true,
	})

	maintenanceClients := make([]*client.Client, len(suite.Cluster.Info().Nodes))

	for i, machine := range suite.Cluster.Info().Nodes {
		var err error

		maintenanceClients[i], err = client.New(
			suite.ctx,
			client.WithMaintenanceMode(machine.IPs[0].String(), nil),
		)
		suite.Require().NoError(err)
	}

	defer func() {
		for _, c := range maintenanceClients {
			suite.Require().NoError(c.Close())
		}
	}()

	suite.Run("wait for maintenance API", func() {
		// we should be able to query version API for every machine
		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			for _, maintenanceClient := range maintenanceClients {
				version, err := maintenanceClient.Version(suite.ctx)
				if !asrt.NoError(err) {
					return
				}

				suite.Assert().Equal(DefaultSettings.CurrentVersion, version.GetMessages()[0].GetVersion().GetTag())
			}
		}, time.Minute, time.Second, "version API should be available")
	})

	suite.Run("trigger Ctrl+Alt+Delete", func() {
		suite.T().Logf("using node %s", suite.Cluster.Info().Nodes[0].Name)

		// using machine 0 for this test
		c := maintenanceClients[0]

		events := make(chan client.EventResult)

		ctx, cancel := context.WithTimeout(suite.ctx, 10*time.Second)
		defer cancel()

		suite.Require().NoError(c.EventsWatchV2(ctx, events))

		suite.sendMonitorCommand(ctx, suite.Cluster.Info().Nodes[0].Name, "sendkey ctrl-alt-delete")

		for {
			select {
			case <-ctx.Done():
				suite.Fail("timeout waiting for Ctrl+Alt+Delete event")
			case event := <-events:
				suite.Require().NoError(event.Error)

				if taskEvent, ok := event.Event.Payload.(*machine.TaskEvent); ok {
					if taskEvent.Action == machine.TaskEvent_START && taskEvent.Task == "reboot" {
						suite.T().Logf("received reboot event")

						return
					}
				}
			}
		}
	})

	suite.Run("ignore Ctrl+Alt+Delete", func() {
		// using machine 1 for this test, before it is powered off
		suite.ignoreCtrlAltDelete(maintenanceClients[1], suite.Cluster.Info().Nodes[1].Name)
	})

	suite.Run("trigger poweroff", func() {
		suite.T().Logf("using node %s", suite.Cluster.Info().Nodes[1].Name)

		// using machine 1 for this test
		c := maintenanceClients[1]

		events := make(chan client.EventResult)

		ctx, cancel := context.WithTimeout(suite.ctx, 10*time.Second)
		defer cancel()

		suite.Require().NoError(c.EventsWatchV2(ctx, events))

		suite.sendMonitorCommand(ctx, suite.Cluster.Info().Nodes[1].Name, "system_powerdown")

		for {
			select {
			case <-ctx.Done():
				suite.Fail("timeout waiting for shutdown event")
			case event := <-events:
				suite.Require().NoError(event.Error)

				if sequenceEvent, ok := event.Event.Payload.(*machine.SequenceEvent); ok {
					if sequenceEvent.Action == machine.SequenceEvent_START && sequenceEvent.Sequence == "shutdown" {
						suite.T().Logf("received shutdown event")

						return
					}
				}
			}
		}
	})
}

// ignoreCtrlAltDelete verifies that Ctrl+Alt+Delete is logged and ignored with the SecurityProfileConfig option set.
func (suite *ExternalTriggerSuite) ignoreCtrlAltDelete(c *client.Client, nodeName string) {
	suite.T().Logf("using node %s", nodeName)

	events := make(chan client.EventResult)

	ctx, cancel := context.WithTimeout(suite.ctx, time.Minute)
	defer cancel()

	suite.Require().NoError(c.EventsWatchV2(ctx, events))

	// a partial config is accepted in maintenance mode, and the node stays in maintenance mode
	suite.applyIgnoreCtrlAltDelete(ctx, c)

	suite.sendMonitorCommand(ctx, nodeName, "sendkey ctrl-alt-delete")

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		assert.Contains(collect, suite.readConsoleLog(nodeName), "Ctrl-Alt-Delete ignored as per SecurityProfileConfig")
	}, 10*time.Second, time.Second, "Ctrl+Alt+Delete should be logged as ignored")

	noRebootCtx, noRebootCancel := context.WithTimeout(ctx, 10*time.Second)
	defer noRebootCancel()

	for {
		select {
		case <-noRebootCtx.Done():
			return
		case event := <-events:
			suite.Require().NoError(event.Error)

			if taskEvent, ok := event.Event.Payload.(*machine.TaskEvent); ok && taskEvent.Task == "reboot" {
				suite.FailNow("unexpected reboot on ignored Ctrl+Alt+Delete")
			}
		}
	}
}

func (suite *ExternalTriggerSuite) applyIgnoreCtrlAltDelete(ctx context.Context, c *client.Client) {
	securityProfile := runtimecfg.NewSecurityProfileConfigV1Alpha1()
	securityProfile.IgnoreCtrlAltDeleteEnabled = new(true)

	cfg, err := container.New(securityProfile)
	suite.Require().NoError(err)

	cfgBytes, err := cfg.Bytes()
	suite.Require().NoError(err)

	_, err = c.ApplyConfiguration(ctx, &machine.ApplyConfigurationRequest{
		Data: cfgBytes,
		Mode: machine.ApplyConfigurationRequest_NO_REBOOT,
	})
	suite.Require().NoError(err)
}

func (suite *ExternalTriggerSuite) readConsoleLog(nodeName string) string {
	statePath, err := suite.Cluster.StatePath()
	suite.Require().NoError(err)

	contents, err := os.ReadFile(filepath.Join(statePath, nodeName+".log"))
	suite.Require().NoError(err)

	return string(contents)
}

func init() {
	allSuites = append(
		allSuites,
		&ExternalTriggerSuite{track: 3},
	)
}
