// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/talos/lifecycle"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/action"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/nodedrain"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/safeout"
	"github.com/siderolabs/talos/pkg/flags"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/version"
	"github.com/siderolabs/talos/pkg/reporter"
)

const (
	upgradeImageFlagName     = "image"
	imageFactoryHostFlagName = "image-factory-host"
	schematicIDFlagName      = "schematic-id"
	talosVersionFlagName     = "talos-version"
	secureBootFlagName       = "secure-boot"
	platformFlagName         = "platform"
)

var upgradeCmdFlags = struct {
	trackableActionCmdFlags
	imageCmdFlagsType

	upgradeImage     string
	imageFactoryHost string
	schematicID      string
	talosVersion     string
	secureBoot       bool
	platform         string

	noReboot   bool
	rebootMode flags.PflagExtended[machine.RebootRequest_Mode]
	progress   flags.PflagExtended[reporter.OutputMode]

	drain        bool
	drainTimeout time.Duration

	legacy   bool
	force    bool // Deprecated: only used for legacy upgrade path, to be removed in Talos 1.18.
	preserve bool // Deprecated: only used for legacy upgrade path, to be removed in Talos 1.18.
	stage    bool // Deprecated: only used for legacy upgrade path, to be removed in Talos 1.18.
}{
	rebootMode: flags.ProtoEnum(machine.RebootRequest_DEFAULT, machine.RebootRequest_Mode_value, machine.RebootRequest_Mode_name),
	progress:   reporter.NewOutputModeFlag(),
}

const upgradeCmdDescription = `Upgrade Talos on the target node(s)

For nodes running Talos versions before 1.14 supplying an --image flag is necessary.
For nodes running Talos 1.14 and newer the upgrade image is automatically detected for each node.
It can be optionally modified with the cli flags.`

// upgradeCmd represents the processes command.
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: upgradeCmdDescription,
	Long:  ``,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if upgradeCmdFlags.debug {
			upgradeCmdFlags.wait = true
		}

		if upgradeCmdFlags.drain {
			upgradeCmdFlags.wait = true
		}

		return upgradeRun(cmd.Context(), cmd.Flags())
	},
}

func upgradeRun(ctx context.Context, flags *pflag.FlagSet) error {
	clientFactory, err := NewClientFactory(ctx, &upgradeCmdFlags, action.GRPCDialOptions()...)
	if err != nil {
		return err
	}

	defer clientFactory.Close() //nolint:errcheck

	return upgradeViaLifecycleService(ctx, clientFactory, flags)
}

var (
	talosUpgradeAPIVersionRange            = semver.MustParseRange(">1.13.0-alpha.2 <2.0.0")
	talosImageFactorySchematicVersionRange = semver.MustParseRange(">=1.14.0")
)

// upgradeViaLifecycleService tries the new LifecycleService.Upgrade streaming API.
// If the server returns codes.Unimplemented, it falls back to the legacy MachineService.Upgrade.
//
//nolint:gocyclo,cyclop
func upgradeViaLifecycleService(ctx context.Context, clientFactory *global.ClientFactory, flags *pflag.FlagSet) (retErr error) {
	if upgradeCmdFlags.debug {
		upgradeCmdFlags.wait = true
	}

	if upgradeCmdFlags.noReboot {
		upgradeCmdFlags.drain = false
	}

	if upgradeCmdFlags.legacy {
		safeout.Warningf("Forcing use of legacy upgrade method. This flag is deprecated and will be removed in Talos 1.18.")

		if err := requireImageFlag(flags); err != nil {
			return err
		}

		return upgradeLegacy(ctx, clientFactory)
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
		reporter.WithLineFilter(safeout.String),
	)

	if err = helpers.TalosVersionCheck(ctx, clientFactory, talosUpgradeAPIVersionRange); err != nil {
		if xerrors.TagIs[helpers.VersionOutsideRangeError](err) {
			if err = requireImageFlag(flags); err != nil {
				return err
			}

			rep.Report(reporter.Update{
				Status:  reporter.StatusError,
				Message: "New upgrade API is not available, falling back to legacy",
			})

			return upgradeLegacy(ctx, clientFactory)
		}

		return fmt.Errorf("error checking Talos version compatibility: %w", err)
	}

	// Talos < 1.14 doesn't have the ImageFactorySchematic resource, so the install image can't be derived from the node state.
	if upgradeCmdFlags.upgradeImage == "" {
		if err = helpers.TalosVersionCheck(ctx, clientFactory, talosImageFactorySchematicVersionRange); err != nil {
			if xerrors.TagIs[helpers.VersionOutsideRangeError](err) {
				return requireImageFlag(flags)
			}

			return fmt.Errorf("error checking Talos version compatibility: %w", err)
		}
	}

	nodes := clientFactory.Nodes()

	nodeImageMap, err := getNodeInstallImageMap(ctx, nodes, clientFactory, flags)
	if err != nil {
		return fmt.Errorf("failed to build the upgrade image: %w", err)
	}

	_, err = imagePullPerNode(ctx, clientFactory, containerdInstance, nodeImageMap, rep)
	if err != nil {
		return fmt.Errorf("error pulling upgrade image: %w", err)
	}

	_, err = upgradeInternal(ctx, clientFactory, containerdInstance, nodeImageMap, rep)
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

	if !upgradeCmdFlags.noReboot {
		err = rebootInternal(ctx, clientFactory, upgradeCmdFlags.wait, upgradeCmdFlags.debug, upgradeCmdFlags.timeout, rep, opts...)
		if err != nil {
			return fmt.Errorf("error during upgrade: %w", err)
		}
	}

	return nil
}

