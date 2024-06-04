// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/rotate/pki/kubernetes"
	"github.com/siderolabs/talos/pkg/rotate/pki/talos"
)

var rotateCACmdFlags struct {
	clusterState     clusterNodes
	forceEndpoint    string
	output           string
	withExamples     bool
	withDocs         bool
	dryRun           bool
	rotateTalos      bool
	rotateKubernetes bool
}

// RotateCACmd represents the rotate-ca command.
var RotateCACmd = &cobra.Command{
	Use:   "rotate-ca",
	Short: "Rotate cluster CAs (Talos and Kubernetes APIs).",
	Long: `The command can rotate both Talos and Kubernetes root CAs (for the API).
By default both CAs are rotated, but you can choose to rotate just one or another.
The command starts by generating new CAs, and gracefully applying it to the cluster.

For Kubernetes, the command only rotates the API server issuing CA, and other Kubernetes
PKI can be rotated by applying machine config changes to the controlplane nodes.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := rotateCACmdFlags.clusterState.InitNodeInfos()
		if err != nil {
			return err
		}

		return WithClient(rotateCA)
	},
}

func rotateCA(ctx context.Context, c *client.Client) error {
	commentsFlags := encoder.CommentsDisabled
	if rotateCACmdFlags.withDocs {
		commentsFlags |= encoder.CommentsDocs
	}

	if rotateCACmdFlags.withExamples {
		commentsFlags |= encoder.CommentsExamples
	}

	encoderOpt := encoder.WithComments(commentsFlags)

	clusterInfo, err := buildClusterInfo(rotateCACmdFlags.clusterState)
	if err != nil {
		return err
	}

	newBundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersionCurrent)
	if err != nil {
		return fmt.Errorf("error generating new Talos CA: %w", err)
	}

	if rotateCACmdFlags.rotateTalos {
		var newTalosconfig *clientconfig.Config

		newTalosconfig, err = rotateTalosCA(ctx, c, encoderOpt, clusterInfo, newBundle)
		if err != nil {
			return fmt.Errorf("error rotating Talos CA: %w", err)
		}

		// re-create client with new Talos PKI
		c, err = client.New(ctx, client.WithConfig(newTalosconfig))
		if err != nil {
			return fmt.Errorf("failed to create new client with rotated Talos CA: %w", err)
		}
	}

	if rotateCACmdFlags.rotateKubernetes {
		if err = rotateKubernetesCA(ctx, c, encoderOpt, clusterInfo, newBundle); err != nil {
			return fmt.Errorf("error rotating Kubernetes CA: %w", err)
		}
	}

	return nil
}

func rotateTalosCA(ctx context.Context, oldClient *client.Client, encoderOpt encoder.Option, clusterInfo cluster.Info, newBundle *secrets.Bundle) (*clientconfig.Config, error) {
	oldTalosconfig, err := clientconfig.Open(GlobalArgs.Talosconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %q: %w", GlobalArgs.Talosconfig, err)
	}

	configContext := oldTalosconfig.Context

	if GlobalArgs.CmdContext != "" {
		configContext = GlobalArgs.CmdContext
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
		return nil, err
	}

	if rotateCACmdFlags.dryRun {
		fmt.Println("> Dry-run mode enabled, no changes were made to the cluster, re-run with `--dry-run=false` to apply the changes.")

		return nil, nil
	}

	fmt.Printf("> Writing new talosconfig to %q\n", rotateCACmdFlags.output)

	return newTalosconfig, newTalosconfig.Save(rotateCACmdFlags.output)
}

func rotateKubernetesCA(ctx context.Context, c *client.Client, encoderOpt encoder.Option, clusterInfo cluster.Info, newBundle *secrets.Bundle) error {
	options := kubernetes.Options{
		DryRun: rotateCACmdFlags.dryRun,

		TalosClient: c,
		ClusterInfo: clusterInfo,

		NewKubernetesCA: newBundle.Certs.K8s,

		EncoderOption: encoderOpt,

		Printf: func(format string, args ...any) { fmt.Printf(format, args...) },
	}

	if err := kubernetes.Rotate(ctx, options); err != nil {
		return err
	}

	if rotateCACmdFlags.dryRun {
		fmt.Println("> Dry-run mode enabled, no changes were made to the cluster, re-run with `--dry-run=false` to apply the changes.")

		return nil
	}

	fmt.Printf("> Kubernetes CA rotation done, new 'kubeconfig' can be fetched with `talosctl kubeconfig`.\n")

	return nil
}

func init() {
	addCommand(RotateCACmd)
	RotateCACmd.Flags().StringVar(&rotateCACmdFlags.clusterState.InitNode, "init-node", "", "specify IPs of init node")
	RotateCACmd.Flags().StringSliceVar(&rotateCACmdFlags.clusterState.ControlPlaneNodes, "control-plane-nodes", nil, "specify IPs of control plane nodes")
	RotateCACmd.Flags().StringSliceVar(&rotateCACmdFlags.clusterState.WorkerNodes, "worker-nodes", nil, "specify IPs of worker nodes")
	RotateCACmd.Flags().StringVar(&rotateCACmdFlags.forceEndpoint, "k8s-endpoint", "", "use endpoint instead of kubeconfig default")
	RotateCACmd.Flags().BoolVarP(&rotateCACmdFlags.withExamples, "with-examples", "", true, "patch all machine configs with the commented examples")
	RotateCACmd.Flags().BoolVarP(&rotateCACmdFlags.withDocs, "with-docs", "", true, "patch all machine configs adding the documentation for each field")
	RotateCACmd.Flags().StringVarP(&rotateCACmdFlags.output, "output", "o", "talosconfig", "path to the output new `talosconfig`")
	RotateCACmd.Flags().BoolVarP(&rotateCACmdFlags.dryRun, "dry-run", "", true, "dry-run mode (no changes to the cluster)")
	RotateCACmd.Flags().BoolVarP(&rotateCACmdFlags.rotateTalos, "talos", "", true, "rotate Talos API CA")
	RotateCACmd.Flags().BoolVarP(&rotateCACmdFlags.rotateKubernetes, "kubernetes", "", true, "rotate Kubernetes API CA")
}
