// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/safeout"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

var memoryCmdFlags struct {
	global.InsecureFlags

	verbose bool
}

// memoryCmd represents the processes command.
var memoryCmd = &cobra.Command{
	Use:     "memory",
	Aliases: []string{"m", "free"},
	Short:   "Show memory usage",
	Long:    ``,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		clientFactory, err := NewClientFactory(ctx, &memoryCmdFlags)
		if err != nil {
			return err
		}

		defer clientFactory.Close() //nolint:errcheck

		responseChan := multiplex.UnaryViaFactory(
			ctx, clientFactory,
			func(ctx context.Context, c *client.Client) (*machineapi.MemoryResponse, error) {
				return c.Memory(ctx)
			},
		)

		if memoryCmdFlags.verbose {
			return renderVerbose(responseChan)
		}

		return renderBrief(responseChan)
	},
}

func renderBrief(responseChan <-chan multiplex.Response[*machineapi.MemoryResponse]) error {
	w := tabwriter.NewWriter(safeout.Stdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NODE\tTOTAL\tUSED\tFREE\tSHARED\tBUFFERS\tCACHE\tAVAILABLE")

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
					// Default to displaying output as MB
					safeout.Fprintf(
						w, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
						resp.Node,
						msg.Meminfo.Memtotal/1024,
						(msg.Meminfo.Memtotal-msg.Meminfo.Memfree-msg.Meminfo.Cached-msg.Meminfo.Buffers)/1024,
						msg.Meminfo.Memfree/1024,
						msg.Meminfo.Shmem/1024,
						msg.Meminfo.Buffers/1024,
						msg.Meminfo.Cached/1024,
						msg.Meminfo.Memavailable/1024,
					)
				}
			}

			flushTimer.Reset(outputFlushInterval)
		case <-flushTimer.C:
			if err := w.Flush(); err != nil {
				errs = errors.Join(errs, fmt.Errorf("error flushing output: %w", err))
			}
		}
	}
}

