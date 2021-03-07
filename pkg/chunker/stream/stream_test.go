// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package stream_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/pkg/chunker/stream"
)

type StreamChunkerSuite struct {
	suite.Suite

	reader *io.PipeReader
	writer *io.PipeWriter
}

func (suite *StreamChunkerSuite) SetupTest() {
	suite.reader, suite.writer = io.Pipe()
}

func (suite *StreamChunkerSuite) TearDownTest() {
	suite.Require().NoError(suite.writer.Close())
	suite.Require().NoError(suite.reader.Close())
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

func (suite *StreamChunkerSuite) TestStreaming() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	chunker := stream.NewChunker(ctx, suite.reader)

	chunksCh := chunker.Read()
	combinedCh := collectChunks(chunksCh)

	//nolint:errcheck
	suite.writer.Write([]byte("abc"))
	//nolint:errcheck
	suite.writer.Write([]byte("def"))
	//nolint:errcheck
	suite.writer.Write([]byte("ghi"))
	time.Sleep(50 * time.Millisecond)
	//nolint:errcheck
	suite.writer.Write([]byte("jkl"))
	//nolint:errcheck
	suite.writer.Write([]byte("mno"))

	suite.Require().NoError(suite.writer.Close())

	suite.Require().Equal([]byte("abcdefghijklmno"), <-combinedCh)
}

func (suite *StreamChunkerSuite) TestStreamingSmallBuf() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	chunker := stream.NewChunker(ctx, suite.reader, stream.Size(1))

	chunksCh := chunker.Read()
	combinedCh := collectChunks(chunksCh)

	//nolint:errcheck
	suite.writer.Write([]byte("abc"))
	//nolint:errcheck
	suite.writer.Write([]byte("def"))
	//nolint:errcheck
	suite.writer.Write([]byte("ghi"))
	time.Sleep(50 * time.Millisecond)
	//nolint:errcheck
	suite.writer.Write([]byte("jkl"))
	//nolint:errcheck
	suite.writer.Write([]byte("mno"))

	suite.Require().NoError(suite.writer.Close())

	suite.Require().Equal([]byte("abcdefghijklmno"), <-combinedCh)
}

func (suite *StreamChunkerSuite) TestStreamingCancel() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	chunker := stream.NewChunker(ctx, suite.reader)

	chunksCh := chunker.Read()
	combinedCh := collectChunks(chunksCh)

	//nolint:errcheck
	suite.writer.Write([]byte("abc"))
	//nolint:errcheck
	suite.writer.Write([]byte("def"))
	//nolint:errcheck
	suite.writer.Write([]byte("ghi"))
	time.Sleep(50 * time.Millisecond)
	//nolint:errcheck
	suite.writer.Write([]byte("jkl"))
	//nolint:errcheck
	suite.writer.Write([]byte("mno"))
	time.Sleep(50 * time.Millisecond)

	ctxCancel()

	// need any I/O for chunker to notice that context got canceled
	//nolint:errcheck
	suite.writer.Write([]byte(""))

	suite.Require().Equal([]byte("abcdefghijklmno"), <-combinedCh)
}

func TestStreamChunkerSuite(t *testing.T) {
	suite.Run(t, new(StreamChunkerSuite))
}
