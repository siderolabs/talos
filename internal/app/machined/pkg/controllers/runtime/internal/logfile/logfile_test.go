// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package logfile_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/logfile"
)

func TestWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	lf := logfile.NewLogFile(path, 1024)
	defer require.NoError(t, lf.Close())

	err := lf.Write([]byte("hello world"))
	require.NoError(t, err)

	// Expect write to retain data in the buffer
	st, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, int64(0), st.Size(), "file should be empty before flush")
	require.NoError(t, lf.Flush())

	// After flush, check the data got written to the file
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", string(content))
}

func TestWriteMultipleLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	lf := logfile.NewLogFile(path, 1024)
	defer require.NoError(t, lf.Close())

	lines := []string{"line1", "line2", "line3"}
	for _, line := range lines {
		require.NoError(t, lf.Write([]byte(line)))
	}

	require.NoError(t, lf.Flush())

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "line1\nline2\nline3\n", string(content))
}

func TestLogRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	expectedRotatedPath := path + ".1"

	lf := logfile.NewLogFile(path, 50)
	defer require.NoError(t, lf.Close())

	// We write 4 lines (indices 0-3)
	// expecting 0-2 to be written before rotation and 3 after rotation
	for i := range 4 {
		line := []byte("_20_character_line_" + strconv.Itoa(i))
		require.NoError(t, lf.Write(line))
	}

	_, err := os.Stat(expectedRotatedPath)
	require.NoError(t, err)

	// Verify the rotated file contains the written data
	rotatedContent, err := os.ReadFile(expectedRotatedPath)
	require.NoError(t, err)
	require.Len(t, rotatedContent, 63)
	require.Contains(t, string(rotatedContent), "_20_character_line_2")
	require.NoError(t, lf.Flush())

	currentContent, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Len(t, currentContent, 21)
	require.Contains(t, string(currentContent), "_20_character_line_3")
}

func TestLogRotationMultipleTimes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	rotatedPath := path + ".1"

	lf := logfile.NewLogFile(path, 40)
	defer require.NoError(t, lf.Close())

	for i := range 10 {
		line := []byte("_20_character_line_" + strconv.Itoa(i))
		require.NoError(t, lf.Write(line))
	}

	// Rotated file should exist and contain most recent events before the current
	rotatedContent, err := os.ReadFile(rotatedPath)
	require.NoError(t, err)
	require.Len(t, rotatedContent, 42)
	require.Contains(t, string(rotatedContent), "_20_character_line_8")
	require.Contains(t, string(rotatedContent), "_20_character_line_9")
}

func TestFlushWithoutFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	lf := logfile.NewLogFile(path, 1024)
	defer require.NoError(t, lf.Close())

	require.NoError(t, lf.Flush())
}

func TestClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	lf := logfile.NewLogFile(path, 1024)

	require.NoError(t, lf.Write([]byte("data")))

	err := lf.Close()
	require.NoError(t, err)

	// Expect Close to have flushed the buffer
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(content), "data")
}

func TestCloseWithoutWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	lf := logfile.NewLogFile(path, 1024)
	require.NoError(t, lf.Close())
}

func TestConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Do not rotate while the test runs
	lf := logfile.NewLogFile(path, 100000)
	defer require.NoError(t, lf.Close())

	var wg sync.WaitGroup

	numGoroutines := 10
	writesPerGoroutine := 100

	for range numGoroutines {
		wg.Go(func() {
			for range writesPerGoroutine {
				require.NoError(t, lf.Write([]byte("goroutine write")))
			}
		})
	}

	wg.Wait()

	require.NoError(t, lf.Flush())

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	// Count lines to verify all writes succeeded
	lineCount := bytes.Count(content, []byte("\n"))
	expectedLines := numGoroutines * writesPerGoroutine
	require.Equal(t, expectedLines, lineCount)
}

func TestConcurrentWriteAndFlush(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	lf := logfile.NewLogFile(path, 10000)
	defer require.NoError(t, lf.Close())

	var wg sync.WaitGroup

	// Writer goroutines
	for range 5 {
		wg.Go(func() {
			for range 50 {
				require.NoError(t, lf.Write([]byte("concurrent data")))
			}
		})
	}

	// Flusher goroutines
	for range 3 {
		wg.Go(func() {
			for range 10 {
				require.NoError(t, lf.Flush())
			}
		})
	}

	wg.Wait()

	require.NoError(t, lf.Flush())
}

func TestConcurrentWritesWithRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Small threshold to trigger rotation during concurrent writes
	lf := logfile.NewLogFile(path, 100)
	defer require.NoError(t, lf.Close())

	var wg sync.WaitGroup

	numGoroutines := 5
	writesPerGoroutine := 50

	for range numGoroutines {
		wg.Go(func() {
			for range writesPerGoroutine {
				require.NoError(t, lf.Write([]byte("rotation test line")))
			}
		})
	}

	wg.Wait()

	_, err := os.Stat(path + ".1")
	require.NoError(t, err)
}
