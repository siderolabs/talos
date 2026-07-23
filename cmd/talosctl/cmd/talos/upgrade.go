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
	"sync"
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

	imageRefs, err := resolveUpgradeImages(ctx, clientFactory)
	if err != nil {
		return err
	}

	if upgradeCmdFlags.legacy {
		cli.Warning("Forcing use of legacy upgrade method. This flag is deprecated and will be removed in Talos 1.18.")

		return upgradeLegacy(ctx, clientFactory, imageRefs)
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

			return upgradeLegacy(ctx, clientFactory, imageRefs)
		}

		return fmt.Errorf("error checking Talos version compatibility: %w", err)
	}

	_, err = imagePullInternal(ctx, clientFactory, containerdInstance, imageRefs, rep)
	if err != nil {
		return fmt.Errorf("error pulling upgrade image: %w", err)
	}

	_, err = upgradeInternal(ctx, clientFactory, containerdInstance, imageRefs, rep)
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

// resolveUpgradeImages resolves the installer image reference for each targeted node.
//
// If --image is set, it is used verbatim for every node (legacy behavior). Otherwise each node's
// reference is built independently as <factory>/<platform>-installer[-secureboot]/<schematic>:<version>
// from the component flags (--factory, --schematic, --talos-version, --secure-boot, --platform),
// filling in components not set explicitly from that node's own machine state. Resolving per node
// lets a single invocation upgrade a heterogeneous cluster correctly (e.g. nodes with different
// schematics, platforms, or SecureBoot state) instead of assuming a uniform configuration.
//
// If a node's state cannot be read, that node falls back to the built-in defaults (empty schematic,
// metal platform, public factory, non-SecureBoot).
func resolveUpgradeImages(ctx context.Context, clientFactory *global.ClientFactory) (map[string]string, error) {
	nodes := clientFactory.Nodes()

	if upgradeCmdFlags.upgradeImage != "" {
		if upgradeCmdFlags.componentFlagsChanged {
			cli.Warning("--image is set, ignoring component flags (--factory, --schematic, --talos-version, --secure-boot, --platform)")
		}

		return uniformImageRefs(nodes, upgradeCmdFlags.upgradeImage), nil
	}

	// If every derived component is set explicitly, no per-node state is needed: the image is uniform.
	if upgradeCmdFlags.schematic != "" && upgradeCmdFlags.platform != "" &&
		upgradeCmdFlags.secureBootChanged && upgradeCmdFlags.factoryChanged && upgradeCmdFlags.talosVersion != ""{
		imageRef := buildUpgradeImage(&helpers.MachineContext{})

		fmt.Printf("upgrade image: %s\n", imageRef)

		return uniformImageRefs(nodes, imageRef), nil
	}

	imageRefs := make(map[string]string, len(nodes))

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)

	for _, node := range nodes {
		wg.Add(1)

		go func() {
			defer wg.Done()

			machineCtx := &helpers.MachineContext{}

			queryCtx, c, err := clientFactory.BuildClient(ctx, node)
			if err != nil {
				cli.Warning("node %s: error building client to read machine state, using the default installer image: %v", node, err)
			} else if machineCtx, err = helpers.QueryMachineContext(queryCtx, c); err != nil {
				cli.Warning("node %s: error reading machine state, using the default installer image: %v", node, err)

				machineCtx = &helpers.MachineContext{}
			}

			imageRef := buildUpgradeImage(machineCtx)

			mu.Lock()
			imageRefs[node] = imageRef
			fmt.Printf("%s: upgrade image: %s\n", node, imageRef)
			mu.Unlock()
		}()
	}

	wg.Wait()

	return imageRefs, nil
}

// buildUpgradeImage builds the installer image reference for a single node, taking each component
// from the explicit flag when set and otherwise from the node's machine state, with the built-in
// defaults as the final fallback.
func buildUpgradeImage(machineCtx *helpers.MachineContext) string {
	targetVersion := upgradeCmdFlags.talosVersion
	if targetVersion == "" {
		targetVersion = version.Tag
	}

	schematic := upgradeCmdFlags.schematic
	if schematic == "" {
		schematic = machineCtx.Schematic
	}

	if schematic == "" {
        schematic = images.DefaultInstallerImageSchematic
    }

	platform := upgradeCmdFlags.platform
	if platform == "" {
		platform = machineCtx.Platform
	}

	if platform == "" {
		platform = "metal"
	}

	secureBoot := upgradeCmdFlags.secureBoot
	if !upgradeCmdFlags.secureBootChanged {
		secureBoot = machineCtx.SecureBoot
	}

	factory := upgradeCmdFlags.factory
	if !upgradeCmdFlags.factoryChanged {
		factory = machineCtx.FactoryHost
	}

	if factory == "" {
		factory = images.Factory
	}

	return helpers.BuildImageFactoryURL(factory, schematic, targetVersion, platform, secureBoot)
}

