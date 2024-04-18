// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/pcapgo"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
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

Raw pcap file can be saved with ` + "`--output`" + ` flag:

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
			}

			var err error

			req.BpfFilter, err = parseBPFInstructions(pcapCmdFlags.bpfFilter)
			if err != nil {
				return err
			}

			r, err := c.PacketCapture(ctx, &req)
			if err != nil {
				return fmt.Errorf("error copying: %w", err)
			}

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
				defer func() {
					if cerr := out.Close(); cerr != nil && err == nil {
						err = cerr
					}
				}()
			}

			_, err = io.Copy(out, r)

			if errors.Is(err, io.EOF) || client.StatusCode(err) == codes.DeadlineExceeded {
				err = nil
			}

			return err
		})
	},
}

// snapLength defines a snap length for the packet reading. For some reason
// TF_PACKET captures more than the snap length. Tools like tcpdump ignore snaplen entirely and set their own
// (https://github.com/the-tcpdump-group/tcpdump/blob/9fad826b0e487e8939325d62b7a461619b2722eb/netdissect.h#L342)
// so it makes sense to do the same.
const snapLength = 262144

func dumpPackets(ctx context.Context, r io.Reader) error {
	src, err := pcapgo.NewReader(r)
	if err != nil {
		if errors.Is(err, io.EOF) {
			// nothing in the capture at all
			return nil
		}

		return fmt.Errorf("error opening pcap reader: %w", err)
	}

	src.SetSnaplen(snapLength)

	forEachPacket(
		ctx,
		gopacket.NewZeroCopyPacketSource(src, src.LinkType(), gopacket.WithPool(true)),
		func(packet gopacket.Packet, err error) {
			switch err {
			case nil:
				fmt.Println(packet)
			default:
				fmt.Println("packet capture error:", err)
			}
		},
	)

	return nil
}

// parseBPFInstructions parses the BPF raw instructions in 'tcpdump -dd' format.
//
// Example:
//
//	{ 0x30, 0, 0, 0x00000000 },
//	{ 0x54, 0, 0, 0x000000f0 },
//	{ 0x15, 0, 8, 0x00000060 },
//
//nolint:dupword
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
	pcapCmd.Flags().IntVarP(&pcapCmdFlags.snaplen, "snaplen", "s", 4096, "maximum packet size to capture")
	pcapCmd.Flags().StringVarP(&pcapCmdFlags.output, "output", "o", "", "if not set, decode packets to stdout; if set write raw pcap data to a file, use '-' for stdout")
	pcapCmd.Flags().StringVar(&pcapCmdFlags.bpfFilter, "bpf-filter", "", "bpf filter to apply, tcpdump -dd format")
	pcapCmd.Flags().DurationVar(&pcapCmdFlags.duration, "duration", 0, "duration of the capture")
	pcapCmd.Flags().MarkDeprecated("snaplen", "support of snap length is removed") //nolint:errcheck

	addCommand(pcapCmd)
}

// forEachPacket reads packets from the packet source and calls the provided function for each packet. fn should not
// store the packet as it will be reused for the next packet. It will also call fn with nil packet and non nill
// error if the error is not known. If the context is canceled, the function will return as soon as
// [gopacket.PacketSource.NextPacket] returns.
//
// This function is more or less direct copy of [gopacket.PacketSource.PacketsCtx] minus the sleeps.
//
//nolint:gocyclo
func forEachPacket(ctx context.Context, p *gopacket.PacketSource, fn func(gopacket.Packet, error)) {
	for ctx.Err() == nil {
		packet, err := p.NextPacket()
		if err == nil {
			fn(packet, nil)

			if ctx.Err() != nil {
				break
			}

			// If we use pooled packets, we need to send them back to the pool
			if pooled, ok := packet.(gopacket.PooledPacket); ok {
				pooled.Dispose()
			}

			continue
		}

		// if timeout error -> retry
		var netErr net.Error
		if ok := errors.As(err, &netErr); ok && netErr.Timeout() {
			continue
		}

		// Immediately break for known unrecoverable errors
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
			errors.Is(err, io.ErrNoProgress) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, io.ErrShortBuffer) ||
			errors.Is(err, syscall.EBADF) ||
			strings.Contains(err.Error(), "use of closed file") {
			break
		}

		// Otherwise, send error to the caller
		fn(nil, err)

		// and try again if context is not canceled
		if ctx.Err() != nil {
			break
		}
	}
}
