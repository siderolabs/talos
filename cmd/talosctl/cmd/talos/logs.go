// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/siderolabs/gen/xslices"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/safeout"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

var logsCmdFlags struct {
	global.InsecureFlags
	containerNamespaceFlag

	follow bool
	tail   int32
	kind   string
}

var logsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "Retrieve service, container, or VM serial logs",
	Long: "Select container, VM, or service logs with --kind. Omitting --kind preserves legacy service/container routing. " +
		"VM serial history uses --kind=vm with a raw VM name and requires Admin access and one target node. " +
		"Tail reads only the active file; follow drains it and follows replacements after rotation. " +
		"VM output is safely rendered as a byte stream, including unterminated prompts; it does not attach to the live console.",
	Args: cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
		}

		if _, err := resolveLogKind(cmd); err != nil {
			return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
		}

		if logsCmdFlags.kind == "vm" {
			return completeResourceID(cmd.Context(), hypervisor.VirtualMachineSpecType, hypervisor.NamespaceName)
		}

		if logsCmdFlags.kind == "service" {
			return getServiceFromNode(cmd.Context(), &logsCmdFlags), cobra.ShellCompDirectiveNoFileComp
		}

		if logsCmdFlags.namespace == constants.TalosContainersContainerdNamespace {
			return getTalosContainerLogs(cmd.Context(), &logsCmdFlags), cobra.ShellCompDirectiveNoFileComp
		}

		if logsCmdFlags.kubernetes {
			return getContainersFromNode(cmd.Context(), &logsCmdFlags), cobra.ShellCompDirectiveNoFileComp
		}

		if logsCmdFlags.kind == "container" {
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

		kind, err := resolveLogKind(cmd)
		if err != nil {
			return err
		}

		clientFactory, err := NewClientFactory(ctx, &logsCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		namespace, driver, err := logsCmdFlags.resolveContainerNamespace()
		if err != nil {
			return err
		}

		if kind == machine.LogKind_LOG_KIND_VM {
			return streamVMLogs(ctx, clientFactory, args[0])
		}

		responseChan := multiplex.StreamingViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (machine.MachineService_LogsClient, error) {
				return c.LogsWithKind(ctx, namespace, driver, args[0], logsCmdFlags.follow, logsCmdFlags.tail, kind)
			},
		)

		// logs arrive as arbitrary byte chunks per node, so buffer each node's bytes
		// until a newline is seen and emit complete lines: this keeps the output
		// line-aligned even when interleaving logs from multiple nodes.
		lineBuffers := map[string][]byte{}

		emit := func(node string, line []byte) error {
			_, err := safeout.Printf("%s: %s\n", node, line)

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

func resolveLogKind(cmd *cobra.Command) (machine.LogKind, error) {
	var kind machine.LogKind

	switch logsCmdFlags.kind {
	case "":
		if cmd.Flags().Changed("kind") {
			return kind, errors.New("--kind must be container, vm, or service")
		}
	case "container":
		kind = machine.LogKind_LOG_KIND_CONTAINER
	case "vm":
		kind = machine.LogKind_LOG_KIND_VM
	case "service":
		kind = machine.LogKind_LOG_KIND_SERVICE
	default:
		return kind, fmt.Errorf("invalid --kind %q: must be container, vm, or service", logsCmdFlags.kind)
	}

	namespace, _, err := logsCmdFlags.resolveContainerNamespace()
	if err != nil {
		return kind, err
	}

	if (kind == machine.LogKind_LOG_KIND_VM || kind == machine.LogKind_LOG_KIND_SERVICE) && namespace != constants.SystemContainerdNamespace {
		return kind, fmt.Errorf("--kind=%s requires --namespace=system and cannot be combined with --kubernetes", logsCmdFlags.kind)
	}

	return kind, nil
}

// streamVMLogs uses a single node's safe byte stream so unterminated guest prompts
// are visible immediately and a newline-free guest cannot grow the line buffer.
func streamVMLogs(ctx context.Context, factory *global.ClientFactory, id string) error {
	ctx, c, _, err := factory.BuildClientEnforceSingleNode(ctx, "talosctl logs --kind=vm <name>")
	if err != nil {
		return err
	}

	stream, err := c.LogsWithKind(ctx, constants.SystemContainerdNamespace, common.ContainerDriver_CONTAINERD, id, logsCmdFlags.follow, logsCmdFlags.tail, machine.LogKind_LOG_KIND_VM)
	if err != nil {
		return err
	}

	for {
		data, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) || client.StatusCode(recvErr) == codes.Canceled {
			return nil
		}

		if recvErr != nil {
			return recvErr
		}

		if _, err = safeout.Stdout().Write(data.Bytes); err != nil {
			return err
		}
	}
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

// getTalosContainerLogs suggests the containers declared via ContainerConfig which have logs.
//
// A container that has never started has no buffer, and so does not appear.
func getTalosContainerLogs(ctx context.Context, flags any) []string {
	return stripTalosContainerLogPrefix(getLogsContainers(ctx, flags))
}

// stripTalosContainerLogPrefix keeps only the ids carrying the taloscontainers log prefix, with the
// prefix removed.
//
// The registered identifiers carry the namespace prefix, but the command takes the container name, so
// the prefix is stripped back off.
func stripTalosContainerLogPrefix(ids []string) []string {
	var result []string

	for _, id := range ids {
		if name, ok := strings.CutPrefix(id, constants.TalosContainersLogPrefix); ok {
			result = append(result, name)
		}
	}

	return result
}

func init() {
	addContainerNamespaceFlags(logsCmd, &logsCmdFlags.containerNamespaceFlag)
	logsCmd.Flags().StringVar(&logsCmdFlags.kind, "kind", "", "log source kind: container, vm, or service (default: legacy routing)")
	cli.Should(logsCmd.RegisterFlagCompletionFunc("kind", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"container", "vm", "service"}, cobra.ShellCompDirectiveNoFileComp
	}))
	logsCmd.Flags().BoolVarP(&logsCmdFlags.follow, "follow", "f", false, "specify if the logs should be streamed")
	logsCmd.Flags().Int32VarP(&logsCmdFlags.tail, "tail", "", -1, "lines of log file to display (default is to show from the beginning)")

	logsCmd.Flags().Bool("use-cri", false, "use the CRI driver")
	logsCmd.Flags().MarkHidden("use-cri") //nolint:errcheck

	logsCmdFlags.InsecureFlags.AddFlags(logsCmd)

	addCommand(logsCmd)
}
