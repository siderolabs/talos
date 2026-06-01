// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package logfile implements a buffered, rotating log file.
package logfile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
)

// LogFile is an implementation of a buffered and rotated log file.
type LogFile struct {
	mu   sync.Mutex
	file *os.File
	buf  bufio.Writer

	path              string
	size              int64
	rotationThreshold int64
}

// NewLogFile creates a LogFile.
func NewLogFile(path string, rotationThreshold int64) *LogFile {
	return &LogFile{
		path:              path,
		rotationThreshold: rotationThreshold,
	}
}

// Write appends a line to the end of file, handling file creation and rotation.
func (lf *LogFile) Write(line []byte) error {
	var err error

	lf.mu.Lock()
	defer lf.mu.Unlock()

	if lf.file == nil {
		lf.file, err = os.OpenFile(lf.path, os.O_CREATE|os.O_WRONLY, 0o640)
		if err != nil {
			return fmt.Errorf("error opening log file %q: %w", lf.path, err)
		}

		lf.size, err = lf.file.Seek(0, io.SeekEnd)
		if err != nil {
			return fmt.Errorf("error determining log file %q length: %w", lf.path, err)
		}

		lf.buf.Reset(lf.file)
	}

	var n int
	if n, err = lf.buf.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("error writing log line to file %q: %w", lf.path, err)
	}

	lf.size += int64(n)
	if lf.size < lf.rotationThreshold {
		return nil
	}

	if err = lf.close(); err != nil {
		return err
	}

	if err = os.Rename(lf.path, lf.path+".1"); err != nil {
		return fmt.Errorf("error renaming log file %q: %w", lf.path, err)
	}

	return nil
}

func (lf *LogFile) flush() error {
	if err := lf.buf.Flush(); err != nil {
		return fmt.Errorf("failed to flush log file %s buffer: %w", lf.path, err)
	}

	return nil
}

// Flush flushes the internal buffer to persist data to the filesystem.
func (lf *LogFile) Flush() error {
	lf.mu.Lock()
	defer lf.mu.Unlock()

	return lf.flush()
}

func (lf *LogFile) close() error {
	if err := lf.flush(); err != nil {
		return err
	}

	lf.buf.Reset(nil)

	if lf.file == nil {
		return nil
	}

	err := lf.file.Close()
	lf.file = nil

	if err != nil {
		return fmt.Errorf("failed to close log file %s: %w", lf.path, err)
	}

	return nil
}

// Close flushes and closes the underlying file.
func (lf *LogFile) Close() error {
	lf.mu.Lock()
	defer lf.mu.Unlock()

	return lf.close()
}