type nodeImageParams struct {
	factoryHost string
	schematic   string
	platform    string
	secureBoot  bool
}

// changedImageFlags tracks which install image components were set explicitly via flags.
type changedImageFlags struct {
	factoryHost bool
	schematic   bool
	platform    bool
	secureBoot  bool
}

func newChangedImageFlags(flags *pflag.FlagSet) changedImageFlags {
	return changedImageFlags{
		factoryHost: flags.Changed(imageFactoryHostFlagName),
		schematic:   flags.Changed(schematicIDFlagName),
		platform:    flags.Changed(platformFlagName),
		secureBoot:  flags.Changed(secureBootFlagName),
	}
}

func (c changedImageFlags) all() bool {
	return c.factoryHost && c.schematic && c.platform && c.secureBoot
}

// readNodeImageParams reads the install image components from the node state, skipping the ones set explicitly via flags.
//
//nolint:gocyclo
func readNodeImageParams(ctx context.Context, st state.State, changed changedImageFlags) (nodeImageParams, error) {
	p := nodeImageParams{
		factoryHost: upgradeCmdFlags.imageFactoryHost,
		schematic:   upgradeCmdFlags.schematicID,
		platform:    upgradeCmdFlags.platform,
		secureBoot:  upgradeCmdFlags.secureBoot,
	}

	if !changed.factoryHost || !changed.schematic {
		schematic, err := safe.StateGetByID[*runtime.ImageFactorySchematic](ctx, st, runtime.ImageFactorySchematicID)
		if err != nil {
			return nodeImageParams{}, fmt.Errorf("error reading image factory schematic: %w", err)
		}

		if !changed.factoryHost {
			factoryURL, err := helpers.ParseImageFactoryURL(schematic.TypedSpec().APIURL)
			if err != nil {
				return nodeImageParams{}, err
			}

			p.factoryHost = factoryURL.Host
		}

		if !changed.schematic {
			p.schematic = schematic.TypedSpec().SchematicID
		}
	}

	if !changed.platform {
		platform, err := safe.StateGetByID[*runtime.PlatformMetadata](ctx, st, runtime.PlatformMetadataID)
		if err != nil {
			return nodeImageParams{}, fmt.Errorf("error reading platform metadata: %w", err)
		}

		p.platform = platform.TypedSpec().Platform
	}

	if !changed.secureBoot {
		security, err := safe.StateGetByID[*runtime.SecurityState](ctx, st, runtime.SecurityStateID)
		if err != nil {
			return nodeImageParams{}, fmt.Errorf("error reading security state: %w", err)
		}

		p.secureBoot = security.TypedSpec().SecureBoot
	}

	return p, nil
}

// checkImageFlagConflicts returns an error if any of the image component flags is set along with --image.
func checkImageFlagConflicts(flags *pflag.FlagSet) error {
	var conflicting []string

	for _, name := range []string{
		imageFactoryHostFlagName,
		schematicIDFlagName,
		talosVersionFlagName,
		secureBootFlagName,
		platformFlagName,
	} {
		if flags.Changed(name) {
			conflicting = append(conflicting, "--"+name)
		}
	}

	if len(conflicting) > 0 {
		return fmt.Errorf("--%s flag cannot be used together with: %s", upgradeImageFlagName, strings.Join(conflicting, ", "))
	}

	return nil
}

// requireImageFlag returns an error if --image is not set or any of the image component flags is set.
//
// It is used for the nodes which can't report the components to build the install image from.
func requireImageFlag(flags *pflag.FlagSet) error {
	if upgradeCmdFlags.upgradeImage == "" {
		return fmt.Errorf("--%s flag is required for updating nodes running Talos versions before 1.14", upgradeImageFlagName)
	}

	return checkImageFlagConflicts(flags)
}

