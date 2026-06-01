// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/blang/semver/v4"
	"github.com/siderolabs/gen/xerrors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos/lifecycle"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos/multiplex"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/action"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/nodedrain"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/flags"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/reporter"
)

var upgradeCmdFlags = struct {
	trackableActionCmdFlags
	imageCmdFlagsType

	upgradeImage string
	rebootMode   flags.PflagExtended[machine.RebootRequest_Mode]
	progress     flags.PflagExtended[reporter.OutputMode]

	drain        bool
	drainTimeout time.Duration

	legacy   bool
	force    bool // Deprecated: only used for legacy upgrade path, to be removed in Talos 1.18.
	insecure bool // Deprecated: only used for legacy upgrade path, to be removed in Talos 1.18.
	preserve bool // Deprecated: only used for legacy upgrade path, to be removed in Talos 1.18.
	stage    bool // Deprecated: only used for legacy upgrade path, to be removed in Talos 1.18.
}{
	rebootMode: flags.ProtoEnum(machine.RebootRequest_DEFAULT, machine.RebootRequest_Mode_value, machine.RebootRequest_Mode_name),
	progress:   reporter.NewOutputModeFlag(),
}

// upgradeCmd represents the processes command.
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade Talos on the target node",
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if upgradeCmdFlags.debug {
			upgradeCmdFlags.wait = true
		}

		if upgradeCmdFlags.drain {
			upgradeCmdFlags.wait = true
		}

		if upgradeCmdFlags.wait && upgradeCmdFlags.insecure {
			return errors.New("cannot use --wait and --insecure together")
		}

		return upgradeRun(cmd.Context())
	},
}

func upgradeRun(ctx context.Context) error {
	return WithClientAndNodes(ctx, upgradeViaLifecycleService)
}

var talosUpgradeAPIVersionRange = semver.MustParseRange(">1.13.0-alpha.2 <2.0.0")

// upgradeViaLifecycleService tries the new LifecycleService.Upgrade streaming API.
// If the server returns codes.Unimplemented, it falls back to the legacy MachineService.Upgrade.
//
//nolint:gocyclo
func upgradeViaLifecycleService(ctx context.Context, c *client.Client, nodes []string) (retErr error) {
	if upgradeCmdFlags.debug {
		upgradeCmdFlags.wait = true
	}

	if upgradeCmdFlags.legacy {
		cli.Warning("Forcing use of legacy upgrade method. This flag is deprecated and will be removed in Talos 1.18.")

		return upgradeLegacy(ctx)
	}

	opts := []client.RebootMode{
		client.WithRebootMode(upgradeCmdFlags.rebootMode.Value()),
	}

	containerdInstance, err := upgradeCmdFlags.containerdInstance()
	if err != nil {
		return err
	}

	rep := reporter.New(
		reporter.WithOutputMode(upgradeCmdFlags.progress.Value()),
	)

	err = WithClient(ctx, func(ctx context.Context, c *client.Client) error {
		return helpers.TalosVersionCheck(ctx, c, talosUpgradeAPIVersionRange)
	})
	if err != nil {
		if xerrors.TagIs[helpers.VersionOutsideRangeError](err) {
			rep.Report(reporter.Update{
				Status:  reporter.StatusError,
				Message: "New upgrade API is not available, falling back to legacy",
			})

			return upgradeLegacy(ctx)
		}

		return fmt.Errorf("error checking Talos version compatibility: %w", err)
	}

	_, err = imagePullInternal(ctx, c, containerdInstance, nodes, upgradeCmdFlags.upgradeImage, rep)
	if err != nil {
		return fmt.Errorf("error pulling upgrade image: %w", err)
	}

	_, err = upgradeInternal(ctx, c, containerdInstance, nodes, upgradeCmdFlags.upgradeImage, rep)
	if err != nil {
		return fmt.Errorf("error during upgrade: %w", err)
	}

	var nodeNames map[string]string

	if upgradeCmdFlags.drain {
		nodeNames, err = drainNodes(ctx, c, nodes, upgradeCmdFlags.drainTimeout, rep)
		if err != nil {
			return err
		}
	}

	defer func() {
		if !upgradeCmdFlags.drain {
			return
		}

		if len(nodeNames) > 0 {
			if uncordonErr := uncordonNodes(ctx, c, nodeNames, upgradeCmdFlags.timeout, rep); uncordonErr != nil {
				retErr = errors.Join(retErr, uncordonErr)
			}
		}
	}()

	err = rebootInternal(ctx, upgradeCmdFlags.wait, upgradeCmdFlags.debug, upgradeCmdFlags.timeout, rep, opts...)
	if err != nil {
		return fmt.Errorf("error during upgrade: %w", err)
	}

	return nil
}

