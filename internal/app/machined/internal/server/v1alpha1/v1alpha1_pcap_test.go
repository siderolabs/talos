// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"bytes"
	"io"
	"net"
	"testing"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/afpacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/bpf"

	runtime "github.com/siderolabs/talos/internal/app/machined/internal/server/v1alpha1"
	"github.com/siderolabs/talos/internal/pkg/pcap"
)

type captureStep struct {
	data []byte
	ci   gopacket.CaptureInfo
	err  error
}

// fakeCaptureHandle replays a fixed sequence of [afpacket.TPacket.ZeroCopyReadPacketData] results.
type fakeCaptureHandle struct {
	steps  []captureStep
	closed bool
}

// check that the fake implements the same interface as the real handle.
var (
	_ runtime.PacketCaptureHandle = (*fakeCaptureHandle)(nil)
	_ runtime.PacketCaptureHandle = (*afpacket.TPacket)(nil)
)

func (h *fakeCaptureHandle) ZeroCopyReadPacketData() ([]byte, gopacket.CaptureInfo, error) {
	if len(h.steps) == 0 {
		// unrecoverable error terminates the capture
		return nil, gopacket.CaptureInfo{}, io.EOF
	}

	step := h.steps[0]
	h.steps = h.steps[1:]

	return step.data, step.ci, step.err
}

func (h *fakeCaptureHandle) Stats() (afpacket.Stats, error) {
	return afpacket.Stats{}, nil
}

func (h *fakeCaptureHandle) SocketStats() (afpacket.SocketStats, afpacket.SocketStatsV3, error) {
	return afpacket.SocketStats{}, afpacket.SocketStatsV3{}, nil
}

func (h *fakeCaptureHandle) Close() {
	h.closed = true
}

func buildFrame(t *testing.T, vlanID uint16) []byte {
	t.Helper()

	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0x00, 0x50, 0x56, 0x8f, 0xa5, 0xcb},
		DstMAC:       net.HardwareAddr{0x00, 0x50, 0x56, 0x8f, 0xd0, 0xc3},
		EthernetType: layers.EthernetTypeIPv4,
	}

	ip := &layers.IPv4{
		Version:  4,
		TTL:      64,
		Protocol: layers.IPProtocolICMPv4,
		SrcIP:    net.IPv4(11, 0, 1, 1),
		DstIP:    net.IPv4(11, 0, 1, 254),
	}

	icmp := &layers.ICMPv4{
		TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
		Id:       1,
		Seq:      17,
	}

	serializable := []gopacket.SerializableLayer{eth, ip, icmp, gopacket.Payload(bytes.Repeat([]byte{0xaa}, 56))}

	if vlanID != 0 {
		eth.EthernetType = layers.EthernetTypeDot1Q

		serializable = append(
			[]gopacket.SerializableLayer{eth, &layers.Dot1Q{VLANIdentifier: vlanID, Type: layers.EthernetTypeIPv4}},
			serializable[1:]...,
		)
	}

	buf := gopacket.NewSerializeBuffer()

	require.NoError(t, gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, serializable...))

	return buf.Bytes()
}

