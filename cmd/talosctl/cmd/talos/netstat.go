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

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/cli"
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
	tcp       bool
	udp       bool
	udplite   bool
	raw       bool
	ipv4      bool
	ipv6      bool
}

// netstatCmd represents the ls command.
var netstatCmd = &cobra.Command{
	Use:     "netstat",
	Aliases: []string{"ss"},
	Short:   "Retrieve a socket listing of connections",
	Long:    ``,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			req := netstatFlagsToRequest()
			response, err := c.Netstat(ctx, req)
			if err != nil {
				if response == nil {
					return fmt.Errorf("error getting netstat: %w", err)
				}

				cli.Warning("%s", err)
			}

			err = printNetstat(response)

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

//nolint:gocyclo
func printNetstat(response *machine.NetstatResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	node := ""

	labels := strings.Join(
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

	if netstatCmdFlags.timers {
		labels += "\t" + "Timer"
	}

	for i, message := range response.Messages {
		if message.Metadata != nil && message.Metadata.Hostname != "" {
			node = message.Metadata.Hostname
		}

		if len(message.Connectrecord) == 0 {
			continue
		}

		for j, record := range message.Connectrecord {
			if i == 0 && j == 0 {
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
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.tcp, "tcp", "t", false, "display only TCP sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.udp, "udp", "u", false, "display only UDP sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.udplite, "udplite", "U", false, "display only UDPLite sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.raw, "raw", "w", false, "display only RAW sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.ipv4, "ipv4", "4", false, "display only ipv4 sockets")
	netstatCmd.Flags().BoolVarP(&netstatCmdFlags.ipv4, "ipv6", "6", false, "display only ipv6 sockets")

	addCommand(netstatCmd)
}