func upgradeInternal(ctx context.Context, c *client.Client, containerdInstance *common.ContainerdInstance, nodes []string, imageRef string, rep *reporter.Reporter) (map[string]int32, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		errs error
		w    lifecycle.ProgressWriter
	)

	finishedUpgrades := map[string]int32{}

	responseChan := multiplex.Streaming(
		ctx, nodes,
		func(ctx context.Context) (grpc.ServerStreamingClient[machine.LifecycleServiceUpgradeResponse], error) {
			return c.LifecycleClient.Upgrade(ctx, &machine.LifecycleServiceUpgradeRequest{
				Containerd: containerdInstance,
				Source: &machine.InstallArtifactsSource{
					ImageName: imageRef,
				},
			})
		},
	)

	for resp := range responseChan {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

			continue
		}

		switch resp.Payload.GetProgress().GetResponse().(type) {
		case *machine.LifecycleServiceInstallProgress_Message:
			w.UpdateJob(resp.Node, resp.Payload.GetProgress())

			w.PrintLayerProgress(rep)
		case *machine.LifecycleServiceInstallProgress_ExitCode:
			finishedUpgrades[resp.Node] = resp.Payload.GetProgress().GetExitCode()

			w.UpdateJob(resp.Node, resp.Payload.GetProgress())

			w.PrintLayerProgress(rep)
		}
	}

	if len(finishedUpgrades) > 0 {
		var sb strings.Builder

		status := reporter.StatusSucceeded

		for node, exitCode := range finishedUpgrades {
			if exitCode != 0 {
				errs = errors.Join(errs, fmt.Errorf("node %s: upgrade failed with exit code %d", node, exitCode))

				status = reporter.StatusError

				fmt.Fprintf(&sb, "%s: upgrade failed with exit code %d\n", node, exitCode)
			} else {
				fmt.Fprintf(&sb, "%s: upgrade completed\n", node)
			}
		}

		rep.Report(reporter.Update{
			Message: sb.String(),
			Status:  status,
		})
	}

	return finishedUpgrades, errs
}

// upgradeLegacy dispatches to the legacy upgrade path, respecting --wait.
//
// Note: remove me in Talos 1.18.
func upgradeLegacy(ctx context.Context) error {
	rebootModeStr := strings.ToUpper(upgradeCmdFlags.rebootMode.String())

	rebootMode, ok := machine.UpgradeRequest_RebootMode_value[rebootModeStr]
	if !ok {
		return fmt.Errorf("invalid reboot mode: %s", upgradeCmdFlags.rebootMode)
	}

	opts := []client.UpgradeOption{
		client.WithUpgradeImage(upgradeCmdFlags.upgradeImage),
		client.WithUpgradeRebootMode(machine.UpgradeRequest_RebootMode(rebootMode)),
		client.WithUpgradePreserve(upgradeCmdFlags.preserve),
		client.WithUpgradeStage(upgradeCmdFlags.stage),
		client.WithUpgradeForce(upgradeCmdFlags.force),
	}

	if !upgradeCmdFlags.wait {
		return runUpgradeLegacyNoWaitWithOpts(ctx, opts)
	}

	return action.NewTracker(
		&GlobalArgs,
		action.MachineReadyEventFn,
		func(ctx context.Context, c *client.Client) (string, error) {
			return upgradeGetActorID(ctx, c, opts)
		},
		action.WithPostCheck(action.BootIDChangedPostCheckFn),
		action.WithDebug(upgradeCmdFlags.debug),
		action.WithTimeout(upgradeCmdFlags.timeout),
	).Run(ctx)
}

// runUpgradeLegacyNoWaitWithOpts runs the legacy upgrade without waiting.
//
// Note: remove me in Talos 1.18.
func runUpgradeLegacyNoWaitWithOpts(ctx context.Context, opts []client.UpgradeOption) error {
	if upgradeCmdFlags.insecure {
		return WithClientMaintenance(ctx, nil, func(ctx context.Context, c *client.Client) error {
			return doUpgradeLegacy(ctx, c, opts)
		})
	}

	return WithClient(ctx, func(ctx context.Context, c *client.Client) error {
		return doUpgradeLegacy(ctx, c, opts)
	})
}

