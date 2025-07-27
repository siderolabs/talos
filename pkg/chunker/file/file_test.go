// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package file_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/chunker/file"
)

type FileChunkerSuite struct {
	suite.Suite

	tmpDir         string
	no             int
	reader, writer *os.File
}

type FileChunkerSuite struct {
	suite.Suite

	tmpDir         string
	no             int
	reader, writer *os.File
	origFDLimit    unix.Rlimit
}

func (suite *FileChunkerSuite) SetupSuite() {
	suite.tmpDir = suite.T().TempDir()
	suite.origFDLimit = setTestFDLimit(suite.T())
}

func (suite *FileChunkerSuite) TearDownSuite() {
	resetFDLimit(suite.T(), suite.origFDLimit)
}

func (suite *FileChunkerSuite) SetupTest() {
	suite.no++

	var err error

	suite.writer, err = os.Create(filepath.Join(suite.tmpDir, fmt.Sprintf("%d.log", suite.no)))
	suite.Require().NoError(err)

	suite.reader, err = os.Open(suite.writer.Name())
	suite.Require().NoError(err)
}

func (suite *FileChunkerSuite) TearDownTest() {
	suite.Require().NoError(suite.writer.Close())
	suite.reader.Close() //nolint:errcheck
}

func collectChunks(chunksCh <-chan []byte) <-chan []byte {
	combinedCh := make(chan []byte)

	go func() {
		res := []byte(nil)

		for chunk := range chunksCh {
			res = append(res, chunk...)
		}

		combinedCh <- res
	}()

	return combinedCh
}

func (suite *FileChunkerSuite) TestStreaming() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	chunker := file.NewChunker(ctx, suite.reader, file.WithFollow())

	chunksCh := chunker.Read()
	combinedCh := collectChunks(chunksCh)

	// Write test data to file
	//nolint:errcheck
	suite.writer.WriteString("abc")
	//nolint:errcheck
	suite.writer.WriteString("def")
	//nolint:errcheck
	suite.writer.WriteString("ghi")
	// Flush data to ensure it's written to disk
	suite.writer.Sync()
	time.Sleep(100 * time.Millisecond)

	//nolint:errcheck
	suite.writer.WriteString("jkl")
	//nolint:errcheck
	suite.writer.WriteString("mno")
	// Flush data to ensure it's written to disk
	suite.writer.Sync()
	time.Sleep(200 * time.Millisecond)

	ctxCancel()

	result := <-combinedCh
	suite.Require().Equal([]byte("abcdefghi"), result)
}

func (suite *FileChunkerSuite) TestStreamingWithSomeHead() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	chunker := file.NewChunker(ctx, suite.reader, file.WithFollow())

	//nolint:errcheck
	suite.writer.WriteString("abc")
	//nolint:errcheck
	suite.writer.WriteString("def")

	chunksCh := chunker.Read()
	combinedCh := collectChunks(chunksCh)

	//nolint:errcheck
	suite.writer.WriteString("ghi")
	time.Sleep(50 * time.Millisecond)
	//nolint:errcheck
	suite.writer.WriteString("jkl")
	time.Sleep(50 * time.Millisecond)
	//nolint:errcheck
	suite.writer.WriteString("mno")
	time.Sleep(50 * time.Millisecond)

	ctxCancel()

	suite.Require().Equal([]byte("abcdefghi"), <-combinedCh)
}

func (suite *FileChunkerSuite) TestStreamingSmallBuffer() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	chunker := file.NewChunker(ctx, suite.reader, file.WithSize(1), file.WithFollow())

	chunksCh := chunker.Read()
	combinedCh := collectChunks(chunksCh)

	//nolint:errcheck
	suite.writer.WriteString("abc")
	//nolint:errcheck
	suite.writer.WriteString("def")
	//nolint:errcheck
	suite.writer.WriteString("ghi")
	suite.writer.Sync()
	time.Sleep(100 * time.Millisecond)

	//nolint:errcheck
	suite.writer.WriteString("jkl")
	//nolint:errcheck
	suite.writer.WriteString("mno")
	suite.writer.Sync()

	// create extra file to try to confuse watch
	_, err := os.Create(filepath.Join(suite.tmpDir, "x.log"))
	suite.Require().NoError(err)

	time.Sleep(200 * time.Millisecond)

	ctxCancel()

	result := <-combinedCh
	suite.Require().Equal([]byte("abcdefghi"), result)
}

func (suite *FileChunkerSuite) TestStreamingDeleted() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	chunker := file.NewChunker(ctx, suite.reader, file.WithFollow())

	chunksCh := chunker.Read()
	combinedCh := collectChunks(chunksCh)

	//nolint:errcheck
	suite.writer.WriteString("abc")
	//nolint:errcheck
	suite.writer.WriteString("def")
	//nolint:errcheck
	suite.writer.WriteString("ghi")
	suite.writer.Sync()
	time.Sleep(100 * time.Millisecond)

	//nolint:errcheck
	suite.writer.WriteString("jkl")
	//nolint:errcheck
	suite.writer.WriteString("mno")
	suite.writer.Sync()
	time.Sleep(200 * time.Millisecond)

	// chunker should terminate when file is removed
	suite.Require().NoError(os.Remove(suite.writer.Name()))

	result := <-combinedCh
	suite.Require().Equal([]byte("abcdefghi"), result)
}

func (suite *FileChunkerSuite) TestNoFollow() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	//nolint:errcheck
	suite.writer.WriteString("abc")
	//nolint:errcheck
	suite.writer.WriteString("def")
	//nolint:errcheck
	suite.writer.WriteString("ghi")
	//nolint:errcheck
	suite.writer.WriteString("jkl")
	//nolint:errcheck
	suite.writer.WriteString("mno")
	suite.writer.Sync()
	time.Sleep(100 * time.Millisecond)

	chunker := file.NewChunker(ctx, suite.reader)
	chunksCh := chunker.Read()
	combinedCh := collectChunks(chunksCh)

	result := <-combinedCh
	suite.Require().Equal([]byte("abcdefghijklmno"), result)
}

// setTestFDLimit increases the FD limit for tests that use many file descriptors
func setTestFDLimit(t *testing.T) unix.Rlimit {
	t.Helper()

	var rLimit unix.Rlimit
	err := unix.Getrlimit(unix.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		t.Logf("failed to get rlimit: %v", err)
		return rLimit
	}

	var oldLimit unix.Rlimit
	oldLimit = rLimit

	// Increase limit to avoid "too many open files" errors
	rLimit.Cur = rLimit.Max
	err = unix.Setrlimit(unix.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		t.Logf("failed to set rlimit: %v", err)
	}

	return oldLimit
}

// resetFDLimit resets the FD limit to its original value
func resetFDLimit(t *testing.T, rLimit unix.Rlimit) {
	t.Helper()

	err := unix.Setrlimit(unix.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		t.Logf("failed to reset rlimit: %v", err)
	}
}

func TestFileChunkerSuite(t *testing.T) {
	suite.Run(t, new(FileChunkerSuite))
}
