// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/action"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

var upgradeCmdFlags struct {
	trackableActionCmdFlags

	factory      string
	schematic    string
	version      string
	secureBoot   bool
	platform     string

	upgradeImage string

	rebootMode string
	preserve   bool
	stage      bool
	force      bool
	insecure   bool
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

		if upgradeCmdFlags.wait && upgradeCmdFlags.insecure {
			return errors.New("cannot use --wait and --insecure together")
		}

		rebootModeStr := strings.ToUpper(upgradeCmdFlags.rebootMode)

		rebootMode, rebootModeOk := machine.UpgradeRequest_RebootMode_value[rebootModeStr]
		if !rebootModeOk {
			return fmt.Errorf("invalid reboot mode: %s", upgradeCmdFlags.rebootMode)
		}

		var imageURL string
		var err error

		if upgradeCmdFlags.upgradeImage != "" {
			// Legacy path: use provided image
			imageURL = upgradeCmdFlags.upgradeImage
			if upgradeCmdFlags.factory != "factory.talos.dev" || upgradeCmdFlags.schematic != "" ||
				upgradeCmdFlags.version != "" || cmd.Flags().Changed("secure-boot") || upgradeCmdFlags.platform != "" {
				cli.Warning("--image flag is set, ignoring component flags (--factory, --schematic, --version, --secure-boot, --platform)")
			}
		} else {
			imageURL, err = buildUpgradeImage(cmd.Context(), cmd)
			if err != nil {
				return fmt.Errorf("failed to build upgrade image: %w", err)
			}
		}

		opts := []client.UpgradeOption{
			client.WithUpgradeImage(imageURL),
			client.WithUpgradeRebootMode(machine.UpgradeRequest_RebootMode(rebootMode)),
			client.WithUpgradePreserve(upgradeCmdFlags.preserve),
			client.WithUpgradeStage(upgradeCmdFlags.stage),
			client.WithUpgradeForce(upgradeCmdFlags.force),
		}

		if !upgradeCmdFlags.wait {
			return runUpgradeNoWait(opts)
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
		).Run()
	},
}

