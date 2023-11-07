// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package pcap implements writing packet data to pcap files.
package pcap

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/gopacket/gopacket"
)

// Writer wraps an underlying io.Writer to write packet data in PCAP
// format.  See http://wiki.wireshark.org/Development/LibpcapFileFormat
// for information on the file format.
//
// For those that care, we currently write v2.4 files with nanosecond
// or microsecond timestamp resolution and little-endian encoding.
type Writer struct {
	w   io.Writer
	buf [16]byte
}

const (
	magicNanoseconds = 0xA1B23C4D
	versionMajor     = 2
	versionMinor     = 4
)

// LinkType is the link type for the pcap file.
type LinkType uint32

// LinkType values.
const (
	LinkTypeEthernet LinkType = 1
	LinkTypeRaw      LinkType = 101
)

// NewWriter returns a new writer object.
//
// If this is a new empty writer (as opposed to
// an append), you must call WriteFileHeader before WritePacket.  Packet
// timestamps are written with nanosecond precision.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// WriteFileHeader writes a file header out to the writer.
// This must be called exactly once per output.
func (w *Writer) WriteFileHeader(snaplen uint32, linktype LinkType) error {
	var buf [24]byte

	binary.LittleEndian.PutUint32(buf[0:4], magicNanoseconds)
	binary.LittleEndian.PutUint16(buf[4:6], versionMajor)
	binary.LittleEndian.PutUint16(buf[6:8], versionMinor)

	// bytes 8:12 stay 0 (timezone = UTC)
	// bytes 12:16 stay 0 (sigfigs is always set to zero, according to
	//   http://wiki.wireshark.org/Development/LibpcapFileFormat
	binary.LittleEndian.PutUint32(buf[16:20], snaplen)
	binary.LittleEndian.PutUint32(buf[20:24], uint32(linktype))

	_, err := w.w.Write(buf[:])

	return err
}

func (w *Writer) writePacketHeader(ci gopacket.CaptureInfo) error {
	t := ci.Timestamp
	if t.IsZero() {
		t = time.Now()
	}

	secs := t.Unix()
	binary.LittleEndian.PutUint32(w.buf[0:4], uint32(secs))

	usecs := t.Nanosecond()
	binary.LittleEndian.PutUint32(w.buf[4:8], uint32(usecs))

	binary.LittleEndian.PutUint32(w.buf[8:12], uint32(ci.CaptureLength))
	binary.LittleEndian.PutUint32(w.buf[12:16], uint32(ci.Length))

	_, err := w.w.Write(w.buf[:])

	return err
}

// WritePacket writes the given packet data out to the file.
func (w *Writer) WritePacket(ci gopacket.CaptureInfo, data []byte) error {
	if ci.CaptureLength != len(data) {
		return fmt.Errorf("capture length %d does not match data length %d", ci.CaptureLength, len(data))
	}

	if ci.CaptureLength > ci.Length {
		return fmt.Errorf("invalid capture info %+v:  capture length > length", ci)
	}

	if err := w.writePacketHeader(ci); err != nil {
		return fmt.Errorf("error writing packet header: %v", err)
	}

	_, err := w.w.Write(data)

	return err
}