func renderVerbose(responseChan <-chan multiplex.Response[*machineapi.MemoryResponse]) error {
	var errs error

	for resp := range responseChan {
		if resp.Err != nil {
			errs = errors.Join(errs, fmt.Errorf("error from node %s: %w", resp.Node, resp.Err))

			continue
		}

		// Dump as /proc/meminfo
		for _, msg := range resp.Payload.Messages {
			safeout.Printf("%s: %s\n", "NODE", resp.Node)
			safeout.Printf("%s: %d %s\n", "MemTotal", msg.Meminfo.Memtotal, "kB")
			safeout.Printf("%s: %d %s\n", "MemFree", msg.Meminfo.Memfree, "kB")
			safeout.Printf("%s: %d %s\n", "MemAvailable", msg.Meminfo.Memavailable, "kB")
			safeout.Printf("%s: %d %s\n", "Buffers", msg.Meminfo.Buffers, "kB")
			safeout.Printf("%s: %d %s\n", "Cached", msg.Meminfo.Cached, "kB")
			safeout.Printf("%s: %d %s\n", "SwapCached", msg.Meminfo.Swapcached, "kB")
			safeout.Printf("%s: %d %s\n", "Active", msg.Meminfo.Active, "kB")
			safeout.Printf("%s: %d %s\n", "Inactive", msg.Meminfo.Inactive, "kB")
			safeout.Printf("%s: %d %s\n", "ActiveAnon", msg.Meminfo.Activeanon, "kB")
			safeout.Printf("%s: %d %s\n", "InactiveAnon", msg.Meminfo.Inactiveanon, "kB")
			safeout.Printf("%s: %d %s\n", "ActiveFile", msg.Meminfo.Activefile, "kB")
			safeout.Printf("%s: %d %s\n", "InactiveFile", msg.Meminfo.Inactivefile, "kB")
			safeout.Printf("%s: %d %s\n", "Unevictable", msg.Meminfo.Unevictable, "kB")
			safeout.Printf("%s: %d %s\n", "Mlocked", msg.Meminfo.Mlocked, "kB")
			safeout.Printf("%s: %d %s\n", "SwapTotal", msg.Meminfo.Swaptotal, "kB")
			safeout.Printf("%s: %d %s\n", "SwapFree", msg.Meminfo.Swapfree, "kB")
			safeout.Printf("%s: %d %s\n", "Dirty", msg.Meminfo.Dirty, "kB")
			safeout.Printf("%s: %d %s\n", "Writeback", msg.Meminfo.Writeback, "kB")
			safeout.Printf("%s: %d %s\n", "AnonPages", msg.Meminfo.Anonpages, "kB")
			safeout.Printf("%s: %d %s\n", "Mapped", msg.Meminfo.Mapped, "kB")
			safeout.Printf("%s: %d %s\n", "Shmem", msg.Meminfo.Shmem, "kB")
			safeout.Printf("%s: %d %s\n", "Slab", msg.Meminfo.Slab, "kB")
			safeout.Printf("%s: %d %s\n", "SReclaimable", msg.Meminfo.Sreclaimable, "kB")
			safeout.Printf("%s: %d %s\n", "SUnreclaim", msg.Meminfo.Sunreclaim, "kB")
			safeout.Printf("%s: %d %s\n", "KernelStack", msg.Meminfo.Kernelstack, "kB")
			safeout.Printf("%s: %d %s\n", "PageTables", msg.Meminfo.Pagetables, "kB")
			safeout.Printf("%s: %d %s\n", "NFSUnstable", msg.Meminfo.Nfsunstable, "kB")
			safeout.Printf("%s: %d %s\n", "Bounce", msg.Meminfo.Bounce, "kB")
			safeout.Printf("%s: %d %s\n", "WritebackTmp", msg.Meminfo.Writebacktmp, "kB")
			safeout.Printf("%s: %d %s\n", "CommitLimit", msg.Meminfo.Commitlimit, "kB")
			safeout.Printf("%s: %d %s\n", "CommittedAS", msg.Meminfo.Committedas, "kB")
			safeout.Printf("%s: %d %s\n", "VmallocTotal", msg.Meminfo.Vmalloctotal, "kB")
			safeout.Printf("%s: %d %s\n", "VmallocUsed", msg.Meminfo.Vmallocused, "kB")
			safeout.Printf("%s: %d %s\n", "VmallocChunk", msg.Meminfo.Vmallocchunk, "kB")
			safeout.Printf("%s: %d %s\n", "HardwareCorrupted", msg.Meminfo.Hardwarecorrupted, "kB")
			safeout.Printf("%s: %d %s\n", "AnonHugePages", msg.Meminfo.Anonhugepages, "kB")
			safeout.Printf("%s: %d %s\n", "ShmemHugePages", msg.Meminfo.Shmemhugepages, "kB")
			safeout.Printf("%s: %d %s\n", "ShmemPmdMapped", msg.Meminfo.Shmempmdmapped, "kB")
			safeout.Printf("%s: %d %s\n", "CmaTotal", msg.Meminfo.Cmatotal, "kB")
			safeout.Printf("%s: %d %s\n", "CmaFree", msg.Meminfo.Cmafree, "kB")
			safeout.Printf("%s: %d\n", "HugePagesTotal", msg.Meminfo.Hugepagestotal)
			safeout.Printf("%s: %d\n", "HugePagesFree", msg.Meminfo.Hugepagesfree)
			safeout.Printf("%s: %d\n", "HugePagesRsvd", msg.Meminfo.Hugepagesrsvd)
			safeout.Printf("%s: %d\n", "HugePagesSurp", msg.Meminfo.Hugepagessurp)
			safeout.Printf("%s: %d %s\n", "Hugepagesize", msg.Meminfo.Hugepagesize, "kB")
			safeout.Printf("%s: %d %s\n", "DirectMap4k", msg.Meminfo.Directmap4K, "kB")
			safeout.Printf("%s: %d %s\n", "DirectMap2M", msg.Meminfo.Directmap2M, "kB")
			safeout.Printf("%s: %d %s\n", "DirectMap1G", msg.Meminfo.Directmap1G, "kB")
		}
	}

	return errs
}

func init() {
	memoryCmd.Flags().BoolVarP(&memoryCmdFlags.verbose, "verbose", "v", false, "display extended memory statistics")
	memoryCmdFlags.InsecureFlags.AddFlags(memoryCmd)
	addCommand(memoryCmd)
}
