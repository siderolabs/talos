// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/rotate/pki/talos"
)

var rotateCACmdFlags struct {
	clusterState  clusterNodes
	forceEndpoint string
	output        string
	withExamples  bool
	withDocs      bool
	dryRun        bool
}

// rotateCACmd represents the rotate-ca command.
var rotateCACmd = &cobra.Command{
	Use:   "rotate-ca",
	Short: "Rotate cluster CAs (Talos and Kubernetes APIs).",
	Long:  `The command starts by generating new CAs, and gracefully applying it to the cluster.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := rotateCACmdFlags.clusterState.InitNodeInfos()
		if err != nil {
			return err
		}

		return WithClient(rotateCA)
	},
}

func rotateCA(ctx context.Context, oldClient *client.Client) error {
	commentsFlags := encoder.CommentsDisabled
	if upgradeK8sCmdFlags.withDocs {
		commentsFlags |= encoder.CommentsDocs
	}

	if upgradeK8sCmdFlags.withExamples {
		commentsFlags |= encoder.CommentsExamples
	}

	encoderOpt := encoder.WithComments(commentsFlags)

	clusterInfo, err := buildClusterInfo(rotateCACmdFlags.clusterState)
	if err != nil {
		return err
	}

	oldTalosconfig, err := clientconfig.Open(GlobalArgs.Talosconfig)
	if err != nil {
		return fmt.Errorf("failed to open config file %q: %w", GlobalArgs.Talosconfig, err)
	}

	configContext := oldTalosconfig.Context

	if GlobalArgs.CmdContext != "" {
		configContext = GlobalArgs.CmdContext
	}

	newBundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersionCurrent)
	if err != nil {
		return fmt.Errorf("error generating new Talos CA: %w", err)
	}

	options := talos.Options{
		DryRun: rotateCACmdFlags.dryRun,

		CurrentClient: oldClient,
		ClusterInfo:   clusterInfo,

		ContextName: configContext,
		Endpoints:   oldClient.GetEndpoints(),

		NewTalosCA: newBundle.Certs.OS,

		EncoderOption: encoderOpt,

		Printf: func(format string, args ...any) { fmt.Printf(format, args...) },
	}

	newTalosconfig, err := talos.Rotate(ctx, options)
	if err != nil {
		return err
	}

	if rotateCACmdFlags.dryRun {
		fmt.Println("> Dry-run mode enabled, no changes were made to the cluster, re-run with `--dry-run=false` to apply the changes.")

		return nil
	}

	fmt.Printf("> Writing new talosconfig to %q\n", rotateCACmdFlags.output)

	return newTalosconfig.Save(rotateCACmdFlags.output)
}

func init() {
	addCommand(rotateCACmd)
	rotateCACmd.Flags().StringVar(&rotateCACmdFlags.clusterState.InitNode, "init-node", "", "specify IPs of init node")
	rotateCACmd.Flags().StringSliceVar(&rotateCACmdFlags.clusterState.ControlPlaneNodes, "control-plane-nodes", nil, "specify IPs of control plane nodes")
	rotateCACmd.Flags().StringSliceVar(&rotateCACmdFlags.clusterState.WorkerNodes, "worker-nodes", nil, "specify IPs of worker nodes")
	rotateCACmd.Flags().StringVar(&rotateCACmdFlags.forceEndpoint, "k8s-endpoint", "", "use endpoint instead of kubeconfig default")
	rotateCACmd.Flags().BoolVarP(&rotateCACmdFlags.withExamples, "with-examples", "", true, "patch all machine configs with the commented examples")
	rotateCACmd.Flags().BoolVarP(&rotateCACmdFlags.withDocs, "with-docs", "", true, "patch all machine configs adding the documentation for each field")
	rotateCACmd.Flags().StringVarP(&rotateCACmdFlags.output, "output", "o", "talosconfig", "path to the output new `talosconfig`")
	rotateCACmd.Flags().BoolVarP(&rotateCACmdFlags.dryRun, "dry-run", "", true, "dry-run mode (no changes to the cluster)")
}
