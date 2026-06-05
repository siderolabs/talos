// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var restartCmdFlags struct {
	kubernetesNamespaceFlag
}

// restartCmd represents the restart command.
var restartCmd = &cobra.Command{
	Use:   "restart <id>",
	Short: "Restart a process",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
		}

		return getContainersFromNode(cmd.Context(), &restartCmdFlags), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &restartCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		var (
			namespace string
			driver    common.ContainerDriver
		)

		if restartCmdFlags.kubernetes {
			namespace = constants.K8sContainerdNamespace
			driver = common.ContainerDriver_CRI
		} else {
			namespace = constants.SystemContainerdNamespace
			driver = common.ContainerDriver_CONTAINERD
		}

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (struct{}, error) {
				return struct{}{}, c.Restart(ctx, namespace, driver, args[0])
			},
		)

		var errs error

		for resp := range responseChan {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error restarting process on node %s: %w", resp.Node, resp.Err))
			}
		}

		return errs
	},
}

func init() {
	restartCmd.Flags().BoolVarP(&restartCmdFlags.kubernetes, "kubernetes", "k", false, "use the k8s.io containerd namespace")

	restartCmd.Flags().Bool("use-cri", false, "use the CRI driver")
	restartCmd.Flags().MarkHidden("use-cri") //nolint:errcheck

	addCommand(restartCmd)
}
