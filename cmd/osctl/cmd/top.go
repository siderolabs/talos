/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"fmt"
	"log"
	"sort"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/spf13/cobra"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/proc"
	"golang.org/x/crypto/ssh/terminal"
)

// versionCmd represents the version command
var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Streams top output",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := client.NewDefaultClientCredentials(talosconfig)
		if err != nil {
			helpers.Fatalf("error getting client credentials: %s", err)
		}
		c, err := client.NewClient(constants.OsdPort, creds)
		if err != nil {
			helpers.Fatalf("error constructing client: %s", err)
		}

		if err := ui.Init(); err != nil {
			log.Fatalf("failed to initialize termui: %v", err)
		}
		defer ui.Close()

		topUI(c)
	},
}

func init() {
	rootCmd.AddCommand(topCmd)
}

func topUI(c *client.Client) {

	l := widgets.NewList()
	l.Title = "Top"
	l.TextStyle.Fg = ui.ColorYellow

	draw := func(procs []proc.ProcessList) {
		rss := func(p1, p2 *proc.ProcessList) bool {
			// Reverse sort ( Descending )
			return p1.ResidentMemory > p2.ResidentMemory
		}
		by(rss).sort(procs)
		s := make([]string, 0, len(procs))
		s = append(s, fmt.Sprintf("%s %s %s %s %s %s %s %s", "PID", "State", "Threads", "CPU Time", "VirtMem", "ResMem", "Command", "Exec/Args"))
		for _, p := range procs {
			s = append(s, p.String())
		}
		l.Rows = s

		// Attempt to get terminal dimensions
		// Since we're getting this data on each call
		// we'll be able to handle terminal window resizing
		w, h, err := terminal.GetSize(0)
		if err != nil {
			log.Fatal("Unable to determine terminal size")
		}
		// x, y, w, h
		l.SetRect(0, 0, w, h)

		ui.Render(l)
	}

	procs, err := c.Top()
	if err != nil {
		log.Println(err)
		return
	}
	draw(procs)
	uiEvents := ui.PollEvents()
	ticker := time.NewTicker(time.Second).C
	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return
			}
		case <-ticker:
			procs, err := c.Top()
			if err != nil {
				log.Println(err)
				return
			}
			draw(procs)
		}
	}
}

type by func(p1, p2 *proc.ProcessList) bool

func (b by) sort(procs []proc.ProcessList) {
	ps := &procSorter{
		procs: procs,
		by:    b, // The Sort method's receiver is the function (closure) that defines the sort order.
	}
	sort.Sort(ps)
}

type procSorter struct {
	procs []proc.ProcessList
	by    func(p1, p2 *proc.ProcessList) bool // Closure used in the Less method.
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
	return s.by(&s.procs[i], &s.procs[j])
}
