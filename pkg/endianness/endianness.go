// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package endianness

import (
	"bytes"
	"encoding/binary"
)

// ToMiddleEndian converts a byte slice representation of a UUID to a
// middle-endian byte slice representation of a UUID.
//
//nolint: dupl
func ToMiddleEndian(data []byte) (b []byte, err error) {
	buf := bytes.NewBuffer(make([]byte, 0, 16))

	timeLow := binary.BigEndian.Uint32(data[0:4])
	if err := binary.Write(buf, binary.LittleEndian, timeLow); err != nil {
		return nil, err
	}

	timeMid := binary.BigEndian.Uint16(data[4:6])
	if err := binary.Write(buf, binary.LittleEndian, timeMid); err != nil {
		return nil, err
	}

	timeHigh := binary.BigEndian.Uint16(data[6:8])
	if err := binary.Write(buf, binary.LittleEndian, timeHigh); err != nil {
		return nil, err
	}

	clockSeqHi := data[8:9][0]
	if err := binary.Write(buf, binary.BigEndian, clockSeqHi); err != nil {
		return nil, err
	}

	clockSeqLow := data[9:10][0]
	if err := binary.Write(buf, binary.BigEndian, clockSeqLow); err != nil {
		return nil, err
	}

	node := data[10:16]
	if err := binary.Write(buf, binary.BigEndian, node); err != nil {
		return nil, err
	}

	b = buf.Bytes()

	return b, nil
}

// FromMiddleEndian converts a middle-endian byte slice representation of a
// UUID to a big-endian byte slice representation of a UUID.
//
//nolint: dupl
func FromMiddleEndian(data []byte) (b []byte, err error) {
	buf := bytes.NewBuffer(make([]byte, 0, 16))

	timeLow := binary.LittleEndian.Uint32(data[0:4])
	if err := binary.Write(buf, binary.BigEndian, timeLow); err != nil {
		return nil, err
	}

	timeMid := binary.LittleEndian.Uint16(data[4:6])
	if err := binary.Write(buf, binary.BigEndian, timeMid); err != nil {
		return nil, err
	}

	timeHigh := binary.LittleEndian.Uint16(data[6:8])
	if err := binary.Write(buf, binary.BigEndian, timeHigh); err != nil {
		return nil, err
	}

	clockSeqHi := data[8:9][0]
	if err := binary.Write(buf, binary.BigEndian, clockSeqHi); err != nil {
		return nil, err
	}

	clockSeqLow := data[9:10][0]
	if err := binary.Write(buf, binary.BigEndian, clockSeqLow); err != nil {
		return nil, err
	}

	node := data[10:16]
	if err := binary.Write(buf, binary.BigEndian, node); err != nil {
		return nil, err
	}

	b = buf.Bytes()

	return b, nil
}
