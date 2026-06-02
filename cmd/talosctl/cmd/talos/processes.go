// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

var processesCmdFlags struct {
	sortMethod string
}

// processesCmd represents the processes command.
var processesCmd = &cobra.Command{
	Use:     "processes",
	Aliases: []string{"p", "ps"},
	Short:   "List running processes",
	Long:    ``,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, nil)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (*machineapi.ProcessesResponse, error) {
				return c.Processes(ctx)
			},
		)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NODE\tPID\tSTATE\tTHREADS\tCPU-TIME\tVIRTMEM\tRESMEM\tLABEL\tCOMMAND")

		flushTimer := time.NewTimer(outputFlushInterval)
		defer flushTimer.Stop()

		flushTimer.Stop()

		var errs error

		for {
			select {
			case resp, ok := <-responseChan:
				if !ok {
					return errors.Join(errs, w.Flush())
				}

				if resp.Err != nil {
					errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))
				} else {
					for _, msg := range resp.Payload.Messages {
						procs := msg.Processes

						switch processesCmdFlags.sortMethod {
						case "cpu":
							by(cpu).sort(procs)
						default:
							by(rss).sort(procs)
						}

						for _, p := range procs {
							fmt.Fprintf(
								w, "%s\t%d\t%s\t%d\t%.2f\t%s\t%s\t%s\t%s\n",
								resp.Node, p.Pid, p.State, p.Threads, p.CpuTime,
								humanize.Bytes(p.VirtualMemory), humanize.Bytes(p.ResidentMemory),
								p.Label, processArgs(p),
							)
						}
					}
				}

				flushTimer.Reset(outputFlushInterval)
			case <-flushTimer.C:
				if err := w.Flush(); err != nil {
					errs = errors.Join(errs, fmt.Errorf("error flushing output: %w", err))
				}
			}
		}
	},
}

func processArgs(p *machineapi.ProcessInfo) string {
	var args string

	switch {
	case p.Executable == "":
		args = p.Command
	case p.Args != "" && strings.Fields(p.Args)[0] == filepath.Base(strings.Fields(p.Executable)[0]):
		args = strings.Replace(p.Args, strings.Fields(p.Args)[0], p.Executable, 1)
	default:
		args = p.Args
	}

	// filter out non-printable characters
	return strings.Map(func(r rune) rune {
		if r < 32 || r > 126 {
			return ' '
		}

		return r
	}, args)
}

func init() {
	processesCmd.Flags().StringVarP(&processesCmdFlags.sortMethod, "sort", "s", "rss", "Column to sort output by. [rss|cpu]")
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
