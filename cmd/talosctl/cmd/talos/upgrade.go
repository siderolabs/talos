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

	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos/lifecycle"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/action"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/nodedrain"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/flags"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/reporter"
)

var upgradeCmdFlags = struct {
	trackableActionCmdFlags
	imageCmdFlagsType

	factory      string
	schematic    string
	talosVersion string
	secureBoot   bool
	platform     string

	upgradeImage string
	rebootMode   flags.PflagExtended[machine.RebootRequest_Mode]
	progress     flags.PflagExtended[reporter.OutputMode]

	drain        bool
	drainTimeout time.Duration

	// set in RunE: whether --secure-boot / --factory / any component flag was set explicitly.
	secureBootChanged     bool
	factoryChanged        bool
	componentFlagsChanged bool

	legacy   bool
	force    bool // Deprecated: only used for legacy upgrade path, to be removed in Talos 1.18.
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

		upgradeCmdFlags.secureBootChanged = cmd.Flags().Changed("secure-boot")
		upgradeCmdFlags.factoryChanged = cmd.Flags().Changed("factory")

		for _, name := range []string{"factory", "schematic", "talos-version", "secure-boot", "platform"} {
			if cmd.Flags().Changed(name) {
				upgradeCmdFlags.componentFlagsChanged = true

				break
			}
		}

		return upgradeRun(cmd.Context())
	},
}

func upgradeRun(ctx context.Context) error {
	clientFactory, err := NewClientFactory(ctx, &upgradeCmdFlags, action.GRPCDialOptions()...)
	if err != nil {
		return err
	}

	defer clientFactory.Close() //nolint:errcheck

	return upgradeViaLifecycleService(ctx, clientFactory)
}

var talosUpgradeAPIVersionRange = semver.MustParseRange(">1.13.0-alpha.2 <2.0.0")