//nolint:gocyclo,cyclop
func getNodeInstallImageMap(ctx context.Context, nodes []string, clientFactory *global.ClientFactory, flags *pflag.FlagSet) (map[string]string, error) {
	changed := newChangedImageFlags(flags)

	nodeImageMap := map[string]string{}
	uniformImage := ""
	// If --image is set, ignore the component flags and use it for upgrading every node (legacy behavior).
	if upgradeCmdFlags.upgradeImage != "" {
		if err := checkImageFlagConflicts(flags); err != nil {
			return nil, err
		}

		uniformImage = upgradeCmdFlags.upgradeImage
	}

	// If every derived component is set explicitly, no per-node state is needed: the image is uniform.
	if changed.all() {
		uniformImage = helpers.BuildInstallerImageRef(
			upgradeCmdFlags.imageFactoryHost,
			upgradeCmdFlags.schematicID,
			upgradeCmdFlags.talosVersion,
			upgradeCmdFlags.platform,
			upgradeCmdFlags.secureBoot,
		)
	}

	if uniformImage != "" {
		for _, node := range nodes {
			nodeImageMap[node] = uniformImage
		}

		return nodeImageMap, nil
	}

	respCh := multiplex.UnaryViaFactory(ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (nodeImageParams, error) {
			return readNodeImageParams(ctx, c.COSI, changed)
		})

	var errs error

	for resp := range respCh {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("node %s: %w", resp.Node, resp.Err))

			continue
		}

		p := resp.Payload

		nodeImageMap[resp.Node] = helpers.BuildInstallerImageRef(p.factoryHost, p.schematic, upgradeCmdFlags.talosVersion, p.platform, p.secureBoot)
	}

	return nodeImageMap, errs
}

func upgradeInternal(
	ctx context.Context,
	clientFactory *global.ClientFactory,
	containerdInstance *common.ContainerdInstance,
	nodeImageMap map[string]string,
	rep *reporter.Reporter,
) (map[string]int32, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var errs error

	w := lifecycle.NewProgressWriter(rep.IsColorized())

	finishedUpgrades := map[string]int32{}

	responseChan := multiplex.StreamingViaFactoryPerNode(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client, node multiplex.NodeRef) (grpc.ServerStreamingClient[machine.LifecycleServiceUpgradeResponse], error) {
			return c.LifecycleClient.Upgrade(ctx, &machine.LifecycleServiceUpgradeRequest{
				Containerd: containerdInstance,
				Source: &machine.InstallArtifactsSource{
					ImageName: nodeImageMap[node],
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

				fmt.Fprintln(&sb, w.Failure(node, "upgrade", exitCode))
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
func upgradeLegacy(ctx context.Context, clientFactory *global.ClientFactory) error {
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
				safeout.Warningf("%s", err)
			}

			return resp, nil
		},
	)

	w := tabwriter.NewWriter(safeout.Stdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tACK\tSTARTED")

	var errs error

	for resp := range responseChan {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

			continue
		}

		for _, msg := range resp.Payload.Messages {
			safeout.Fprintf(w, "%s\t%s\t%s\t\n", resp.Node, msg.Ack, time.Now())
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
	upgradeCmd.Flags().StringVarP(&upgradeCmdFlags.upgradeImage, upgradeImageFlagName, "i", "",
		"the container image to use for performing the install")
	upgradeCmd.Flags().MarkHidden(upgradeImageFlagName) //nolint:errcheck

	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.imageFactoryHost, imageFactoryHostFlagName, "",
		"Image Factory host to use for the installer image, e.g. factory.talos.dev (defaults to the node's factory)")
	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.schematicID, schematicIDFlagName, "",
		"Image Factory schematic ID to use for the installer image (defaults to the node's current schematic)")
	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.talosVersion, talosVersionFlagName, version.Tag,
		"Talos version to upgrade to (defaults to talosctl version)")
	upgradeCmd.Flags().BoolVar(&upgradeCmdFlags.secureBoot, secureBootFlagName, false,
		"use the SecureBoot installer image (defaults to the node's current SecureBoot state)")
	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.platform, platformFlagName, "",
		"platform to use for the installer image (defaults to the node's platform)")

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
	upgradeCmd.Flags().BoolVar(&upgradeCmdFlags.noReboot, "no-reboot", false, "do not reboot the node after upgrade (skip reboot and drain)")
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
		helpers.MarkFlagDeprecated(upgradeCmd.Flags(), flag, "legacy flag for MachineService.Upgrade fallback, to be removed in Talos 1.18") //nolint:errcheck
	}

	addCommand(upgradeCmd)
}
