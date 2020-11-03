// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package endianness

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// ToMiddleEndian converts a byte slice representation of a UUID to a
// middle-endian byte slice representation of a UUID.
//
//nolint: dupl
func ToMiddleEndian(data []byte) (b []byte, err error) {
	buf := bytes.NewBuffer(make([]byte, 0, 16))

	timeLow := binary.BigEndian.Uint32(data[0:4])
	if err := binary.Write(buf, binary.LittleEndian, timeLow); err != nil {
		return nil, fmt.Errorf("failed to write time low: %w", err)
	}

	timeMid := binary.BigEndian.Uint16(data[4:6])
	if err := binary.Write(buf, binary.LittleEndian, timeMid); err != nil {
		return nil, fmt.Errorf("failed to write time mid: %w", err)
	}

	timeHigh := binary.BigEndian.Uint16(data[6:8])
	if err := binary.Write(buf, binary.LittleEndian, timeHigh); err != nil {
		return nil, fmt.Errorf("failed to write time high: %w", err)
	}

	clockSeqHi := data[8:9][0]
	if err := binary.Write(buf, binary.BigEndian, clockSeqHi); err != nil {
		return nil, fmt.Errorf("failed to write clock seq hi: %w", err)
	}

	clockSeqLow := data[9:10][0]
	if err := binary.Write(buf, binary.BigEndian, clockSeqLow); err != nil {
		return nil, fmt.Errorf("failed to write clock seq low: %w", err)
	}

	node := data[10:16]
	if err := binary.Write(buf, binary.BigEndian, node); err != nil {
		return nil, fmt.Errorf("failed to write time node: %w", err)
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
		return nil, fmt.Errorf("failed to read time low: %w", err)
	}

	timeMid := binary.LittleEndian.Uint16(data[4:6])
	if err := binary.Write(buf, binary.BigEndian, timeMid); err != nil {
		return nil, fmt.Errorf("failed to read time mid: %w", err)
	}

	timeHigh := binary.LittleEndian.Uint16(data[6:8])
	if err := binary.Write(buf, binary.BigEndian, timeHigh); err != nil {
		return nil, fmt.Errorf("failed to read time low: %w", err)
	}

	clockSeqHi := data[8:9][0]
	if err := binary.Write(buf, binary.BigEndian, clockSeqHi); err != nil {
		return nil, fmt.Errorf("failed to read clock seq hi: %w", err)
	}

	clockSeqLow := data[9:10][0]
	if err := binary.Write(buf, binary.BigEndian, clockSeqLow); err != nil {
		return nil, fmt.Errorf("failed to read clock seq low: %w", err)
	}

	node := data[10:16]
	if err := binary.Write(buf, binary.BigEndian, node); err != nil {
		return nil, fmt.Errorf("failed to read clock seq node: %w", err)
	}

	b = buf.Bytes()

	return b, nil
}
