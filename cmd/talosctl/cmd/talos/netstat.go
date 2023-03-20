// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	criconstants "github.com/containerd/containerd/pkg/cri/constants"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
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
	client      *client.Client
	node        string
	nodePidPods map[string]map[uint32]string
}

// netstatCmd represents the netstat command.
var netstatCmd = &cobra.Command{
	Use:     "netstat",
	Aliases: []string{"ss"},
	Short:   "Retrieve a socket listing of connections",
	Long: `Retrieve a socket listing of connections.
	Optional argument can be passed to view a specific pod's connections. Format argument in the namespace/pod format.
	If no argument is passed, host connections are shown.`,
	Args: cobra.MaximumNArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
		}

		var podList []string

		if WithClient(func(ctx context.Context, c *client.Client) error {
			nstat := netstat{
				nodePidPods: make(map[string]map[uint32]string),
				client:      c,
			}

			err := nstat.getPodPidsFromNode(ctx, false)
			if err != nil {
				return err
			}

			for node, pidPods := range nstat.nodePidPods {
				for _, podName := range pidPods {
					podList = append(podList, fmt.Sprintf("%s\t%s", podName, node))
				}
			}

			return nil
		}) != nil {
			return nil, cobra.ShellCompDirectiveError | cobra.ShellCompDirectiveNoFileComp
		}

		return podList, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		response := make([]*machine.NetstatResponse, 1)
		respondedNode := make([]string, 1)

		req := netstatFlagsToRequest()

		return WithClient(func(ctx context.Context, c *client.Client) error {
			findThePod := len(args) > 0 && !netstatCmdFlags.pods
			var printNodeHeader bool

			nstat := netstat{
				client: c,
			}

			nstat.nodePidPods = make(map[string]map[uint32]string)

			if findThePod {
				var foundPod uint32

				err := nstat.getPodPidsFromNode(ctx, false)
				if err != nil {
					return err
				}

				ctx, foundPod = nstat.findPodPidAndUpdateContext(ctx, args[0])

				if foundPod == 0 {
					cli.Fatalf("pod %s not found", args[0])
				}

				req.Netns.Podpids = []uint32{foundPod}
				req.Netns.Hostnetwork = false
			}

			var err error

			if !netstatCmdFlags.pods {
				response, printNodeHeader, err = singleNetstatRequest(ctx, c, req)
			} else {
				response, respondedNode, printNodeHeader, err = nstat.multiUniqueNetstatRequest(ctx, c, req)
			}

			if err != nil {
				return err
			}

			err = nstat.printNetstat(response, respondedNode, printNodeHeader)
			if err != nil {
				return err
			}

			return nil
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

func (nstat *netstat) getPodPidsFromNode(ctx context.Context, hostNetworkFilter bool) (err error) {
	resp, err := nstat.client.Containers(ctx, criconstants.K8sContainerdNamespace, common.ContainerDriver_CRI)
	if err != nil {
		cli.Warning("error getting containers: %v", err)

		return err
	}

	for _, msg := range resp.Messages {
		for _, p := range msg.Containers {
			if p.Pid == 0 {
				continue
			}

			if p.Id != p.PodId {
				continue
			}

			if hostNetworkFilter && p.Hostnetwork {
				continue
			}

			if nstat.nodePidPods[msg.Metadata.Hostname] == nil {
				nstat.nodePidPods[msg.Metadata.Hostname] = make(map[uint32]string)
			}

			nstat.nodePidPods[msg.Metadata.Hostname][p.Pid] = p.Id
		}
	}

	return nil
}

func (nstat *netstat) getPodPids() (pids []uint32) {
	for podPid := range nstat.nodePidPods[nstat.node] {
		pids = append(pids, podPid)
	}

	return pids
}

func (nstat *netstat) findPodPidAndUpdateContext(ctx context.Context, findNamespaceAndPod string) (context.Context, uint32) {
	var foundPid uint32

	for node, pidPods := range nstat.nodePidPods {
		if len(pidPods) > 0 {
			for pid, podName := range pidPods {
				if podName == strings.ToLower(findNamespaceAndPod) {
					foundPid = pid
					ctx = client.WithNode(ctx, node)

					break
				}
			}
		}
	}

	return ctx, foundPid
}

func singleNetstatRequest(ctx context.Context, c *client.Client, req *machine.NetstatRequest) (response []*machine.NetstatResponse, printNodeHeader bool, err error) {
	response = make([]*machine.NetstatResponse, 1)
	response[0], err = c.Netstat(ctx, req)

	if err != nil {
		if response[0] == nil {
			return response, printNodeHeader, err
		}

		cli.Warning("%s", err)
	}

	printNodeHeader = len(response[0].Messages) > 1

	return response, printNodeHeader, nil
}

func (nstat *netstat) multiUniqueNetstatRequest(ctx context.Context, c *client.Client, req *machine.NetstatRequest) (
	response []*machine.NetstatResponse,
	respondedNode []string,
	printNodeHeader bool,
	err error,
) {
	md, _ := metadata.FromOutgoingContext(ctx)
	nodes := md.Get("nodes")

	response = make([]*machine.NetstatResponse, len(nodes))
	respondedNode = make([]string, len(nodes))
	printNodeHeader = len(nodes) > 1

	err = nstat.getPodPidsFromNode(ctx, true)
	if err != nil {
		return nil, nil, false, err
	}

	for i, node := range nodes {
		nstat.node = node
		nodeCtx := client.WithNode(ctx, node)

		req.Netns.Podpids = nstat.getPodPids()
		respondedNode[i] = node

		response[i], err = c.Netstat(nodeCtx, req)
		if err != nil {
			if response[0] == nil {
				return nil, nil, false, fmt.Errorf("error getting netstat: %w", err)
			}

			cli.Warning("%s", err)
		}
	}

	return response, respondedNode, printNodeHeader, nil
}

//nolint:gocyclo
func (nstat *netstat) printNetstat(responses []*machine.NetstatResponse, respondedNode []string, printNodeHeader bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	headerPrinted := false

	for i, response := range responses {
		if response == nil {
			continue
		}

		for _, message := range response.Messages {
			if len(message.Connectrecord) == 0 {
				continue
			}

			for _, record := range message.Connectrecord {
				if !headerPrinted {
					fmt.Fprintln(w, netstatSummary(printNodeHeader))

					headerPrinted = true
				}

				args := []interface{}{}

				if printNodeHeader {
					if message.Metadata == nil {
						args = append(args, respondedNode[i])
					} else {
						args = append(args, message.Metadata.Hostname)
					}
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
					if record.Process.Pid == 0 {
						args = append(args, []interface{}{
							"-",
						}...)
					} else {
						args = append(args, []interface{}{
							fmt.Sprintf("%d/%s", record.Process.Pid, record.Process.Name),
						}...)
					}
				}

				if netstatCmdFlags.pods {
					if record.Podpid == 0 || nstat.nodePidPods[respondedNode[i]] == nil {
						args = append(args, []interface{}{
							"-",
						}...)
					} else {
						args = append(args, []interface{}{
							nstat.nodePidPods[respondedNode[i]][record.Podpid],
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
	}

	return w.Flush()
}

func netstatSummary(printNodeHeader bool) (labels string) {
	if printNodeHeader {
		labels = "NODE\t"
	}

	labels += strings.Join(
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
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.verbose, "verbose", "v", false, "display sockets of all supported transport protocols")
	// extend is normally -e but cannot be used as this is endpoint in talosctl
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.extend, "extend", "x", false, "show detailed socket information")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.pid, "programs", "p", false, "show process using socket")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.timers, "timers", "o", false, "display timers")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.listening, "listening", "l", false, "display listening server sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.all, "all", "a", false, "display all sockets states (default: connected)")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.pods, "pods", "k", false, "show sockets used by kubernetes pods")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.tcp, "tcp", "t", false, "display only TCP sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.udp, "udp", "u", false, "display only UDP sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.udplite, "udplite", "U", false, "display only UDPLite sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.raw, "raw", "w", false, "display only RAW sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.ipv4, "ipv4", "4", false, "display only ipv4 sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.ipv6, "ipv6", "6", false, "display only ipv6 sockets")

	addCommand(netstatCmd)
}
