// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
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
	"github.com/siderolabs/talos/pkg/images"
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
)

var upgradeCmdFlags = struct {
	trackableActionCmdFlags
	imageCmdFlagsType

	upgradeImage     string
	imageFactoryHost string
	schematicID      string
	talosVersion     string
	secureBoot       bool
	// no platform flag as changing the platform via an upgrade is not officially supported

	noReboot   bool
	rebootMode flags.PflagExtended[machine.RebootRequest_Mode]
	progress   flags.PflagExtended[reporter.OutputMode]

	drain        bool
	drainTimeout time.Duration
	k8sEndpoint  string

	legacy   bool
	force    bool // Deprecated: only used for legacy upgrade path, to be removed in Talos 1.18.
	preserve bool // Deprecated: only used for legacy upgrade path, to be removed in Talos 1.18.
	stage    bool // Deprecated: only used for legacy upgrade path, to be removed in Talos 1.18.
}{
	rebootMode: flags.ProtoEnum(machine.RebootRequest_DEFAULT, machine.RebootRequest_Mode_value, machine.RebootRequest_Mode_name),
	progress:   reporter.NewOutputModeFlag(),
}

const (
	upgradeCmdDescription     = "Upgrade Talos on the target node(s)"
	upgradeCmdDescriptionLong = upgradeCmdDescription + "\n" + `
For nodes running Talos versions before 1.14 supplying an --image flag is necessary.

For nodes running Talos 1.14 and newer, the upgrade image is automatically detected for each node.
This means that each node retains its schematic, Image Factory host, platform and secureboot setting as they are set on that node.
The options can be optionally modified with the cli flags.`
)

