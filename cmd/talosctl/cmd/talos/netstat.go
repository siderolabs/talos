// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var netstatCmdFlags struct {
	verbose   bool
	extend    bool
	pid       bool
	timers    bool
	listening bool
	all       bool
	pods      bool
	tcp       bool
	udp       bool
	udplite   bool
	raw       bool
	ipv4      bool
	ipv6      bool
}

// netstatPrinter renders netstat responses, attaching the node explicitly and
// resolving pod names from the per-node network namespace map.
type netstatPrinter struct {
	w             *tabwriter.Writer
	nodeNetNSPods map[string]map[string]string
	headerWritten bool
}

// netstatCmd represents the netstat command.
var netstatCmd = &cobra.Command{
	Use:     "netstat",
	Aliases: []string{"ss"},
	Short:   "Show network connections and sockets",
	Long: `Show network connections and sockets.

You can pass an optional argument to view a specific pod's connections.
To do this, format the argument as "namespace/pod".
Note that only pods with a pod network namespace are allowed.
If you don't pass an argument, the command will show host connections.`,
	Args: cobra.MaximumNArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
		}

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &netstatCmdFlags)
		if err != nil {
			cobra.CompError(fmt.Sprintf("error creating client factory: %v", err))

			return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
		}

		defer clientFactory.Close() //nolint:errcheck

		nodeNetNSPods, err := getPodNetNs(ctx, clientFactory)
		if err != nil {
			cobra.CompError(fmt.Sprintf("error getting pod network namespaces: %v", err))

			return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
		}

		var podList []string

		for _, netNsPods := range nodeNetNSPods {
			for _, podName := range netNsPods {
				podList = append(podList, podName)
			}
		}

		return podList, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if netstatCmdFlags.pods && len(args) > 0 {
			return errors.New("cannot use --pods and specify a pod")
		}

		req := netstatFlagsToRequest()

		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &netstatCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		findThePod := len(args) > 0

		var nodeNetNSPods map[string]map[string]string

		if findThePod || netstatCmdFlags.pods {
			nodeNetNSPods, err = getPodNetNs(ctx, clientFactory)
			if err != nil {
				return err
			}
		}

		printer := &netstatPrinter{
			w:             tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0),
			nodeNetNSPods: nodeNetNSPods,
		}

		// Finding a specific pod changes the flow: instead of multiplexing across all
		// nodes, we locate the single node hosting the pod and query only that node.
		if findThePod {
			foundNode, foundNetNs := findPodNetNs(nodeNetNSPods, args[0])

			if foundNetNs == "" {
				cli.Fatalf("pod %s not found", args[0])
			}

			req.Netns.Netns = []string{foundNetNs}
			req.Netns.Hostnetwork = false

			ctx, c, err := clientFactory.BuildClient(ctx, foundNode)
			if err != nil {
				return err
			}

			response, err := c.Netstat(ctx, req)
			if err != nil {
				if response == nil {
					return err
				}

				cli.Warning("%s", err)
			}

			printer.printResponse(foundNode, response)

			return printer.flush()
		}

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (*machine.NetstatResponse, error) {
				return c.Netstat(ctx, req)
			},
		)

		var errs error

		for resp := range responseChan {
			if resp.Err != nil {
				errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

				continue
			}

			printer.printResponse(resp.Node, resp.Payload)
		}

		return errors.Join(errs, printer.flush())
	},
}

