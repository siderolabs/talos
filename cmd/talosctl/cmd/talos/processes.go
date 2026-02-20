// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/ryanuber/columnize"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

var sortMethod string

// processesCmd represents the processes command.
var processesCmd = &cobra.Command{
	Use:     "processes",
	Aliases: []string{"p", "ps"},
	Short:   "List running processes",
	Long:    ``,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			output, err := processesOutput(ctx, c)
			if err != nil {
				return err
			}

			fmt.Println(output)

			return nil
		})
	},
}

func init() {
	processesCmd.Flags().StringVarP(&sortMethod, "sort", "s", "rss", "Column to sort output by. [rss|cpu]")
	addCommand(processesCmd)
}

type by func(p1, p2 *machineapi.ProcessInfo) bool

func (b by) sort(procs []*machineapi.ProcessInfo) {
	ps := &procSorter{
		procs: procs,
		by:    b, // The Sort method's receiver is the function (closure) that defines the sort order.
	}
	sort.Sort(ps)
}

type procSorter struct {
	procs []*machineapi.ProcessInfo
	by    func(p1, p2 *machineapi.ProcessInfo) bool // Closure used in the Less method.
}

// Len is part of sort.Interface.
func (s *procSorter) Len() int {
	return len(s.procs)
}

// Swap is part of sort.Interface.
func (s *procSorter) Swap(i, j int) {
	s.procs[i], s.procs[j] = s.procs[j], s.procs[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (s *procSorter) Less(i, j int) bool {
	return s.by(s.procs[i], s.procs[j])
}

// Sort Methods.
var rss = func(p1, p2 *machineapi.ProcessInfo) bool {
	// Reverse sort ( Descending )
	return p1.ResidentMemory > p2.ResidentMemory
}

var cpu = func(p1, p2 *machineapi.ProcessInfo) bool {
	// Reverse sort ( Descending )
	return p1.CpuTime > p2.CpuTime
}

//nolint:gocyclo
func processesOutput(ctx context.Context, c *client.Client) (output string, err error) {
	var remotePeer peer.Peer

	resp, err := c.Processes(ctx, grpc.Peer(&remotePeer))
	if err != nil {
		return output, err
	}

	defaultNode := client.AddrFromPeer(&remotePeer)

	var s []string

	s = append(s, "NODE | PID | STATE | THREADS | CPU-TIME | VIRTMEM | RESMEM | LABEL | COMMAND")

	for _, msg := range resp.Messages {
		procs := msg.Processes

		switch sortMethod {
		case "cpu":
			by(cpu).sort(procs)
		default:
			by(rss).sort(procs)
		}

		var args string

		for _, p := range procs {
			switch {
			case p.Executable == "":
				args = p.Command
			case p.Args != "" && strings.Fields(p.Args)[0] == filepath.Base(strings.Fields(p.Executable)[0]):
				args = strings.Replace(p.Args, strings.Fields(p.Args)[0], p.Executable, 1)
			default:
				args = p.Args
			}

			// filter out non-printable characters
			args = strings.Map(func(r rune) rune {
				if r < 32 || r > 126 {
					return ' '
				}

				return r
			}, args)

			node := defaultNode

			if msg.Metadata != nil {
				node = msg.Metadata.Hostname
			}

			s = append(s,
				fmt.Sprintf("%12s | %6d | %1s | %4d | %8.2f | %7s | %7s | %64s | %s",
					node, p.Pid, p.State, p.Threads, p.CpuTime, humanize.Bytes(p.VirtualMemory), humanize.Bytes(p.ResidentMemory), p.Label, args))
		}
	}

	res := columnize.SimpleFormat(s)

	return res, helpers.CheckErrors(resp.Messages...)
}
