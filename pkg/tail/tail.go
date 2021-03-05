// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package tail

import (
	"bytes"
	"fmt"
	"io"
)

// Window is the read window size for tail scanning.
const Window = 4096

// SeekLines seeks the passed io.ReadSeeker so that it's -N lines from the tail.
//
// SeekLines might modify file offset even in case of error.
//
//nolint:gocyclo
func SeekLines(r io.ReadSeeker, lines int) error {
	offset, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	readOffset := offset - Window
	if readOffset < 0 {
		readOffset = 0
	}

	readSize := offset - readOffset

	skippedLines := -1 // we need to skip (lines + 1) \n characters to find position to read from

	buf := make([]byte, Window)
	firstRead := true

	for skippedLines < lines && readSize > 0 {
		_, err = r.Seek(readOffset, io.SeekStart)
		if err != nil {
			return err
		}

		var n int

		n, err = r.Read(buf[:readSize])
		if err != nil {
			return err
		}

		if int64(n) != readSize {
			return fmt.Errorf("unexpected short read: %d != %d", n, readSize)
		}

		if firstRead && buf[n-1] != '\n' {
			// last line might not have '\n'
			skippedLines++
		}

		firstRead = false

		for n > 0 && skippedLines < lines {
			index := bytes.LastIndexByte(buf[:n], '\n')
			if index == -1 {
				break
			}

			skippedLines++

			n = index
		}

		if skippedLines == lines {
			readOffset += int64(n) + 1

			break
		}

		offset = readOffset
		readOffset -= Window

		if readOffset < 0 {
			readOffset = 0
		}

		readSize = offset - readOffset
	}

	_, err = r.Seek(readOffset, io.SeekStart)

	return err
}