// upgradeViaLifecycleService tries the new LifecycleService.Upgrade streaming API.
// If the server returns codes.Unimplemented, it falls back to the legacy MachineService.Upgrade.
//
//nolint:gocyclo
func upgradeViaLifecycleService(ctx context.Context, clientFactory *global.ClientFactory) (retErr error) {
	if upgradeCmdFlags.debug {
		upgradeCmdFlags.wait = true
	}

	imageRef, err := resolveUpgradeImage(ctx, clientFactory)
	if err != nil {
		return err
	}

	if upgradeCmdFlags.legacy {
		cli.Warning("Forcing use of legacy upgrade method. This flag is deprecated and will be removed in Talos 1.18.")

		return upgradeLegacy(ctx, clientFactory, imageRef)
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

	if err = helpers.TalosVersionCheck(ctx, clientFactory, talosUpgradeAPIVersionRange); err != nil {
		if xerrors.TagIs[helpers.VersionOutsideRangeError](err) {
			rep.Report(reporter.Update{
				Status:  reporter.StatusError,
				Message: "New upgrade API is not available, falling back to legacy",
			})

			return upgradeLegacy(ctx, clientFactory, imageRef)
		}

		return fmt.Errorf("error checking Talos version compatibility: %w", err)
	}

	_, err = imagePullInternal(ctx, clientFactory, containerdInstance, imageRef, rep)
	if err != nil {
		return fmt.Errorf("error pulling upgrade image: %w", err)
	}

	_, err = upgradeInternal(ctx, clientFactory, containerdInstance, imageRef, rep)
	if err != nil {
		return fmt.Errorf("error during upgrade: %w", err)
	}

	var nodeNames map[string]string

	if upgradeCmdFlags.drain {
		nodeNames, err = drainNodes(ctx, clientFactory, upgradeCmdFlags.drainTimeout, rep)
		if err != nil {
			return err
		}
	}

	defer func() {
		if !upgradeCmdFlags.drain {
			return
		}

		if len(nodeNames) > 0 {
			if uncordonErr := uncordonNodes(ctx, clientFactory, nodeNames, upgradeCmdFlags.timeout, rep); uncordonErr != nil {
				retErr = errors.Join(retErr, uncordonErr)
			}
		}
	}()

	err = rebootInternal(ctx, clientFactory, upgradeCmdFlags.wait, upgradeCmdFlags.debug, upgradeCmdFlags.timeout, rep, opts...)
	if err != nil {
		return fmt.Errorf("error during upgrade: %w", err)
	}

	return nil
}

// resolveUpgradeImage resolves the installer image reference for the upgrade.
//
// If --image is set, it is used verbatim (legacy behavior). Otherwise the reference is built
// as <factory>/<platform>-installer[-secureboot]/<schematic>:<version> from the component flags
// (--factory, --schematic, --talos-version, --secure-boot, --platform), filling in components
// not set explicitly from the machine's state.
//
//nolint:gocyclo
func resolveUpgradeImage(ctx context.Context, clientFactory *global.ClientFactory) (string, error) {
	if upgradeCmdFlags.upgradeImage != "" {
		if upgradeCmdFlags.componentFlagsChanged {
			cli.Warning("--image is set, ignoring component flags (--factory, --schematic, --talos-version, --secure-boot, --platform)")
		}

		return upgradeCmdFlags.upgradeImage, nil
	}

	targetVersion := upgradeCmdFlags.talosVersion
	if targetVersion == "" {
		targetVersion = version.Tag
	}

	factory := upgradeCmdFlags.factory
	schematic := upgradeCmdFlags.schematic
	secureBoot := upgradeCmdFlags.secureBoot
	platform := upgradeCmdFlags.platform

	// Query the machine state only for the components which were not set explicitly.
	if schematic == "" || platform == "" || !upgradeCmdFlags.secureBootChanged || !upgradeCmdFlags.factoryChanged {
		nodes := clientFactory.Nodes()
		if len(nodes) > 1 {
			cli.Warning("multiple nodes specified, resolving the upgrade image from node %s", nodes[0])
		}

		queryCtx, c, err := clientFactory.BuildClientFirstNode(ctx)
		if err != nil {
			return "", fmt.Errorf("error building client to resolve the upgrade image (use --image or set all component flags to skip): %w", err)
		}

		machineCtx, err := helpers.QueryMachineContext(queryCtx, c)
		if err != nil {
			return "", fmt.Errorf("error reading machine state to resolve the upgrade image (use --image or set all component flags to skip): %w", err)
		}

		if schematic == "" {
			schematic = machineCtx.Schematic

			if schematic == "" {
				cli.Warning("machine schematic ID is not known, using the default (empty) schematic")
			}
		}

		if platform == "" {
			platform = machineCtx.Platform
		}

		if !upgradeCmdFlags.secureBootChanged {
			secureBoot = machineCtx.SecureBoot
		}

		// Prefer the factory host the machine was actually installed from, so a node
		// built from a private/mirror factory upgrades against the same one.
		if !upgradeCmdFlags.factoryChanged && machineCtx.FactoryHost != "" {
			factory = machineCtx.FactoryHost
		}
	}

	if platform == "" {
		platform = "metal"
	}

	imageRef := helpers.BuildImageFactoryURL(factory, schematic, targetVersion, platform, secureBoot)

	fmt.Printf("upgrade image: %s\n", imageRef)

	return imageRef, nil
}

func upgradeInternal(ctx context.Context, clientFactory *global.ClientFactory, containerdInstance *common.ContainerdInstance, imageRef string, rep *reporter.Reporter) (map[string]int32, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		errs error
		w    lifecycle.ProgressWriter
	)

	finishedUpgrades := map[string]int32{}

	responseChan := multiplex.StreamingViaFactory(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (grpc.ServerStreamingClient[machine.LifecycleServiceUpgradeResponse], error) {
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
func upgradeLegacy(ctx context.Context, clientFactory *global.ClientFactory, imageRef string) error {
	rebootModeStr := strings.ToUpper(upgradeCmdFlags.rebootMode.String())

	rebootMode, ok := machine.UpgradeRequest_RebootMode_value[rebootModeStr]
	if !ok {
		return fmt.Errorf("invalid reboot mode: %s", upgradeCmdFlags.rebootMode)
	}

	opts := []client.UpgradeOption{
		client.WithUpgradeImage(imageRef),
		client.WithUpgradeRebootMode(machine.UpgradeRequest_RebootMode(rebootMode)),
		client.WithUpgradePreserve(upgradeCmdFlags.preserve),
		client.WithUpgradeStage(upgradeCmdFlags.stage),
		client.WithUpgradeForce(upgradeCmdFlags.force),
	}

	if !upgradeCmdFlags.wait {
		return runUpgradeLegacyNoWaitWithOpts(ctx, opts)
	}

	return action.NewTracker(
		clientFactory,
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
	clientFactory, err := NewClientFactory(ctx, &upgradeCmdFlags)
	if err != nil {
		return err
	}

	defer clientFactory.Close() //nolint:errcheck

	return doUpgradeLegacy(ctx, clientFactory, opts)
}

// doUpgradeLegacy performs the legacy MachineService.Upgrade call across all nodes.
//
// Note: remove me in Talos 1.18.
func doUpgradeLegacy(ctx context.Context, clientFactory *global.ClientFactory, opts []client.UpgradeOption) error {
	if err := helpers.ClientVersionCheck(ctx, clientFactory); err != nil {
		return err
	}

	responseChan := multiplex.UnaryViaFactory(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (*machine.UpgradeResponse, error) {
			// TODO: See if we can validate version and prevent starting upgrades to an unknown version
			resp, err := c.UpgradeWithOptions(ctx, opts...) //nolint:staticcheck // legacy talosctl methods, to be removed in Talos 1.18
			if err != nil {
				if resp == nil {
					return nil, fmt.Errorf("error performing upgrade: %w", err)
				}

				// partial success: the upgrade was acknowledged but some non-fatal error occurred
				cli.Warning("%s", err)
			}

			return resp, nil
		},
	)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tACK\tSTARTED")

	var errs error

	for resp := range responseChan {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

			continue
		}

		for _, msg := range resp.Payload.Messages {
			fmt.Fprintf(w, "%s\t%s\t%s\t\n", resp.Node, msg.Ack, time.Now())
		}
	}

	return errors.Join(errs, w.Flush())
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
	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.factory, "factory", images.Factory,
		"Image Factory host to use for the installer image")
	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.schematic, "schematic", "",
		"Image Factory schematic ID to use for the installer image (defaults to the machine's current schematic)")
	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.talosVersion, "talos-version", "",
		fmt.Sprintf("Talos version to upgrade to (defaults to talosctl version %s)", version.Tag))
	upgradeCmd.Flags().BoolVar(&upgradeCmdFlags.secureBoot, "secure-boot", false,
		"use the SecureBoot installer image (defaults to the machine's current SecureBoot state)")
	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.platform, "platform", "",
		"platform to use for the installer image (defaults to the machine's platform)")

	// --image overrides the component flags; hidden as a legacy way to specify the installer image.
	upgradeCmd.Flags().StringVarP(&upgradeCmdFlags.upgradeImage, "image", "i", "",
		"the container image to use for performing the install (overrides the component flags)")
	upgradeCmd.Flags().MarkHidden("image") //nolint:errcheck

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
	upgradeCmd.Flags().BoolVarP(&upgradeCmdFlags.preserve, "preserve", "p", false, "preserve data")
	upgradeCmd.Flags().BoolVarP(&upgradeCmdFlags.stage, "stage", "s", false, "stage the upgrade to perform it after a reboot")

	for _, flag := range []string{"force", "insecure", "preserve", "stage"} {
		upgradeCmd.Flags().MarkDeprecated(flag, "legacy flag for MachineService.Upgrade fallback, to be removed in Talos 1.18") //nolint:errcheck
	}

	addCommand(upgradeCmd)
}
