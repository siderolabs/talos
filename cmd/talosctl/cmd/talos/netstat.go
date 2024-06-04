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

	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
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

type netstat struct {
	client        *client.Client
	NodeNetNSPods map[string]map[string]string
}

// NetstatCmd represents the netstat command.
var NetstatCmd = &cobra.Command{
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

		var podList []string

		if WithClient(func(ctx context.Context, c *client.Client) error {
			n := netstat{
				NodeNetNSPods: make(map[string]map[string]string),
				client:        c,
			}

			err := n.getPodNetNsFromNode(ctx)
			if err != nil {
				return err
			}

			for _, netNsPods := range n.NodeNetNSPods {
				for _, podName := range netNsPods {
					podList = append(podList, podName)
				}
			}

			return nil
		}) != nil {
			return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
		}

		return podList, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		req := netstatFlagsToRequest()

		return WithClient(func(ctx context.Context, c *client.Client) (err error) {
			if netstatCmdFlags.pods && len(args) > 0 {
				return errors.New("cannot use --pods and specify a pod")
			}

			findThePod := len(args) > 0

			n := netstat{
				client: c,
			}

			n.NodeNetNSPods = make(map[string]map[string]string)

			if findThePod || netstatCmdFlags.pods {
				err = n.getPodNetNsFromNode(ctx)
				if err != nil {
					return err
				}
			}

			if findThePod {
				var foundNode, foundNetNs string

				foundNode, foundNetNs = n.findPodNetNs(args[0])

				if foundNetNs == "" {
					cli.Fatalf("pod %s not found", args[0])
				}

				ctx = client.WithNode(ctx, foundNode)

				req.Netns.Netns = []string{foundNetNs}
				req.Netns.Hostnetwork = false
			}

			response, err := c.Netstat(ctx, req)
			if err != nil {
				if response == nil {
					return err
				}

				cli.Warning("%s", err)
			}

			err = n.printNetstat(response)

			return err
		})
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

func (n *netstat) getPodNetNsFromNode(ctx context.Context) (err error) {
	resp, err := n.client.Containers(ctx, constants.K8sContainerdNamespace, common.ContainerDriver_CRI)
	if err != nil {
		cli.Warning("error getting containers: %v", err)

		return err
	}

	for _, msg := range resp.Messages {
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

			if n.NodeNetNSPods[msg.Metadata.Hostname] == nil {
				n.NodeNetNSPods[msg.Metadata.Hostname] = make(map[string]string)
			}

			n.NodeNetNSPods[msg.Metadata.Hostname][p.NetworkNamespace] = p.Id
		}
	}

	return nil
}

func (n *netstat) findPodNetNs(findNamespaceAndPod string) (string, string) {
	var foundNetNs, foundNode string

	for node, netNSPods := range n.NodeNetNSPods {
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
func (n *netstat) printNetstat(response *machine.NetstatResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	node := ""

	for i, message := range response.Messages {
		if message.Metadata != nil && message.Metadata.Hostname != "" {
			node = message.Metadata.Hostname
		}

		if len(message.Connectrecord) == 0 {
			continue
		}

		for j, record := range message.Connectrecord {
			if i == 0 && j == 0 {
				labels := netstatSummaryLabels()

				if node != "" {
					fmt.Fprintln(w, "NODE\t"+labels)
				} else {
					fmt.Fprintln(w, labels)
				}
			}

			args := []interface{}{}

			if node != "" {
				args = append(args, node)
			}

			state := ""
			if record.State != 7 {
				state = record.State.String()
			}

			args = append(args, []interface{}{
				record.L4Proto,
				strconv.FormatUint(record.Rxqueue, 10),
				strconv.FormatUint(record.Txqueue, 10),
				fmt.Sprintf("%s:%d", record.Localip, record.Localport),
				fmt.Sprintf("%s:%s", record.Remoteip, wildcardIfZero(record.Remoteport)),
				state,
			}...)

			if netstatCmdFlags.extend {
				args = append(args, []interface{}{
					strconv.FormatUint(uint64(record.Uid), 10),
					strconv.FormatUint(record.Inode, 10),
				}...)
			}

			if netstatCmdFlags.pid {
				if record.Process.Pid != 0 {
					args = append(args, []interface{}{
						fmt.Sprintf("%d/%s", record.Process.Pid, record.Process.Name),
					}...)
				} else {
					args = append(args, []interface{}{
						"-",
					}...)
				}
			}

			if netstatCmdFlags.pods {
				if record.Netns == "" || node == "" || n.NodeNetNSPods[node] == nil {
					args = append(args, []interface{}{
						"-",
					}...)
				} else {
					args = append(args, []interface{}{
						n.NodeNetNSPods[node][record.Netns],
					}...)
				}
			}

			if netstatCmdFlags.timers {
				timerwhen := strconv.FormatFloat(float64(record.Timerwhen)/100, 'f', 2, 64)

				args = append(args, []interface{}{
					fmt.Sprintf("%s (%s/%d/%d)", strings.ToLower(record.Tr.String()), timerwhen, record.Retrnsmt, record.Timeout),
				}...)
			}

			pattern := strings.Repeat("%s\t", len(args))
			pattern = strings.TrimSpace(pattern) + "\n"

			fmt.Fprintf(w, pattern, args...)
		}
	}

	return w.Flush()
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
		}, "\t")

	if netstatCmdFlags.extend {
		labels += "\t" + strings.Join(
			[]string{
				"Uid",
				"Inode",
			}, "\t")
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
	NetstatCmd.Flags().BoolVarP(&netstatCmdFlags.verbose, "verbose", "v", false, "display sockets of all supported transport protocols")
	// extend is normally -e but cannot be used as this is endpoint in talosctl
	NetstatCmd.Flags().BoolVarP(&netstatCmdFlags.extend, "extend", "x", false, "show detailed socket information")
	NetstatCmd.Flags().BoolVarP(&netstatCmdFlags.pid, "programs", "p", false, "show process using socket")
	NetstatCmd.Flags().BoolVarP(&netstatCmdFlags.timers, "timers", "o", false, "display timers")
	NetstatCmd.Flags().BoolVarP(&netstatCmdFlags.listening, "listening", "l", false, "display listening server sockets")
	NetstatCmd.Flags().BoolVarP(&netstatCmdFlags.all, "all", "a", false, "display all sockets states (default: connected)")
	NetstatCmd.Flags().BoolVarP(&netstatCmdFlags.pods, "pods", "k", false, "show sockets used by Kubernetes pods")
	NetstatCmd.Flags().BoolVarP(&netstatCmdFlags.tcp, "tcp", "t", false, "display only TCP sockets")
	NetstatCmd.Flags().BoolVarP(&netstatCmdFlags.udp, "udp", "u", false, "display only UDP sockets")
	NetstatCmd.Flags().BoolVarP(&netstatCmdFlags.udplite, "udplite", "U", false, "display only UDPLite sockets")
	NetstatCmd.Flags().BoolVarP(&netstatCmdFlags.raw, "raw", "w", false, "display only RAW sockets")
	NetstatCmd.Flags().BoolVarP(&netstatCmdFlags.ipv4, "ipv4", "4", false, "display only ipv4 sockets")
	NetstatCmd.Flags().BoolVarP(&netstatCmdFlags.ipv6, "ipv6", "6", false, "display only ipv6 sockets")

	addCommand(NetstatCmd)
}