// TestCapturePacketsVLAN verifies that a frame with the VLAN header re-inserted by afpacket is captured.
//
// afpacket re-inserts the VLAN header stripped by the kernel into the packet data, but reports the wire length
// as seen by the kernel, i.e. 4 bytes short, so the capture loop has to compensate for that.
func TestCapturePacketsVLAN(t *testing.T) {
	t.Parallel()

	untagged := buildFrame(t, 0)
	tagged := buildFrame(t, 2005)

	require.Len(t, tagged, len(untagged)+4)

	handle := &fakeCaptureHandle{
		steps: []captureStep{
			{
				data: untagged,
				ci:   gopacket.CaptureInfo{CaptureLength: len(untagged), Length: len(untagged)},
			},
			{
				// poll timeouts are retried, not fatal
				err: afpacket.ErrTimeout,
			},
			{
				data: tagged,
				// the kernel doesn't count the stripped VLAN header in the wire length
				ci: gopacket.CaptureInfo{CaptureLength: len(tagged), Length: len(tagged) - 4},
			},
		},
	}

	var out bytes.Buffer

	err := runtime.CapturePackets(t.Context(), &out, handle, afpacket.DefaultFrameSize, pcap.LinkTypeEthernet)
	require.ErrorIs(t, err, io.EOF)
	assert.True(t, handle.closed)

	reader, err := pcapgo.NewReader(&out)
	require.NoError(t, err)

	data, ci, err := reader.ReadPacketData()
	require.NoError(t, err)
	assert.Equal(t, untagged, data)
	assert.Equal(t, len(untagged), ci.CaptureLength)
	assert.Equal(t, len(untagged), ci.Length)

	data, ci, err = reader.ReadPacketData()
	require.NoError(t, err)
	assert.Equal(t, tagged, data)
	assert.Equal(t, len(tagged), ci.CaptureLength)
	assert.Equal(t, len(tagged), ci.Length)

	packet := gopacket.NewPacket(data, layers.LinkTypeEthernet, gopacket.Default)

	dot1q, ok := packet.Layer(layers.LayerTypeDot1Q).(*layers.Dot1Q)
	require.True(t, ok, "no Dot1Q layer in %s", packet)
	assert.Equal(t, uint16(2005), dot1q.VLANIdentifier)

	_, _, err = reader.ReadPacketData()
	assert.ErrorIs(t, err, io.EOF)
}

// fakeFilterHandle records the filters attached to it and replays the frames queued before the filter.
type fakeFilterHandle struct {
	queued  [][]byte
	filters [][]bpf.RawInstruction
	// queuedAtSet is the number of queued frames at the time each filter was attached
	queuedAtSet []int
	readErr     error
}

func (h *fakeFilterHandle) SetBPF(filter []bpf.RawInstruction) error {
	h.filters = append(h.filters, filter)
	h.queuedAtSet = append(h.queuedAtSet, len(h.queued))

	return nil
}

func (h *fakeFilterHandle) ZeroCopyReadPacketData() ([]byte, gopacket.CaptureInfo, error) {
	if h.readErr != nil {
		return nil, gopacket.CaptureInfo{}, h.readErr
	}

	if len(h.queued) == 0 {
		return nil, gopacket.CaptureInfo{}, afpacket.ErrTimeout
	}

	data := h.queued[0]
	h.queued = h.queued[1:]

	return data, gopacket.CaptureInfo{CaptureLength: len(data), Length: len(data)}, nil
}

var _ runtime.PacketCaptureFilterHandle = (*afpacket.TPacket)(nil)

// TestSetPacketCaptureFilterDrainsQueuedFrames verifies that frames queued before the filter
// is attached are discarded, so they don't end up in a filtered capture.
func TestSetPacketCaptureFilterDrainsQueuedFrames(t *testing.T) {
	t.Parallel()

	filter, err := bpf.Assemble([]bpf.Instruction{bpf.RetConstant{Val: 0xffff}})
	require.NoError(t, err)

	rejectAll, err := bpf.Assemble([]bpf.Instruction{bpf.RetConstant{Val: 0}})
	require.NoError(t, err)

	handle := &fakeFilterHandle{
		queued: [][]byte{buildFrame(t, 0), buildFrame(t, 0), buildFrame(t, 10)},
	}

	require.NoError(t, runtime.SetPacketCaptureFilter(handle, filter))

	// reject-all filter first, then the requested one once the queue is empty
	assert.Equal(t, [][]bpf.RawInstruction{rejectAll, filter}, handle.filters)
	assert.Equal(t, []int{3, 0}, handle.queuedAtSet)
	assert.Empty(t, handle.queued)
}

func TestSetPacketCaptureFilterReadError(t *testing.T) {
	t.Parallel()

	filter, err := bpf.Assemble([]bpf.Instruction{bpf.RetConstant{Val: 0xffff}})
	require.NoError(t, err)

	handle := &fakeFilterHandle{readErr: afpacket.ErrPoll}

	err = runtime.SetPacketCaptureFilter(handle, filter)
	require.ErrorIs(t, err, afpacket.ErrPoll)

	// the requested filter is not attached, the caller closes the handle
	assert.Len(t, handle.filters, 1)
}