// doUpgradeLegacy performs the legacy MachineService.Upgrade call on an existing client.
//
// Note: remove me in Talos 1.18.
func doUpgradeLegacy(ctx context.Context, c *client.Client, opts []client.UpgradeOption) error {
	if err := helpers.ClientVersionCheck(ctx, c); err != nil {
		return err
	}

	var remotePeer peer.Peer

	opts = append(opts, client.WithUpgradeGRPCCallOptions(grpc.Peer(&remotePeer)))

	// TODO: See if we can validate version and prevent starting upgrades to an unknown version
	resp, err := c.UpgradeWithOptions(ctx, opts...) //nolint:staticcheck // legacy talosctl methods, to be removed in Talos 1.18
	if err != nil {
		if resp == nil {
			return fmt.Errorf("error performing upgrade: %s", err)
		}

		cli.Warning("%s", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tACK\tSTARTED")

	defaultNode := client.AddrFromPeer(&remotePeer)

	for _, msg := range resp.Messages {
		node := defaultNode

		if msg.Metadata != nil {
			node = msg.Metadata.Hostname
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t\n", node, msg.Ack, time.Now())
	}

	return w.Flush()
}

// upgradeGetActorID is used by the legacy action tracker path.
//
// Note: remove me in Talos 1.18.
func upgradeGetActorID(ctx context.Context, c *client.Client, opts []client.UpgradeOption) (string, error) {
	resp, err := c.UpgradeWithOptions(ctx, opts...) //nolint:staticcheck // legacy talosctl methods, to be removed in Talos 1.18
	if err != nil {
		return "", err
	}

	if len(resp.GetMessages()) == 0 {
		return "", errors.New("no messages returned from action run")
	}

	return resp.GetMessages()[0].GetActorId(), nil
}

func init() {
	upgradeCmd.Flags().StringVarP(&upgradeCmdFlags.upgradeImage, "image", "i",
		fmt.Sprintf("%s:%s", images.InstallerImageRepository("metal"), version.Trim(version.Tag)),
		"the container image to use for performing the install")
	upgradeCmd.Flags().StringVar(
		&upgradeCmdFlags.namespace, "namespace", "system",
		"namespace to use: \"system\" (etcd and kubelet images), \"cri\" for all Kubernetes workloads, \"inmem\" for in-memory containerd instance",
	)
	upgradeCmd.Flags().VarP(
		upgradeCmdFlags.rebootMode, "reboot-mode", "m",
		fmt.Sprintf(
			"select the reboot mode during upgrade. Mode %q bypasses kexec. Values: %v",
			strings.ToLower(machine.UpgradeRequest_POWERCYCLE.String()),
			upgradeCmdFlags.rebootMode.Options(),
		),
	)
	upgradeCmd.Flags().Var(upgradeCmdFlags.progress, "progress", fmt.Sprintf("output mode for upgrade progress. Values: %v", upgradeCmdFlags.progress.Options()))
	upgradeCmd.Flags().BoolVar(&upgradeCmdFlags.drain, "drain", true, "drain the Kubernetes node before rebooting (cordon + evict pods)")
	upgradeCmd.Flags().DurationVar(&upgradeCmdFlags.drainTimeout, "drain-timeout", nodedrain.DefaultDrainTimeout, "timeout for draining the Kubernetes node")

	// Mark legacy-only flags as deprecated. These are only used when falling back
	// to the legacy MachineService.Upgrade unary API for older Talos versions.
	//
	// Note: remove me in Talos 1.18.
	upgradeCmdFlags.addTrackActionFlags(upgradeCmd)
	upgradeCmd.Flags().BoolVar(&upgradeCmdFlags.legacy, "legacy", false, "force use of legacy upgrade method")
	upgradeCmd.Flags().BoolVarP(&upgradeCmdFlags.force, "force", "f", false, "force the upgrade (skip checks on etcd health and members, might lead to data loss)")
	upgradeCmd.Flags().BoolVar(&upgradeCmdFlags.insecure, "insecure", false, "upgrade using the insecure (encrypted with no auth) maintenance service")
	upgradeCmd.Flags().BoolVarP(&upgradeCmdFlags.preserve, "preserve", "p", false, "preserve data")
	upgradeCmd.Flags().BoolVarP(&upgradeCmdFlags.stage, "stage", "s", false, "stage the upgrade to perform it after a reboot")

	for _, flag := range []string{"force", "insecure", "preserve", "stage"} {
		upgradeCmd.Flags().MarkDeprecated(flag, "legacy flag for MachineService.Upgrade fallback, to be removed in Talos 1.18") //nolint:errcheck
	}

	addCommand(upgradeCmd)
}
