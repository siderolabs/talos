// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/talos-systems/talos/pkg/cli"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var verbose bool

// memoryCmd represents the processes command.
var memoryCmd = &cobra.Command{
	Use:     "memory",
	Aliases: []string{"m", "free"},
	Short:   "Show memory usage",
	Long:    ``,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			var remotePeer peer.Peer

			resp, err := c.Memory(ctx, grpc.Peer(&remotePeer))
			if err != nil {
				if resp == nil {
					return fmt.Errorf("error getting memory stats: %s", err)
				}

				cli.Warning("%s", err)
			}

			if verbose {
				return verboseRender(&remotePeer, resp)
			}

			return briefRender(&remotePeer, resp)
		})
	},
}

func briefRender(remotePeer *peer.Peer, resp *machineapi.MemoryResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tTOTAL\tUSED\tFREE\tSHARED\tBUFFERS\tCACHE\tAVAILABLE")

	defaultNode := client.AddrFromPeer(remotePeer)

	for _, msg := range resp.Messages {
		node := defaultNode

		if msg.Metadata != nil {
			node = msg.Metadata.Hostname
		}

		// Default to displaying output as MB
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
			node,
			msg.Meminfo.Memtotal/1024,
			(msg.Meminfo.Memtotal-msg.Meminfo.Memfree-msg.Meminfo.Cached-msg.Meminfo.Buffers)/1024,
			msg.Meminfo.Memfree/1024,
			msg.Meminfo.Shmem/1024,
			msg.Meminfo.Buffers/1024,
			msg.Meminfo.Cached/1024,
			msg.Meminfo.Memavailable/1024,
		)
	}

	return w.Flush()
}

