// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcapgo"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"

	"github.com/talos-systems/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

var pcapCmdFlags struct {
	iface     string
	promisc   bool
	snaplen   int
	output    string
	bpfFilter string
	duration  time.Duration
}

// pcapCmd represents the pcap command.
var pcapCmd = &cobra.Command{
	Use:     "pcap",
	Aliases: []string{"tcpdump"},
	Short:   "Capture the network packets from the node.",
	Long: `The command launches packet capture on the node and streams back the packets as raw pcap file.

Default behavior is to decode the packets with internal decoder to stdout:

  talosctl pcap -i eth0

Raw pcap file can be saved with --output flag:

  talosctl pcap -i eth0 --output eth0.pcap

Output can be piped to tcpdump:

  talosctl pcap -i eth0 -o - | tcpdump -vvv -r -

 BPF filter can be applied, but it has to compiled to BPF instructions first using tcpdump.
 Correct link type should be specified for the tcpdump: EN10MB for Ethernet links and RAW
 for e.g. Wireguard tunnels:

   talosctl pcap -i eth0 --bpf-filter "$(tcpdump -dd -y EN10MB 'tcp and dst port 80')"

   talosctl pcap -i kubespan --bpf-filter "$(tcpdump -dd -y RAW 'port 50000')"

As packet capture is transmitted over the network, it is recommended to filter out the Talos API traffic,
e.g. by excluding packets with the port 50000.
   `,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			if err := helpers.FailIfMultiNodes(ctx, "pcap"); err != nil {
				return err
			}

			if pcapCmdFlags.duration > 0 {
				var cancel context.CancelFunc

				ctx, cancel = context.WithTimeout(ctx, pcapCmdFlags.duration)
				defer cancel()
			}

			req := machine.PacketCaptureRequest{
				Interface:   pcapCmdFlags.iface,
				Promiscuous: pcapCmdFlags.promisc,
				SnapLen:     uint32(pcapCmdFlags.snaplen),
			}

			var err error

			req.BpfFilter, err = parseBPFInstructions(pcapCmdFlags.bpfFilter)
			if err != nil {
				return err
			}

			r, errCh, err := c.PacketCapture(ctx, &req)
			if err != nil {
				return fmt.Errorf("error copying: %w", err)
			}

			var wg sync.WaitGroup

			wg.Add(1)
			go func() {
				defer wg.Done()
				for err := range errCh {
					if client.StatusCode(err) == codes.DeadlineExceeded {
						continue
					}

					fmt.Fprintln(os.Stderr, err.Error())
				}
			}()

			defer wg.Wait()

			if pcapCmdFlags.output == "" {
				return dumpPackets(ctx, r)
			}

			var out io.Writer

			if pcapCmdFlags.output == "-" {
				out = os.Stdout
			} else {
				out, err = os.Create(pcapCmdFlags.output)
				if err != nil {
					return err
				}
			}

			_, err = io.Copy(out, r)

			if errors.Is(err, io.EOF) || client.StatusCode(err) == codes.DeadlineExceeded {
				err = nil
			}

			return err
		})
	},
}

func dumpPackets(ctx context.Context, r io.Reader) error {
	src, err := pcapgo.NewReader(r)
	if err != nil {
		return fmt.Errorf("error opening pcap reader: %w", err)
	}

	packetSource := gopacket.NewPacketSource(src, src.LinkType())

	for packet := range packetSource.Packets() {
		fmt.Println(packet)
	}

	return nil
}

// parseBPFInstructions parses the BPF raw instructions in 'tcpdump -dd' format.
//
// Example:
//   { 0x30, 0, 0, 0x00000000 },
//   { 0x54, 0, 0, 0x000000f0 },
//   { 0x15, 0, 8, 0x00000060 },
func parseBPFInstructions(in string) ([]*machine.BPFInstruction, error) {
	in = strings.TrimSpace(in)

	if in == "" {
		return nil, nil
	}

	var result []*machine.BPFInstruction //nolint:prealloc

	for _, line := range strings.Split(in, "\n") {
		if line == "" {
			continue
		}

		ins := &machine.BPFInstruction{}

		n, err := fmt.Sscanf(line, "{ 0x%x, %d, %d, 0x%x },", &ins.Op, &ins.Jt, &ins.Jf, &ins.K)
		if err != nil {
			return nil, fmt.Errorf("error parsing bpf instruction %q: %w", line, err)
		}

		if n != 4 {
			return nil, fmt.Errorf("error parsing bpf instruction %q: expected 4 fields, got %d", line, n)
		}

		result = append(result, ins)
	}

	return result, nil
}

func init() {
	pcapCmd.Flags().StringVarP(&pcapCmdFlags.iface, "interface", "i", "eth0", "interface name to capture packets on")
	pcapCmd.Flags().BoolVar(&pcapCmdFlags.promisc, "promiscuous", false, "put interface into promiscuous mode")
	pcapCmd.Flags().IntVarP(&pcapCmdFlags.snaplen, "snaplen", "s", 65536, "maximum packet size to capture")
	pcapCmd.Flags().StringVarP(&pcapCmdFlags.output, "output", "o", "", "if not set, decode packets to stdout; if set write raw pcap data to a file, use '-' for stdout")
	pcapCmd.Flags().StringVar(&pcapCmdFlags.bpfFilter, "bpf-filter", "", "bpf filter to apply, tcpdump -dd format")
	pcapCmd.Flags().DurationVar(&pcapCmdFlags.duration, "duration", 0, "duration of the capture")
	addCommand(pcapCmd)
}
