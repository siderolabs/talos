/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"code.cloudfoundry.org/bytefmt"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/ryanuber/columnize"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	osapi "github.com/talos-systems/talos/api/os"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

var sortMethod string
var watchProcesses bool

// processesCmd represents the processes command
var processesCmd = &cobra.Command{
	Use:     "processes",
	Aliases: []string{"p"},
	Short:   "List running processes",
	Long:    ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		setupClient(func(c *client.Client) {
			var err error

			switch {
			case watchProcesses:
				if err = ui.Init(); err != nil {
					log.Fatalf("failed to initialize termui: %v", err)
				}
				defer ui.Close()

				processesUI(globalCtx, c)
			default:
				var output string
				output, err = processesOutput(globalCtx, c)
				helpers.Should(err)
				// Note this is unlimited output of process lines
				// we arent artificially limited by the box we would otherwise draw
				fmt.Println(output)
			}
		})
	},
}

func init() {
	processesCmd.Flags().StringVarP(&sortMethod, "sort", "s", "rss", "Column to sort output by. [rss|cpu]")
	processesCmd.Flags().BoolVarP(&watchProcesses, "watch", "w", false, "Stream running processes")
	rootCmd.AddCommand(processesCmd)
}

// nolint: gocyclo
func processesUI(ctx context.Context, c *client.Client) {
	l := widgets.NewParagraph()
	l.Border = false
	l.WrapText = false
	l.PaddingTop = 0
	l.PaddingBottom = 0

	var processOutput string

	draw := func() {
		// Attempt to get terminal dimensions
		// Since we're getting this data on each call
		// we'll be able to handle terminal window resizing
		w, h, err := terminal.GetSize(0)
		helpers.Should(err)
		// x, y, w, h
		l.SetRect(0, 0, w, h)

		processOutput, err = processesOutput(ctx, c)
		helpers.Should(err)

		// Dont refresh if we dont have any output
		if processOutput == "" {
			return
		}

		// Truncate our output based on terminal size
		l.Text = processOutput

		ui.Render(l)
	}

	draw()

	uiEvents := ui.PollEvents()
	ticker := time.NewTicker(time.Second).C
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return
			case "r", "m":
				sortMethod = "rss"
			case "c":
				sortMethod = "cpu"
			}
		case <-ticker:
			draw()
		}
	}
}

type by func(p1, p2 *osapi.Process) bool

func (b by) sort(procs []*osapi.Process) {
	ps := &procSorter{
		procs: procs,
		by:    b, // The Sort method's receiver is the function (closure) that defines the sort order.
	}
	sort.Sort(ps)
}

type procSorter struct {
	procs []*osapi.Process
	by    func(p1, p2 *osapi.Process) bool // Closure used in the Less method.
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

// Sort Methods
var rss = func(p1, p2 *osapi.Process) bool {
	// Reverse sort ( Descending )
	return p1.ResidentMemory > p2.ResidentMemory
}

var cpu = func(p1, p2 *osapi.Process) bool {
	// Reverse sort ( Descending )
	return p1.CpuTime > p2.CpuTime
}

func processesOutput(ctx context.Context, c *client.Client) (output string, err error) {
	reply, err := c.Processes(ctx)
	if err != nil {
		// TODO: Figure out how to expose errors to client without messing
		// up display
		// TODO: Update server side code to not throw an error when process
		// no longer exists ( /proc/1234/comm no such file or directory )
		return output, nil
	}

	procs := reply.Processes

	switch sortMethod {
	case "cpu":
		by(cpu).sort(procs)
	default:
		by(rss).sort(procs)
	}

	s := make([]string, 0, len(procs))
	s = append(s, "PID | STATE | THREADS | CPU-TIME | VIRTMEM | RESMEM | COMMAND")
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

		s = append(s,
			fmt.Sprintf("%6d | %1s | %4d | %8.2f | %7s | %7s | %s",
				p.Pid, p.State, p.Threads, p.CpuTime, bytefmt.ByteSize(p.VirtualMemory), bytefmt.ByteSize(p.ResidentMemory), args))
	}

	return columnize.SimpleFormat(s), err
}
