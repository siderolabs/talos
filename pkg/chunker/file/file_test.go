// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package file_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/pkg/chunker/file"
)

type FileChunkerSuite struct {
	suite.Suite

	tmpDir         string
	no             int
	reader, writer *os.File
}

func (suite *FileChunkerSuite) SetupSuite() {
	var err error

	suite.tmpDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)
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

func (suite *FileChunkerSuite) TearDownSuite() {
	suite.Require().NoError(os.RemoveAll(suite.tmpDir))
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

	//nolint:errcheck
	suite.writer.WriteString("abc")
	//nolint:errcheck
	suite.writer.WriteString("def")
	//nolint:errcheck
	suite.writer.WriteString("ghi")
	time.Sleep(50 * time.Millisecond)
	//nolint:errcheck
	suite.writer.WriteString("jkl")
	//nolint:errcheck
	suite.writer.WriteString("mno")
	time.Sleep(50 * time.Millisecond)

	ctxCancel()

	suite.Require().Equal([]byte("abcdefghijklmno"), <-combinedCh)
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

	suite.Require().Equal([]byte("abcdefghijklmno"), <-combinedCh)
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
	time.Sleep(50 * time.Millisecond)
	//nolint:errcheck
	suite.writer.WriteString("jkl")
	//nolint:errcheck
	suite.writer.WriteString("mno")

	// create extra file to try to confuse watch
	_, err := os.Create(filepath.Join(suite.tmpDir, "x.log"))
	suite.Require().NoError(err)

	time.Sleep(50 * time.Millisecond)

	ctxCancel()

	suite.Require().Equal([]byte("abcdefghijklmno"), <-combinedCh)
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
	time.Sleep(50 * time.Millisecond)
	//nolint:errcheck
	suite.writer.WriteString("jkl")
	//nolint:errcheck
	suite.writer.WriteString("mno")
	time.Sleep(50 * time.Millisecond)

	// chunker should terminate when file is removed
	suite.Require().NoError(os.Remove(suite.writer.Name()))

	suite.Require().Equal([]byte("abcdefghijklmno"), <-combinedCh)
}

func (suite *FileChunkerSuite) TestNoFollow() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	chunker := file.NewChunker(ctx, suite.reader)

	//nolint:errcheck
	suite.writer.WriteString("abc")
	//nolint:errcheck
	suite.writer.WriteString("def")
	//nolint:errcheck
	suite.writer.WriteString("ghi")
	time.Sleep(50 * time.Millisecond)

	chunksCh := chunker.Read()
	combinedCh := collectChunks(chunksCh)

	suite.Require().Equal([]byte("abcdefghi"), <-combinedCh)
}

func TestFileChunkerSuite(t *testing.T) {
	suite.Run(t, new(FileChunkerSuite))
}
