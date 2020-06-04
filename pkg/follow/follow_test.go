// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package follow_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/pkg/follow"
)

type FollowSuite struct {
	suite.Suite

	tmpDir         string
	no             int
	reader, writer *os.File

	r *follow.Reader
}

func (suite *FollowSuite) SetupSuite() {
	var err error

	suite.tmpDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)
}

func (suite *FollowSuite) SetupTest() {
	suite.no++

	var err error

	suite.writer, err = os.Create(filepath.Join(suite.tmpDir, fmt.Sprintf("%d.log", suite.no)))
	suite.Require().NoError(err)

	suite.reader, err = os.Open(suite.writer.Name())
	suite.Require().NoError(err)
}

func (suite *FollowSuite) TearDownTest() {
	suite.Require().NoError(suite.writer.Close())

	suite.reader.Close() //nolint: errcheck
}

func (suite *FollowSuite) TearDownSuite() {
	suite.Require().NoError(os.RemoveAll(suite.tmpDir))
}

func (suite *FollowSuite) readAll(ctx context.Context, expectedError string) <-chan []byte {
	combinedCh := make(chan []byte)

	suite.r = follow.NewReader(ctx, suite.reader)

	go func() {
		defer suite.r.Close() //nolint: errcheck

		contents, err := ioutil.ReadAll(suite.r)

		if expectedError == "" {
			suite.Assert().NoError(err)
		} else {
			suite.Assert().EqualError(err, expectedError)
		}

		combinedCh <- contents
	}()

	return combinedCh
}

func (suite *FollowSuite) smallReadAll(ctx context.Context) <-chan []byte {
	combinedCh := make(chan []byte)

	go func() {
		suite.r = follow.NewReader(ctx, suite.reader)
		defer suite.r.Close() //nolint: errcheck

		buf := make([]byte, 1)

		var output bytes.Buffer

		_, err := io.CopyBuffer(&output, suite.r, buf)

		suite.Assert().NoError(err)

		combinedCh <- output.Bytes()
	}()

	return combinedCh
}

func (suite *FollowSuite) TestStreaming() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	combinedCh := suite.readAll(ctx, "")

	// nolint: errcheck
	suite.writer.WriteString("abc")
	// nolint: errcheck
	suite.writer.WriteString("def")
	// nolint: errcheck
	suite.writer.WriteString("ghi")
	time.Sleep(50 * time.Millisecond)
	// nolint: errcheck
	suite.writer.WriteString("jkl")
	// nolint: errcheck
	suite.writer.WriteString("mno")
	time.Sleep(50 * time.Millisecond)

	ctxCancel()

	suite.Require().Equal([]byte("abcdefghijklmno"), <-combinedCh)
}

func (suite *FollowSuite) TestStreamingClose() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	combinedCh := suite.readAll(ctx, "")

	// nolint: errcheck
	suite.writer.WriteString("abc")
	// nolint: errcheck
	suite.writer.WriteString("def")
	// nolint: errcheck
	suite.writer.WriteString("ghi")
	time.Sleep(50 * time.Millisecond)
	// nolint: errcheck
	suite.writer.WriteString("jkl")
	// nolint: errcheck
	suite.writer.WriteString("mno")
	time.Sleep(50 * time.Millisecond)

	suite.Require().NoError(suite.r.Close())

	suite.Require().Equal([]byte("abcdefghijklmno"), <-combinedCh)
}

func (suite *FollowSuite) TestStreamingWithSomeHead() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	// nolint: errcheck
	suite.writer.WriteString("abc")
	// nolint: errcheck
	suite.writer.WriteString("def")

	combinedCh := suite.readAll(ctx, "")

	// nolint: errcheck
	suite.writer.WriteString("ghi")
	time.Sleep(50 * time.Millisecond)
	// nolint: errcheck
	suite.writer.WriteString("jkl")
	time.Sleep(50 * time.Millisecond)
	// nolint: errcheck
	suite.writer.WriteString("mno")
	time.Sleep(50 * time.Millisecond)

	ctxCancel()

	suite.Require().Equal([]byte("abcdefghijklmno"), <-combinedCh)
}

func (suite *FollowSuite) TestStreamingSmallBuffer() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	combinedCh := suite.smallReadAll(ctx)

	// nolint: errcheck
	suite.writer.WriteString("abc")
	// nolint: errcheck
	suite.writer.WriteString("def")
	// nolint: errcheck
	suite.writer.WriteString("ghi")
	time.Sleep(50 * time.Millisecond)
	// nolint: errcheck
	suite.writer.WriteString("jkl")
	// nolint: errcheck
	suite.writer.WriteString("mno")

	// create extra file to try to confuse watch
	_, err := os.Create(filepath.Join(suite.tmpDir, "x.log"))
	suite.Require().NoError(err)

	time.Sleep(50 * time.Millisecond)

	ctxCancel()

	suite.Require().Equal([]byte("abcdefghijklmno"), <-combinedCh)
}

func (suite *FollowSuite) TestDeleted() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	combinedCh := suite.readAll(ctx, "file was removed while watching")

	// nolint: errcheck
	suite.writer.WriteString("abc")
	// nolint: errcheck
	suite.writer.WriteString("def")
	// nolint: errcheck
	suite.writer.WriteString("ghi")
	time.Sleep(50 * time.Millisecond)
	// nolint: errcheck
	suite.writer.WriteString("jkl")
	// nolint: errcheck
	suite.writer.WriteString("mno")
	time.Sleep(50 * time.Millisecond)

	// chunker should terminate when file is removed
	suite.Require().NoError(os.Remove(suite.writer.Name()))

	suite.Require().Equal([]byte("abcdefghijklmno"), <-combinedCh)
}

func (suite *FollowSuite) TestReadWrite() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	r := follow.NewReader(ctx, suite.reader)

	buf := make([]byte, 256)

	// nolint: errcheck
	suite.writer.WriteString("abc")

	n, err := r.Read(buf)
	suite.Require().NoError(err)
	suite.Require().Equal(3, n)
	suite.Require().Equal([]byte("abc"), buf[:n])

	// nolint: errcheck
	suite.writer.WriteString("def")

	n, err = r.Read(buf)
	suite.Require().NoError(err)
	suite.Require().Equal(3, n)
	suite.Require().Equal([]byte("def"), buf[:n])

	ch := make(chan []byte)

	go func() {
		n, err = r.Read(buf)
		suite.Require().NoError(err)
		suite.Require().Equal(3, n)

		ch <- buf[:n]
	}()

	// Read should block on no new data
	select {
	case <-ch:
		suite.Require().Fail("should block on read")
	case <-time.After(50 * time.Millisecond):
	}

	// nolint: errcheck
	suite.writer.WriteString("ghi")
	suite.Require().Equal([]byte("ghi"), <-ch)
}

func TestFollowSuite(t *testing.T) {
	suite.Run(t, new(FollowSuite))
}