//nolint:gocyclo
func netstatFlagsToRequest() *machine.NetstatRequest {
	req := machine.NetstatRequest{
		Feature: &machine.NetstatRequest_Feature{
			Pid: netstatCmdFlags.pid,
		},
		L4Proto: &machine.NetstatRequest_L4Proto{
			Tcp:      netstatCmdFlags.tcp,
			Tcp6:     netstatCmdFlags.tcp,
			Udp:      netstatCmdFlags.udp,
			Udp6:     netstatCmdFlags.udp,
			Udplite:  netstatCmdFlags.udplite,
			Udplite6: netstatCmdFlags.udplite,
			Raw:      netstatCmdFlags.raw,
			Raw6:     netstatCmdFlags.raw,
		},
		Netns: &machine.NetstatRequest_NetNS{
			Allnetns:    netstatCmdFlags.pods,
			Hostnetwork: true,
		},
	}

	switch {
	case netstatCmdFlags.all:
		req.Filter = machine.NetstatRequest_ALL
	case netstatCmdFlags.listening:
		req.Filter = machine.NetstatRequest_LISTENING
	default:
		req.Filter = machine.NetstatRequest_CONNECTED
	}

	if netstatCmdFlags.verbose {
		req.L4Proto.Tcp = true
		req.L4Proto.Tcp6 = true
		req.L4Proto.Udp = true
		req.L4Proto.Udp6 = true
		req.L4Proto.Udplite = true
		req.L4Proto.Udplite6 = true
		req.L4Proto.Raw = true
		req.L4Proto.Raw6 = true
	}

	if !req.L4Proto.Tcp && !req.L4Proto.Tcp6 && !req.L4Proto.Udp && !req.L4Proto.Udp6 && !req.L4Proto.Udplite && !req.L4Proto.Udplite6 && !req.L4Proto.Raw && !req.L4Proto.Raw6 {
		req.L4Proto.Tcp = true
		req.L4Proto.Tcp6 = true
		req.L4Proto.Udp = true
		req.L4Proto.Udp6 = true
	}

	if netstatCmdFlags.ipv4 && !netstatCmdFlags.ipv6 {
		req.L4Proto.Tcp6 = false
		req.L4Proto.Udp6 = false
		req.L4Proto.Udplite6 = false
		req.L4Proto.Raw6 = false
	}

	if netstatCmdFlags.ipv6 && !netstatCmdFlags.ipv4 {
		req.L4Proto.Tcp = false
		req.L4Proto.Udp = false
		req.L4Proto.Udplite = false
		req.L4Proto.Raw = false
	}

	return &req
}

// getPodNetNs builds a per-node map of network namespace -> pod ID across all nodes.
func getPodNetNs(ctx context.Context, clientFactory *global.ClientFactory) (map[string]map[string]string, error) {
	responseChan := multiplex.UnaryViaFactory(
		ctx, clientFactory,
		func(ctx context.Context, c *client.Client) (*machine.ContainersResponse, error) {
			return c.Containers(ctx, constants.K8sContainerdNamespace, common.ContainerDriver_CRI)
		},
	)

	nodeNetNSPods := map[string]map[string]string{}

	var errs error

	for resp := range responseChan {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("error getting containers from node %s: %w", resp.Node, resp.Err))

			continue
		}

		for _, msg := range resp.Payload.Messages {
			for _, p := range msg.Containers {
				if p.NetworkNamespace == "" {
					continue
				}

				if p.Pid == 0 {
					continue
				}

				if p.Id != p.PodId {
					continue
				}

				if nodeNetNSPods[resp.Node] == nil {
					nodeNetNSPods[resp.Node] = make(map[string]string)
				}

				nodeNetNSPods[resp.Node][p.NetworkNamespace] = p.Id
			}
		}
	}

	return nodeNetNSPods, errs
}

func findPodNetNs(nodeNetNSPods map[string]map[string]string, findNamespaceAndPod string) (string, string) {
	var foundNetNs, foundNode string

	for node, netNSPods := range nodeNetNSPods {
		for NetNs, podName := range netNSPods {
			if podName == strings.ToLower(findNamespaceAndPod) {
				foundNetNs = NetNs
				foundNode = node

				break
			}
		}
	}

	return foundNode, foundNetNs
}