func verboseRender(remotePeer *peer.Peer, resp *machineapi.MemoryResponse) error {
	defaultNode := client.AddrFromPeer(remotePeer)

	// Dump as /proc/meminfo
	for _, msg := range resp.Messages {
		node := defaultNode

		if msg.Metadata != nil {
			node = msg.Metadata.Hostname
		}

		fmt.Printf("%s: %s\n", "NODE", node)
		fmt.Printf("%s: %d %s\n", "MemTotal", msg.Meminfo.Memtotal, "kB")
		fmt.Printf("%s: %d %s\n", "MemFree", msg.Meminfo.Memfree, "kB")
		fmt.Printf("%s: %d %s\n", "MemAvailable", msg.Meminfo.Memavailable, "kB")
		fmt.Printf("%s: %d %s\n", "Buffers", msg.Meminfo.Buffers, "kB")
		fmt.Printf("%s: %d %s\n", "Cached", msg.Meminfo.Cached, "kB")
		fmt.Printf("%s: %d %s\n", "SwapCached", msg.Meminfo.Swapcached, "kB")
		fmt.Printf("%s: %d %s\n", "Active", msg.Meminfo.Active, "kB")
		fmt.Printf("%s: %d %s\n", "Inactive", msg.Meminfo.Inactive, "kB")
		fmt.Printf("%s: %d %s\n", "ActiveAnon", msg.Meminfo.Activeanon, "kB")
		fmt.Printf("%s: %d %s\n", "InactiveAnon", msg.Meminfo.Inactiveanon, "kB")
		fmt.Printf("%s: %d %s\n", "ActiveFile", msg.Meminfo.Activefile, "kB")
		fmt.Printf("%s: %d %s\n", "InactiveFile", msg.Meminfo.Inactivefile, "kB")
		fmt.Printf("%s: %d %s\n", "Unevictable", msg.Meminfo.Unevictable, "kB")
		fmt.Printf("%s: %d %s\n", "Mlocked", msg.Meminfo.Mlocked, "kB")
		fmt.Printf("%s: %d %s\n", "SwapTotal", msg.Meminfo.Swaptotal, "kB")
		fmt.Printf("%s: %d %s\n", "SwapFree", msg.Meminfo.Swapfree, "kB")
		fmt.Printf("%s: %d %s\n", "Dirty", msg.Meminfo.Dirty, "kB")
		fmt.Printf("%s: %d %s\n", "Writeback", msg.Meminfo.Writeback, "kB")
		fmt.Printf("%s: %d %s\n", "AnonPages", msg.Meminfo.Anonpages, "kB")
		fmt.Printf("%s: %d %s\n", "Mapped", msg.Meminfo.Mapped, "kB")
		fmt.Printf("%s: %d %s\n", "Shmem", msg.Meminfo.Shmem, "kB")
		fmt.Printf("%s: %d %s\n", "Slab", msg.Meminfo.Slab, "kB")
		fmt.Printf("%s: %d %s\n", "SReclaimable", msg.Meminfo.Sreclaimable, "kB")
		fmt.Printf("%s: %d %s\n", "SUnreclaim", msg.Meminfo.Sunreclaim, "kB")
		fmt.Printf("%s: %d %s\n", "KernelStack", msg.Meminfo.Kernelstack, "kB")
		fmt.Printf("%s: %d %s\n", "PageTables", msg.Meminfo.Pagetables, "kB")
		fmt.Printf("%s: %d %s\n", "NFSUnstable", msg.Meminfo.Nfsunstable, "kB")
		fmt.Printf("%s: %d %s\n", "Bounce", msg.Meminfo.Bounce, "kB")
		fmt.Printf("%s: %d %s\n", "WritebackTmp", msg.Meminfo.Writebacktmp, "kB")
		fmt.Printf("%s: %d %s\n", "CommitLimit", msg.Meminfo.Commitlimit, "kB")
		fmt.Printf("%s: %d %s\n", "CommittedAS", msg.Meminfo.Committedas, "kB")
		fmt.Printf("%s: %d %s\n", "VmallocTotal", msg.Meminfo.Vmalloctotal, "kB")
		fmt.Printf("%s: %d %s\n", "VmallocUsed", msg.Meminfo.Vmallocused, "kB")
		fmt.Printf("%s: %d %s\n", "VmallocChunk", msg.Meminfo.Vmallocchunk, "kB")
		fmt.Printf("%s: %d %s\n", "HardwareCorrupted", msg.Meminfo.Hardwarecorrupted, "kB")
		fmt.Printf("%s: %d %s\n", "AnonHugePages", msg.Meminfo.Anonhugepages, "kB")
		fmt.Printf("%s: %d %s\n", "ShmemHugePages", msg.Meminfo.Shmemhugepages, "kB")
		fmt.Printf("%s: %d %s\n", "ShmemPmdMapped", msg.Meminfo.Shmempmdmapped, "kB")
		fmt.Printf("%s: %d %s\n", "CmaTotal", msg.Meminfo.Cmatotal, "kB")
		fmt.Printf("%s: %d %s\n", "CmaFree", msg.Meminfo.Cmafree, "kB")
		fmt.Printf("%s: %d\n", "HugePagesTotal", msg.Meminfo.Hugepagestotal)
		fmt.Printf("%s: %d\n", "HugePagesFree", msg.Meminfo.Hugepagesfree)
		fmt.Printf("%s: %d\n", "HugePagesRsvd", msg.Meminfo.Hugepagesrsvd)
		fmt.Printf("%s: %d\n", "HugePagesSurp", msg.Meminfo.Hugepagessurp)
		fmt.Printf("%s: %d %s\n", "Hugepagesize", msg.Meminfo.Hugepagesize, "kB")
		fmt.Printf("%s: %d %s\n", "DirectMap4k", msg.Meminfo.Directmap4K, "kB")
		fmt.Printf("%s: %d %s\n", "DirectMap2M", msg.Meminfo.Directmap2M, "kB")
		fmt.Printf("%s: %d %s\n", "DirectMap1G", msg.Meminfo.Directmap1G, "kB")
	}

	return nil
}

func init() {
	memoryCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "display extended memory statistics")
	addCommand(memoryCmd)
}
