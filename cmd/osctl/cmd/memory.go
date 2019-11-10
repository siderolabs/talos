// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: dupl,golint
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	osapi "github.com/talos-systems/talos/api/os"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
)

var verbose bool

// memoryCmd represents the processes command
var memoryCmd = &cobra.Command{
	Use:     "memory",
	Aliases: []string{"m"},
	Short:   "Show memory usage",
	Long:    ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			helpers.Should(cmd.Usage())
			os.Exit(1)
		}

		setupClient(func(c *client.Client) {
			reply, err := c.Memory(globalCtx)
			if err != nil {
				helpers.Fatalf("error getting memory stats: %s", err)
			}

			if verbose {
				verboseRender(reply)
			} else {
				briefRender(reply)
			}
		})
	},
}

func briefRender(reply *osapi.MemInfoReply) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tTOTAL\tUSED\tFREE\tSHARED\tBUFFERS\tCACHE\tAVAILABLE")

	for _, resp := range reply.Response {
		// Default to displaying output as MB
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
			resp.Metadata.Hostname,
			resp.Meminfo.Memtotal/1024,
			(resp.Meminfo.Memtotal-resp.Meminfo.Memfree-resp.Meminfo.Cached-resp.Meminfo.Buffers)/1024,
			resp.Meminfo.Memfree/1024,
			resp.Meminfo.Shmem/1024,
			resp.Meminfo.Buffers/1024,
			resp.Meminfo.Cached/1024,
			resp.Meminfo.Memavailable/1024,
		)
	}

	helpers.Should(w.Flush())
}

func verboseRender(reply *osapi.MemInfoReply) {
	// Dump as /proc/meminfo
	for _, resp := range reply.Response {
		fmt.Printf("%s: %s\n", "NODE", resp.Metadata.Hostname)
		fmt.Printf("%s: %d %s\n", "MemTotal", resp.Meminfo.Memtotal, "kB")
		fmt.Printf("%s: %d %s\n", "MemFree", resp.Meminfo.Memfree, "kB")
		fmt.Printf("%s: %d %s\n", "MemAvailable", resp.Meminfo.Memavailable, "kB")
		fmt.Printf("%s: %d %s\n", "Buffers", resp.Meminfo.Buffers, "kB")
		fmt.Printf("%s: %d %s\n", "Cached", resp.Meminfo.Cached, "kB")
		fmt.Printf("%s: %d %s\n", "SwapCached", resp.Meminfo.Swapcached, "kB")
		fmt.Printf("%s: %d %s\n", "Active", resp.Meminfo.Active, "kB")
		fmt.Printf("%s: %d %s\n", "Inactive", resp.Meminfo.Inactive, "kB")
		fmt.Printf("%s: %d %s\n", "ActiveAnon", resp.Meminfo.Activeanon, "kB")
		fmt.Printf("%s: %d %s\n", "InactiveAnon", resp.Meminfo.Inactiveanon, "kB")
		fmt.Printf("%s: %d %s\n", "ActiveFile", resp.Meminfo.Activefile, "kB")
		fmt.Printf("%s: %d %s\n", "InactiveFile", resp.Meminfo.Inactivefile, "kB")
		fmt.Printf("%s: %d %s\n", "Unevictable", resp.Meminfo.Unevictable, "kB")
		fmt.Printf("%s: %d %s\n", "Mlocked", resp.Meminfo.Mlocked, "kB")
		fmt.Printf("%s: %d %s\n", "SwapTotal", resp.Meminfo.Swaptotal, "kB")
		fmt.Printf("%s: %d %s\n", "SwapFree", resp.Meminfo.Swapfree, "kB")
		fmt.Printf("%s: %d %s\n", "Dirty", resp.Meminfo.Dirty, "kB")
		fmt.Printf("%s: %d %s\n", "Writeback", resp.Meminfo.Writeback, "kB")
		fmt.Printf("%s: %d %s\n", "AnonPages", resp.Meminfo.Anonpages, "kB")
		fmt.Printf("%s: %d %s\n", "Mapped", resp.Meminfo.Mapped, "kB")
		fmt.Printf("%s: %d %s\n", "Shmem", resp.Meminfo.Shmem, "kB")
		fmt.Printf("%s: %d %s\n", "Slab", resp.Meminfo.Slab, "kB")
		fmt.Printf("%s: %d %s\n", "SReclaimable", resp.Meminfo.Sreclaimable, "kB")
		fmt.Printf("%s: %d %s\n", "SUnreclaim", resp.Meminfo.Sunreclaim, "kB")
		fmt.Printf("%s: %d %s\n", "KernelStack", resp.Meminfo.Kernelstack, "kB")
		fmt.Printf("%s: %d %s\n", "PageTables", resp.Meminfo.Pagetables, "kB")
		fmt.Printf("%s: %d %s\n", "NFSUnstable", resp.Meminfo.Nfsunstable, "kB")
		fmt.Printf("%s: %d %s\n", "Bounce", resp.Meminfo.Bounce, "kB")
		fmt.Printf("%s: %d %s\n", "WritebackTmp", resp.Meminfo.Writebacktmp, "kB")
		fmt.Printf("%s: %d %s\n", "CommitLimit", resp.Meminfo.Commitlimit, "kB")
		fmt.Printf("%s: %d %s\n", "CommittedAS", resp.Meminfo.Committedas, "kB")
		fmt.Printf("%s: %d %s\n", "VmallocTotal", resp.Meminfo.Vmalloctotal, "kB")
		fmt.Printf("%s: %d %s\n", "VmallocUsed", resp.Meminfo.Vmallocused, "kB")
		fmt.Printf("%s: %d %s\n", "VmallocChunk", resp.Meminfo.Vmallocchunk, "kB")
		fmt.Printf("%s: %d %s\n", "HardwareCorrupted", resp.Meminfo.Hardwarecorrupted, "kB")
		fmt.Printf("%s: %d %s\n", "AnonHugePages", resp.Meminfo.Anonhugepages, "kB")
		fmt.Printf("%s: %d %s\n", "ShmemHugePages", resp.Meminfo.Shmemhugepages, "kB")
		fmt.Printf("%s: %d %s\n", "ShmemPmdMapped", resp.Meminfo.Shmempmdmapped, "kB")
		fmt.Printf("%s: %d %s\n", "CmaTotal", resp.Meminfo.Cmatotal, "kB")
		fmt.Printf("%s: %d %s\n", "CmaFree", resp.Meminfo.Cmafree, "kB")
		fmt.Printf("%s: %d\n", "HugePagesTotal", resp.Meminfo.Hugepagestotal)
		fmt.Printf("%s: %d\n", "HugePagesFree", resp.Meminfo.Hugepagesfree)
		fmt.Printf("%s: %d\n", "HugePagesRsvd", resp.Meminfo.Hugepagesrsvd)
		fmt.Printf("%s: %d\n", "HugePagesSurp", resp.Meminfo.Hugepagessurp)
		fmt.Printf("%s: %d %s\n", "Hugepagesize", resp.Meminfo.Hugepagesize, "kB")
		fmt.Printf("%s: %d %s\n", "DirectMap4k", resp.Meminfo.Directmap4K, "kB")
		fmt.Printf("%s: %d %s\n", "DirectMap2M", resp.Meminfo.Directmap2M, "kB")
		fmt.Printf("%s: %d %s\n", "DirectMap1G", resp.Meminfo.Directmap1G, "kB")
	}
}

func init() {
	memoryCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "display extended memory statistics")
	rootCmd.AddCommand(memoryCmd)
}