//nolint:gocyclo
func (p *netstatPrinter) printResponse(node string, response *machine.NetstatResponse) {
	for _, message := range response.Messages {
		if len(message.Connectrecord) == 0 {
			continue
		}

		for _, record := range message.Connectrecord {
			if !p.headerWritten {
				fmt.Fprintln(p.w, "NODE\t"+netstatSummaryLabels())

				p.headerWritten = true
			}

			args := []any{node}

			state := ""
			if record.State != 7 {
				state = record.State.String()
			}

			args = append(args, []any{
				record.L4Proto,
				strconv.FormatUint(record.Rxqueue, 10),
				strconv.FormatUint(record.Txqueue, 10),
				fmt.Sprintf("%s:%d", record.Localip, record.Localport),
				fmt.Sprintf("%s:%s", record.Remoteip, wildcardIfZero(record.Remoteport)),
				state,
			}...)

			if netstatCmdFlags.extend {
				args = append(args, []any{
					strconv.FormatUint(uint64(record.Uid), 10),
					strconv.FormatUint(record.Inode, 10),
				}...)
			}

			if netstatCmdFlags.pid {
				if record.Process.Pid != 0 {
					args = append(args, []any{
						fmt.Sprintf("%d/%s", record.Process.Pid, record.Process.Name),
					}...)
				} else {
					args = append(args, []any{
						"-",
					}...)
				}
			}

			if netstatCmdFlags.pods {
				if record.Netns == "" || p.nodeNetNSPods[node] == nil {
					args = append(args, []any{
						"-",
					}...)
				} else {
					args = append(args, []any{
						p.nodeNetNSPods[node][record.Netns],
					}...)
				}
			}

			if netstatCmdFlags.timers {
				timerwhen := strconv.FormatFloat(float64(record.Timerwhen)/100, 'f', 2, 64)

				args = append(args, []any{
					fmt.Sprintf("%s (%s/%d/%d)", strings.ToLower(record.Tr.String()), timerwhen, record.Retrnsmt, record.Timeout),
				}...)
			}

			pattern := strings.Repeat("%s\t", len(args))
			pattern = strings.TrimSpace(pattern) + "\n"

			fmt.Fprintf(p.w, pattern, args...)
		}
	}
}

func (p *netstatPrinter) flush() error {
	return p.w.Flush()
}

func netstatSummaryLabels() (labels string) {
	labels = strings.Join(
		[]string{
			"Proto",
			"Recv-Q",
			"Send-Q",
			"Local Address",
			"Foreign Address",
			"State",
		}, "\t",
	)

	if netstatCmdFlags.extend {
		labels += "\t" + strings.Join(
			[]string{
				"Uid",
				"Inode",
			}, "\t",
		)
	}

	if netstatCmdFlags.pid {
		labels += "\t" + "PID/Program name"
	}

	if netstatCmdFlags.pods {
		labels += "\t" + "Pod"
	}

	if netstatCmdFlags.timers {
		labels += "\t" + "Timer"
	}

	return labels
}

func wildcardIfZero(num uint32) string {
	if num == 0 {
		return "*"
	}

	return strconv.FormatUint(uint64(num), 10)
}

func init() {
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.verbose, "verbose", "v", false, "display sockets of all supported transport protocols")
	// extend is normally -e but cannot be used as this is endpoint in talosctl
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.extend, "extend", "x", false, "show detailed socket information")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.pid, "programs", "p", false, "show process using socket")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.timers, "timers", "o", false, "display timers")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.listening, "listening", "l", false, "display listening server sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.all, "all", "a", false, "display all sockets states (default: connected)")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.pods, "pods", "k", false, "show sockets used by Kubernetes pods")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.tcp, "tcp", "t", false, "display only TCP sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.udp, "udp", "u", false, "display only UDP sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.udplite, "udplite", "U", false, "display only UDPLite sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.raw, "raw", "w", false, "display only RAW sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.ipv4, "ipv4", "4", false, "display only ipv4 sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.ipv6, "ipv6", "6", false, "display only ipv6 sockets")

	addCommand(netstatCmd)
}