func runUpgradeNoWait(opts []client.UpgradeOption) error {
	upgradeFn := func(ctx context.Context, c *client.Client) error {
		if err := helpers.ClientVersionCheck(ctx, c); err != nil {
			return err
		}

		var remotePeer peer.Peer

		opts = append(opts, client.WithUpgradeGRPCCallOptions(grpc.Peer(&remotePeer)))

		// TODO: See if we can validate version and prevent starting upgrades to an unknown version
		resp, err := c.UpgradeWithOptions(ctx, opts...)
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

	if upgradeCmdFlags.insecure {
		return WithClientMaintenance(nil, upgradeFn)
	}

	return WithClient(upgradeFn)
}

func upgradeGetActorID(ctx context.Context, c *client.Client, opts []client.UpgradeOption) (string, error) {
	resp, err := c.UpgradeWithOptions(ctx, opts...)
	if err != nil {
		return "", err
	}

	if len(resp.GetMessages()) == 0 {
		return "", errors.New("no messages returned from action run")
	}

	return resp.GetMessages()[0].GetActorId(), nil
}

// buildUpgradeImage constructs the Image Factory URL from component flags.
func buildUpgradeImage(ctx context.Context, cmd *cobra.Command) (string, error) {
	var machineCtx *helpers.MachineContext
	var err error

	err = WithClientNoNodes(func(ctx context.Context, c *client.Client) error {
		machineCtx, err = helpers.QueryMachineContext(ctx, c)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("failed to query machine context: %w", err)
	}

	factory := upgradeCmdFlags.factory

	schematic := upgradeCmdFlags.schematic
	if schematic == "" {
		schematic = machineCtx.Schematic
	}

	targetVersion := upgradeCmdFlags.version
	if targetVersion == "" {
		targetVersion = version.Tag
		fmt.Printf("No version specified, using talosctl version: %s\n", targetVersion)
	}

	secureBoot := machineCtx.SecureBoot
	if cmd.Flags().Changed("secure-boot") {
		secureBoot = upgradeCmdFlags.secureBoot
	}

	platform := upgradeCmdFlags.platform
	if platform == "" {
		platform = machineCtx.Platform
	}

	if err := helpers.ValidateUpgradeTransition(machineCtx, schematic, secureBoot, platform, targetVersion); err != nil {
		if !upgradeCmdFlags.force {
			return "", err
		}

		cli.Warning("Validation failed but continuing due to --force: %v", err)
	}

	// Build Image Factory URL
	imageURL := helpers.BuildImageFactoryURL(factory, schematic, targetVersion, platform, secureBoot)

	// Print upgrade plan
	fmt.Printf("\nUpgrade Plan:\n")
	fmt.Printf("Version:    %s → %s\n", machineCtx.Version, targetVersion)
	fmt.Printf("Schematic:  %s %s\n", schematic,
		helpers.Ternary(schematic == machineCtx.Schematic, "(unchanged)", "(CHANGED)"))
	fmt.Printf("SecureBoot: %v %s\n", secureBoot,
		helpers.Ternary(secureBoot == machineCtx.SecureBoot, "(unchanged)", "(CHANGED)"))
	fmt.Printf("Platform:   %s %s\n", platform,
		helpers.Ternary(platform == machineCtx.Platform, "(unchanged)", "(CHANGED)"))
	fmt.Printf("Factory:    %s\n", factory)
	fmt.Printf("Image:      %s\n\n", imageURL)

	return imageURL, nil
}

func init() {
	rebootModes := maps.Keys(machine.UpgradeRequest_RebootMode_value)
	slices.SortFunc(rebootModes, func(a, b string) int {
		return cmp.Compare(machine.UpgradeRequest_RebootMode_value[a], machine.UpgradeRequest_RebootMode_value[b])
	})

	rebootModes = xslices.Map(rebootModes, strings.ToLower)

	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.factory, "factory", "factory.talos.dev",
		"Image Factory host to use for building the installer image")
	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.schematic, "schematic", "",
		"schematic ID for system extensions (defaults to current machine schematic)")
	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.version, "version", "",
		"Talos version to upgrade to (defaults to talosctl binary version)")
	upgradeCmd.Flags().BoolVar(&upgradeCmdFlags.secureBoot, "secure-boot", false,
		"enable secure boot (if not specified, defaults to current machine setting)")
	upgradeCmd.Flags().StringVar(&upgradeCmdFlags.platform, "platform", "",
		"platform type (defaults to current machine platform)")

	upgradeCmd.Flags().StringVarP(&upgradeCmdFlags.upgradeImage, "image", "i", "",
		"the container image to use for performing the install (legacy, use component flags instead)")

	if err := upgradeCmd.Flags().MarkHidden("image"); err != nil {
		panic(err)
	}

	upgradeCmd.Flags().StringVarP(&upgradeCmdFlags.rebootMode, "reboot-mode", "m", strings.ToLower(machine.UpgradeRequest_DEFAULT.String()),
		fmt.Sprintf("select the reboot mode during upgrade. Mode %q bypasses kexec. Valid values are: %q.",
			strings.ToLower(machine.UpgradeRequest_POWERCYCLE.String()),
			rebootModes))
	upgradeCmd.Flags().BoolVarP(&upgradeCmdFlags.preserve, "preserve", "p", false, "preserve data")
	upgradeCmd.Flags().BoolVarP(&upgradeCmdFlags.stage, "stage", "s", false, "stage the upgrade to perform it after a reboot")
	upgradeCmd.Flags().BoolVarP(&upgradeCmdFlags.force, "force", "f", false, "force the upgrade (skip checks on etcd health and members, might lead to data loss)")
	upgradeCmd.Flags().BoolVar(&upgradeCmdFlags.insecure, "insecure", false, "upgrade using the insecure (encrypted with no auth) maintenance service")
	upgradeCmdFlags.addTrackActionFlags(upgradeCmd)

	if err := upgradeCmd.Flags().MarkHidden("preserve"); err != nil {
		panic(err)
	}

	addCommand(upgradeCmd)
}
