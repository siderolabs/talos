// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/siderolabs/gen/xslices"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var logsCmdFlags struct {
	global.InsecureFlags
	kubernetesNamespaceFlag

	follow bool
	tail   int32
}

var logsCmd = &cobra.Command{
	Use:   "logs <service name>",
	Short: "Retrieve logs for a service",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
		}

		if logsCmdFlags.kubernetes {
			return getContainersFromNode(cmd.Context(), &logsCmdFlags), cobra.ShellCompDirectiveNoFileComp
		}

		return mergeSuggestions(
			getServiceFromNode(cmd.Context(), &logsCmdFlags),
			getContainersFromNode(cmd.Context(), &logsCmdFlags),
			getLogsContainers(cmd.Context(), &logsCmdFlags),
		), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &logsCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		var (
			namespace string
			driver    common.ContainerDriver
		)

		if logsCmdFlags.kubernetes {
			namespace = constants.K8sContainerdNamespace
			driver = common.ContainerDriver_CRI
		} else {
			namespace = constants.SystemContainerdNamespace
			driver = common.ContainerDriver_CONTAINERD
		}

		responseChan := multiplex.StreamingViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (machine.MachineService_LogsClient, error) {
				return c.Logs(ctx, namespace, driver, args[0], logsCmdFlags.follow, logsCmdFlags.tail)
			},
		)

		// logs arrive as arbitrary byte chunks per node, so buffer each node's bytes
		// until a newline is seen and emit complete lines: this keeps the output
		// line-aligned even when interleaving logs from multiple nodes.
		lineBuffers := map[string][]byte{}

		emit := func(node string, line []byte) error {
			_, err := fmt.Printf("%s: %s\n", node, line)

			return err
		}

		var errs error

		for resp := range responseChan {
			if resp.Err != nil {
				// the stream is canceled on Ctrl-C (or context cancellation), which is a normal termination
				if client.StatusCode(resp.Err) == codes.Canceled {
					continue
				}

				errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

				continue
			}

			buf := append(lineBuffers[resp.Node], resp.Payload.Bytes...)

			for {
				idx := bytes.IndexByte(buf, '\n')
				if idx < 0 {
					break
				}

				if err := emit(resp.Node, buf[:idx]); err != nil {
					return err
				}

				buf = buf[idx+1:]
			}

			lineBuffers[resp.Node] = buf
		}

		// flush trailing bytes which were not terminated by a newline
		for _, node := range slices.Sorted(maps.Keys(lineBuffers)) {
			if buf := lineBuffers[node]; len(buf) > 0 {
				if err := emit(node, buf); err != nil {
					return err
				}
			}
		}

		return errs
	},
}

func getLogsContainers(ctx context.Context, flags any) []string {
	clientFactory, err := NewClientFactory(ctx, flags)
	if err != nil {
		cobra.CompError(fmt.Sprintf("error creating client factory: %v", err))

		return nil
	}

	defer clientFactory.Close() //nolint:errcheck

	responseChan := multiplex.UnaryViaFactory(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (*machine.LogsContainersResponse, error) {
			return c.LogsContainers(ctx)
		},
	)

	var result []string

	for resp := range responseChan {
		if resp.Err != nil {
			cobra.CompError(fmt.Sprintf("error from node %s: %v", resp.Node, resp.Err))

			continue
		}

		result = append(result, xslices.FlatMap(resp.Payload.Messages, func(lc *machine.LogsContainer) []string { return lc.Ids })...)
	}

	return result
}

func init() {
	logsCmd.Flags().BoolVarP(&logsCmdFlags.kubernetes, "kubernetes", "k", false, "use the k8s.io containerd namespace")
	logsCmd.Flags().BoolVarP(&logsCmdFlags.follow, "follow", "f", false, "specify if the logs should be streamed")
	logsCmd.Flags().Int32VarP(&logsCmdFlags.tail, "tail", "", -1, "lines of log file to display (default is to show from the beginning)")

	logsCmd.Flags().Bool("use-cri", false, "use the CRI driver")
	logsCmd.Flags().MarkHidden("use-cri") //nolint:errcheck

	logsCmdFlags.InsecureFlags.AddFlags(logsCmd)

	addCommand(logsCmd)
}