// upgradeCmd represents the processes command.
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: upgradeCmdDescription,
	Long:  upgradeCmdDescriptionLong,
	Args:  cobra.NoArgs,
	Example: fmt.Sprintf(`# upgrade node(s) to a set version keeping other parameters the same
# (each node retains its schematic, Image Factory host, platform and secureboot setting)
talosctl upgrade -n <NODE(S)> --talos-version 1.14.2

# upgrade node(s) to a different schematic
# (--talos-version defaults to the talosctl version, so set it explicitly to keep the current Talos version)
talosctl upgrade -n <NODE(S)> --%s=e3c92... --talos-version 1.14.2

# upgrade node(s) to an exact image
talosctl upgrade -n <NODE(S)> --image=factory.talos.dev/metal-installer-secureboot/ecc3...b557:v1.14.2`, schematicIDFlagName),
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
	talosImageFactorySchematicVersionRange = semver.MustParseRange(">=1.14.0-alpha.2")
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

		if upgradeCmdFlags.upgradeImage == "" {
			return errors.New("error: --image flag is required with the legacy mode")
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

	lowestVersion, highestVersion, err := helpers.GetTalosVersionBounds(ctx, clientFactory)
	if err != nil {
		return fmt.Errorf("error checking Talos version compatibility: %w", err)
	}

	versionsErr := fmt.Errorf("the Talos versions across the nodes range from %s to %s", lowestVersion, highestVersion)

	// the range is contiguous, so all nodes are within it if both the lowest and the highest versions are
	if !talosUpgradeAPIVersionRange(lowestVersion) || !talosUpgradeAPIVersionRange(highestVersion) {
		if err = requireImageFlag(versionsErr); err != nil {
			return err
		}

		rep.Report(reporter.Update{
			Status:  reporter.StatusError,
			Message: "New upgrade API is not available, falling back to legacy",
		})

		return upgradeLegacy(ctx, clientFactory)
	}

	// Talos < 1.14 doesn't have the ImageFactorySchematic resource, so the install image can't be derived from the node state.
	if !talosImageFactorySchematicVersionRange(lowestVersion) {
		if err = requireImageFlag(versionsErr); err != nil {
			return err
		}
	}

	nodes := clientFactory.Nodes()

	nodeImageMap, err := getNodeInstallImageMap(ctx, nodes, clientFactory, flags)
	if err != nil {
		return fmt.Errorf("failed to build the upgrade image: %w", err)
	}

	for _, node := range slices.Sorted(maps.Keys(nodeImageMap)) {
		fmt.Fprintf(safeout.Stderr(), "%s upgrade image: %s\n", node, nodeImageMap[node])
	}

	pulledImageMap, err := imagePullPerNode(ctx, clientFactory, containerdInstance, nodeImageMap, rep)
	if err != nil {
		return fmt.Errorf("error pulling upgrade image: %w", err)
	}

	// The image is stored on the node under the reference returned by the pull, which might differ from the requested one
	// (e.g. `repo:tag@digest` is stored as `repo@digest`), so upgrade using the pulled reference.
	maps.Copy(nodeImageMap, pulledImageMap)

	_, err = upgradeInternal(ctx, clientFactory, containerdInstance, nodeImageMap, rep)
	if err != nil {
		return fmt.Errorf("error during upgrade: %w", err)
	}

	var nodeNames map[string]string

	if upgradeCmdFlags.drain {
		nodeNames, err = drainNodes(ctx, clientFactory, upgradeCmdFlags.k8sEndpoint, upgradeCmdFlags.drainTimeout, rep)
		if err != nil {
			return err
		}
	}

	defer func() {
		if !upgradeCmdFlags.drain {
			return
		}

		if len(nodeNames) > 0 {
			if uncordonErr := uncordonNodes(ctx, clientFactory, upgradeCmdFlags.k8sEndpoint, nodeNames, upgradeCmdFlags.timeout, rep); uncordonErr != nil {
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

// nodeImageParams are the components the installer image reference is built from.
type nodeImageParams struct {
	factoryHost string
	schematic   string
	platform    string
	version     string
	secureBoot  bool

	// defaultedFactoryHost is set when the node didn't report the Image Factory URL and the default one was used.
	defaultedFactoryHost bool
}

func (p nodeImageParams) imageRef() string {
	version := p.version
	if !strings.HasPrefix(version, "v") && version != "" {
		version = "v" + version
	}

	return images.NewInstallerImage(p.factoryHost, p.platform, p.schematic, version, images.SecureBoot(p.secureBoot))
}

// imageComponentFlagNames are the flags which build the installer image, they can't be used together with --image.
var imageComponentFlagNames = []string{
	imageFactoryHostFlagName,
	schematicIDFlagName,
	talosVersionFlagName,
	secureBootFlagName,
}

// changedImageFlags tracks which install image components were set explicitly via flags.
type changedImageFlags struct {
	factoryHost bool
	schematic   bool
	secureBoot  bool
}

func newChangedImageFlags(flags *pflag.FlagSet) changedImageFlags {
	return changedImageFlags{
		factoryHost: flags.Changed(imageFactoryHostFlagName),
		schematic:   flags.Changed(schematicIDFlagName),
		secureBoot:  flags.Changed(secureBootFlagName),
	}
}

// imageParamsFromFlags returns the install image components as set via flags.
func imageParamsFromFlags() nodeImageParams {
	return nodeImageParams{
		factoryHost: upgradeCmdFlags.imageFactoryHost,
		schematic:   upgradeCmdFlags.schematicID,
		version:     upgradeCmdFlags.talosVersion,
		secureBoot:  upgradeCmdFlags.secureBoot,
	}
}

// readNodeImageParams reads the install image components from the node state, skipping the ones set explicitly.
//
//nolint:gocyclo
func readNodeImageParams(ctx context.Context, st state.State, overrides nodeImageParams, changed changedImageFlags) (nodeImageParams, error) {
	p := overrides

	if !changed.factoryHost || !changed.schematic {
		schematic, err := safe.StateGetByID[*runtime.ImageFactorySchematic](ctx, st, runtime.ImageFactorySchematicID)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nodeImageParams{}, fmt.Errorf(
					"the node was not installed from Image Factory, so the install image can't be detected: "+
						"use --%s, or both --%s and --%s", upgradeImageFlagName, schematicIDFlagName, imageFactoryHostFlagName,
				)
			}

			return nodeImageParams{}, fmt.Errorf("error reading image factory schematic: %w", err)
		}

		if !changed.factoryHost {
			if schematic.TypedSpec().APIURL == "" {
				p.factoryHost = images.Factory
				p.defaultedFactoryHost = true
			} else {
				factoryURL, err := helpers.ParseImageFactoryURL(schematic.TypedSpec().APIURL)
				if err != nil {
					return nodeImageParams{}, err
				}

				p.factoryHost = factoryURL.Host
			}
		}

		if !changed.schematic {
			p.schematic = schematic.TypedSpec().SchematicID
		}
	}

	platform, err := safe.StateGetByID[*runtime.PlatformMetadata](ctx, st, runtime.PlatformMetadataID)
	if err != nil {
		return nodeImageParams{}, fmt.Errorf("error reading platform metadata: %w", err)
	}

	p.platform = strings.ToLower(platform.TypedSpec().Platform)

	if !changed.secureBoot {
		security, err := safe.StateGetByID[*runtime.SecurityState](ctx, st, runtime.SecurityStateID)
		if err != nil {
			return nodeImageParams{}, fmt.Errorf("error reading security state: %w", err)
		}

		p.secureBoot = security.TypedSpec().SecureBoot
	}

	return p, nil
}

// requireImageFlag returns an error if --image is not set.
//
// It is used for the nodes which can't report the components to build the install image from.
// versionErr, if any, describes why the image can't be detected (e.g. which nodes are running old Talos versions).
func requireImageFlag(versionErr error) error {
	if upgradeCmdFlags.upgradeImage != "" {
		return nil
	}

	err := fmt.Errorf("--%s flag is required if any of the nodes runs Talos version before 1.14", upgradeImageFlagName)
	if versionErr != nil {
		err = fmt.Errorf("%w:\n%w", err, versionErr)
	}

	return err
}

// getNodeInstallImageMap returns the installer image for each node.
func getNodeInstallImageMap(ctx context.Context, nodes []string, clientFactory *global.ClientFactory, flags *pflag.FlagSet) (map[string]string, error) {
	overrides := imageParamsFromFlags()
	changed := newChangedImageFlags(flags)

	if upgradeCmdFlags.upgradeImage != "" {
		return uniformImageMap(nodes, upgradeCmdFlags.upgradeImage), nil
	}

	respCh := multiplex.UnaryViaFactory(ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (nodeImageParams, error) {
			return readNodeImageParams(ctx, c.State(), overrides, changed)
		})

	var errs error

	nodeImageMap := make(map[string]string, len(nodes))

	for resp := range respCh {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("node %s: %w", resp.Node, resp.Err))

			continue
		}

		if resp.Payload.defaultedFactoryHost {
			safeout.Warningf("node %s doesn't report the Image Factory URL, using %s (override with --%s)", resp.Node, images.Factory, imageFactoryHostFlagName)
		}

		nodeImageMap[resp.Node] = resp.Payload.imageRef()
	}

	if errs != nil {
		return nil, errs
	}

	return nodeImageMap, nil
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
			return c.LifecycleService().Upgrade(ctx, &machine.LifecycleServiceUpgradeRequest{
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
	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.imageFactoryHost, imageFactoryHostFlagName, "",
		"Image Factory host to use for the installer image, e.g. factory.talos.dev (defaults to the node's factory)")
	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.schematicID, schematicIDFlagName, "",
		"Image Factory schematic ID to use for the installer image (defaults to the node's current schematic)")
	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.talosVersion, talosVersionFlagName, version.Trim(version.Tag),
		"Talos version to upgrade to (defaults to talosctl version)")
	upgradeCmd.Flags().BoolVar(&upgradeCmdFlags.secureBoot, secureBootFlagName, false,
		"use the SecureBoot installer image (defaults to the node's current SecureBoot state)")

	// --image is a complete image reference, it can't be combined with the flags which build one.
	for _, name := range imageComponentFlagNames {
		upgradeCmd.MarkFlagsMutuallyExclusive(upgradeImageFlagName, name)
	}

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
	helpers.KubernetesEndpointVar(upgradeCmd.Flags(), &upgradeCmdFlags.k8sEndpoint, "k8s-endpoint", "", "use endpoint instead of kubeconfig default when draining the Kubernetes node")

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
