// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_provision

package provision

import (
	"context"
	"fmt"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
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

	suite.Run("trigger poweroff", func() {
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

func init() {
	allSuites = append(
		allSuites,
		&ExternalTriggerSuite{track: 3},
	)
}