func upgradeInternal(ctx context.Context, clientFactory *global.ClientFactory, containerdInstance *common.ContainerdInstance, imageRefs map[string]string, rep *reporter.Reporter) (map[string]int32, error) {
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
					ImageName: imageRefs[nodeFromContext(ctx)],
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
func upgradeLegacy(ctx context.Context, clientFactory *global.ClientFactory, imageRefs map[string]string) error {
	rebootModeStr := strings.ToUpper(upgradeCmdFlags.rebootMode.String())

	rebootMode, ok := machine.UpgradeRequest_RebootMode_value[rebootModeStr]
	if !ok {
		return fmt.Errorf("invalid reboot mode: %s", upgradeCmdFlags.rebootMode)
	}

	// The installer image is added per node (see legacyUpgradeOptsForNode), so it is omitted here.
	baseOpts := []client.UpgradeOption{
		client.WithUpgradeRebootMode(machine.UpgradeRequest_RebootMode(rebootMode)),
		client.WithUpgradePreserve(upgradeCmdFlags.preserve),
		client.WithUpgradeStage(upgradeCmdFlags.stage),
		client.WithUpgradeForce(upgradeCmdFlags.force),
	}

	if !upgradeCmdFlags.wait {
		return runUpgradeLegacyNoWaitWithOpts(ctx, baseOpts, imageRefs)
	}

	return action.NewTracker(
		clientFactory,
		action.MachineReadyEventFn,
		func(ctx context.Context, c *client.Client) (string, error) {
			return upgradeGetActorID(ctx, c, baseOpts, imageRefs)
		},
		action.WithPostCheck(action.BootIDChangedPostCheckFn),
		action.WithDebug(upgradeCmdFlags.debug),
		action.WithTimeout(upgradeCmdFlags.timeout),
	).Run(ctx)
}

// legacyUpgradeOptsForNode returns the upgrade options for a single node, appending that node's
// resolved installer image (recovered from the node-scoped context) to the shared base options.
//
// Note: remove me in Talos 1.18.
func legacyUpgradeOptsForNode(ctx context.Context, baseOpts []client.UpgradeOption, imageRefs map[string]string) []client.UpgradeOption {
	opts := make([]client.UpgradeOption, 0, len(baseOpts)+1)
	opts = append(opts, baseOpts...)
	opts = append(opts, client.WithUpgradeImage(imageRefs[nodeFromContext(ctx)]))

	return opts
}

// runUpgradeLegacyNoWaitWithOpts runs the legacy upgrade without waiting.
//
// Note: remove me in Talos 1.18.
func runUpgradeLegacyNoWaitWithOpts(ctx context.Context, baseOpts []client.UpgradeOption, imageRefs map[string]string) error {
	clientFactory, err := NewClientFactory(ctx, &upgradeCmdFlags)
	if err != nil {
		return err
	}

	defer clientFactory.Close() //nolint:errcheck

	return doUpgradeLegacy(ctx, clientFactory, baseOpts, imageRefs)
}

// doUpgradeLegacy performs the legacy MachineService.Upgrade call across all nodes.
//
// Note: remove me in Talos 1.18.
func doUpgradeLegacy(ctx context.Context, clientFactory *global.ClientFactory, baseOpts []client.UpgradeOption, imageRefs map[string]string) error {
	if err := helpers.ClientVersionCheck(ctx, clientFactory); err != nil {
		return err
	}

	responseChan := multiplex.UnaryViaFactory(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (*machine.UpgradeResponse, error) {
			opts := legacyUpgradeOptsForNode(ctx, baseOpts, imageRefs)

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
func upgradeGetActorID(ctx context.Context, c *client.Client, baseOpts []client.UpgradeOption, imageRefs map[string]string) (string, error) {
	opts := legacyUpgradeOptsForNode(ctx, baseOpts, imageRefs)

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
	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.factory, "factory", "",
		"Image Factory host to use for the installer image (defaults to the machine's factory, then factory.talos.dev)")
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
