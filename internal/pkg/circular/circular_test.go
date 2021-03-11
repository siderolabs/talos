// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package circular_test

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"golang.org/x/time/rate"

	"github.com/talos-systems/talos/internal/pkg/circular"
)

type CircularSuite struct {
	suite.Suite
}

func (suite *CircularSuite) TestWrites() {
	buf, err := circular.NewBuffer(circular.WithInitialCapacity(2048), circular.WithMaxCapacity(100000))
	suite.Require().NoError(err)

	n, err := buf.Write(nil)
	suite.Require().NoError(err)
	suite.Require().Equal(0, n)

	n, err = buf.Write(make([]byte, 100))
	suite.Require().NoError(err)
	suite.Require().Equal(100, n)

	n, err = buf.Write(make([]byte, 1000))
	suite.Require().NoError(err)
	suite.Require().Equal(1000, n)

	suite.Require().Equal(2048, buf.Capacity())
	suite.Require().EqualValues(1100, buf.Offset())

	n, err = buf.Write(make([]byte, 5000))
	suite.Require().NoError(err)
	suite.Require().Equal(5000, n)

	suite.Require().Equal(8192, buf.Capacity())
	suite.Require().EqualValues(6100, buf.Offset())

	for i := 0; i < 20; i++ {
		l := 1 << i

		n, err = buf.Write(make([]byte, l))
		suite.Require().NoError(err)
		suite.Require().Equal(l, n)
	}

	suite.Require().Equal(100000, buf.Capacity())
	suite.Require().EqualValues(6100+(1<<20)-1, buf.Offset())
}

func (suite *CircularSuite) TestStreamingReadWriter() {
	buf, err := circular.NewBuffer(circular.WithInitialCapacity(2048), circular.WithMaxCapacity(65536))
	suite.Require().NoError(err)

	r := buf.GetStreamingReader()

	size := 1048576

	data := make([]byte, size)
	for i := range data {
		data[i] = byte(rand.Int31())
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(1)

	go func() {
		defer wg.Done()

		p := data

		r := rate.NewLimiter(300_000, 1000)

		for i := 0; i < len(data); {
			l := 100 + rand.Intn(100)

			if i+l > len(data) {
				l = len(data) - i
			}

			r.WaitN(context.Background(), l) //nolint:errcheck

			n, e := buf.Write(p[:l])
			suite.Require().NoError(e)
			suite.Require().Equal(l, n)

			i += l
			p = p[l:] //nolint:wastedassign
		}
	}()

	actual := make([]byte, size)

	n, err := io.ReadFull(r, actual)
	suite.Require().NoError(err)
	suite.Require().Equal(size, n)

	suite.Require().Equal(data, actual)

	s := make(chan struct{})

	go func() {
		_, err = r.Read(make([]byte, 1))

		suite.Assert().Equal(err, circular.ErrClosed)

		close(s)
	}()

	time.Sleep(50 * time.Millisecond) // wait for the goroutine to start

	suite.Require().NoError(r.Close())

	// close should abort reader
	<-s

	_, err = r.Read(nil)
	suite.Require().Equal(circular.ErrClosed, err)
}

func (suite *CircularSuite) TestStreamingMultipleReaders() {
	buf, err := circular.NewBuffer(circular.WithInitialCapacity(2048), circular.WithMaxCapacity(65536))
	suite.Require().NoError(err)

	n := 5

	readers := make([]*circular.StreamingReader, n)

	for i := 0; i < n; i++ {
		readers[i] = buf.GetStreamingReader()
	}

	size := 1048576

	data := make([]byte, size)
	for i := range data {
		data[i] = byte(rand.Int31())
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	for i := 0; i < n; i++ {
		wg.Add(1)

		i := i

		go func() {
			defer wg.Done()

			actual := make([]byte, size)

			nn, err := io.ReadFull(readers[i], actual)
			suite.Require().NoError(err)
			suite.Assert().Equal(size, nn)

			suite.Assert().Equal(data, actual)
		}()
	}

	p := data

	r := rate.NewLimiter(300_000, 1000)

	for i := 0; i < len(data); {
		l := 256

		if i+l > len(data) {
			l = len(data) - i
		}

		r.WaitN(context.Background(), l) //nolint:errcheck

		n, e := buf.Write(p[:l])
		suite.Require().NoError(e)
		suite.Require().Equal(l, n)

		i += l
		p = p[l:] //nolint:wastedassign
	}
}

func (suite *CircularSuite) TestStreamingLateAndIdleReaders() {
	buf, err := circular.NewBuffer(circular.WithInitialCapacity(2048), circular.WithMaxCapacity(65536), circular.WithSafetyGap(256))
	suite.Require().NoError(err)

	idleR := buf.GetStreamingReader()

	size := 100000

	data := make([]byte, size)
	for i := range data {
		data[i] = byte(rand.Int31())
	}

	n, err := buf.Write(data)
	suite.Require().NoError(err)
	suite.Require().Equal(size, n)

	lateR := buf.GetStreamingReader()

	go func() {
		time.Sleep(50 * time.Millisecond)

		suite.Require().NoError(lateR.Close())
	}()

	actual, err := ioutil.ReadAll(lateR)
	suite.Require().Equal(circular.ErrClosed, err)
	suite.Require().Equal(65536-256, len(actual))

	suite.Require().Equal(data[size-65536+256:], actual)

	go func() {
		time.Sleep(50 * time.Millisecond)

		suite.Require().NoError(idleR.Close())
	}()

	actual, err = ioutil.ReadAll(idleR)
	suite.Require().Equal(circular.ErrClosed, err)
	suite.Require().Equal(65536, len(actual))

	suite.Require().Equal(data[size-65536:], actual)
}

func (suite *CircularSuite) TestStreamingSeek() {
	buf, err := circular.NewBuffer(circular.WithInitialCapacity(2048), circular.WithMaxCapacity(65536), circular.WithSafetyGap(256))
	suite.Require().NoError(err)

	_, err = buf.Write(bytes.Repeat([]byte{0xff}, 512))
	suite.Require().NoError(err)

	r := buf.GetStreamingReader()

	_, err = buf.Write(bytes.Repeat([]byte{0xfe}, 512))
	suite.Require().NoError(err)

	off, err := r.Seek(0, io.SeekCurrent)
	suite.Require().NoError(err)
	suite.Assert().EqualValues(0, off)

	data := make([]byte, 256)

	n, err := r.Read(data)
	suite.Require().NoError(err)
	suite.Assert().Equal(256, n)
	suite.Assert().Equal(bytes.Repeat([]byte{0xff}, 256), data)

	off, err = r.Seek(0, io.SeekCurrent)
	suite.Require().NoError(err)
	suite.Assert().EqualValues(256, off)

	off, err = r.Seek(-256, io.SeekEnd)
	suite.Require().NoError(err)
	suite.Assert().EqualValues(768, off)

	n, err = r.Read(data)
	suite.Require().NoError(err)
	suite.Assert().Equal(256, n)
	suite.Assert().Equal(bytes.Repeat([]byte{0xfe}, 256), data)

	off, err = r.Seek(2048, io.SeekStart)
	suite.Require().NoError(err)
	suite.Assert().EqualValues(1024, off)

	_, err = buf.Write(bytes.Repeat([]byte{0xfe}, 65536-256))
	suite.Require().NoError(err)

	off, err = r.Seek(0, io.SeekStart)
	suite.Require().NoError(err)
	suite.Assert().EqualValues(1024, off)

	_, err = buf.Write(bytes.Repeat([]byte{0xfe}, 1024))
	suite.Require().NoError(err)

	off, err = r.Seek(0, io.SeekCurrent)
	suite.Require().NoError(err)
	suite.Assert().EqualValues(2048, off)

	_, err = r.Seek(-100, io.SeekStart)
	suite.Require().Equal(circular.ErrSeekBeforeStart, err)
}

func (suite *CircularSuite) TestRegularReaderEmpty() {
	buf, err := circular.NewBuffer()
	suite.Require().NoError(err)

	n, err := buf.GetReader().Read(nil)
	suite.Require().Equal(0, n)
	suite.Require().Equal(io.EOF, err)
}

func (suite *CircularSuite) TestRegularReader() {
	buf, err := circular.NewBuffer()
	suite.Require().NoError(err)

	_, err = buf.Write(bytes.Repeat([]byte{0xff}, 512))
	suite.Require().NoError(err)

	r := buf.GetReader()

	_, err = buf.Write(bytes.Repeat([]byte{0xfe}, 512))
	suite.Require().NoError(err)

	actual, err := ioutil.ReadAll(r)
	suite.Require().NoError(err)
	suite.Require().Equal(bytes.Repeat([]byte{0xff}, 512), actual)
}

func (suite *CircularSuite) TestRegularReaderOutOfSync() {
	buf, err := circular.NewBuffer(circular.WithInitialCapacity(2048), circular.WithMaxCapacity(65536), circular.WithSafetyGap(256))
	suite.Require().NoError(err)

	_, err = buf.Write(bytes.Repeat([]byte{0xff}, 512))
	suite.Require().NoError(err)

	r := buf.GetReader()

	_, err = buf.Write(bytes.Repeat([]byte{0xfe}, 65536-256))
	suite.Require().NoError(err)

	_, err = r.Read(nil)
	suite.Require().Equal(err, circular.ErrOutOfSync)
}

func (suite *CircularSuite) TestRegularReaderFull() {
	buf, err := circular.NewBuffer(circular.WithInitialCapacity(2048), circular.WithMaxCapacity(4096), circular.WithSafetyGap(256))
	suite.Require().NoError(err)

	_, err = buf.Write(bytes.Repeat([]byte{0xff}, 6146))
	suite.Require().NoError(err)

	r := buf.GetReader()

	_, err = buf.Write(bytes.Repeat([]byte{0xfe}, 100))
	suite.Require().NoError(err)

	actual, err := ioutil.ReadAll(r)
	suite.Require().NoError(err)
	suite.Require().Equal(bytes.Repeat([]byte{0xff}, 4096-256), actual)

	suite.Require().NoError(r.Close())

	_, err = r.Read(nil)
	suite.Require().Equal(err, circular.ErrClosed)
}

func (suite *CircularSuite) TestRegularSeek() {
	buf, err := circular.NewBuffer(circular.WithInitialCapacity(2048), circular.WithMaxCapacity(65536), circular.WithSafetyGap(256))
	suite.Require().NoError(err)

	_, err = buf.Write(bytes.Repeat([]byte{0xff}, 512))
	suite.Require().NoError(err)

	_, err = buf.Write(bytes.Repeat([]byte{0xfe}, 512))
	suite.Require().NoError(err)

	r := buf.GetReader()

	_, err = buf.Write(bytes.Repeat([]byte{0xfc}, 512))
	suite.Require().NoError(err)

	off, err := r.Seek(0, io.SeekCurrent)
	suite.Require().NoError(err)
	suite.Assert().EqualValues(0, off)

	data := make([]byte, 256)

	n, err := r.Read(data)
	suite.Require().NoError(err)
	suite.Assert().Equal(256, n)
	suite.Assert().Equal(bytes.Repeat([]byte{0xff}, 256), data)

	off, err = r.Seek(0, io.SeekCurrent)
	suite.Require().NoError(err)
	suite.Assert().EqualValues(256, off)

	off, err = r.Seek(-256, io.SeekEnd)
	suite.Require().NoError(err)
	suite.Assert().EqualValues(768, off)

	n, err = r.Read(data)
	suite.Require().NoError(err)
	suite.Assert().Equal(256, n)
	suite.Assert().Equal(bytes.Repeat([]byte{0xfe}, 256), data)

	off, err = r.Seek(2048, io.SeekStart)
	suite.Require().NoError(err)
	suite.Assert().EqualValues(1024, off)

	_, err = buf.Write(bytes.Repeat([]byte{0xfe}, 65536-256))
	suite.Require().NoError(err)

	off, err = r.Seek(0, io.SeekStart)
	suite.Require().NoError(err)
	suite.Assert().EqualValues(0, off)

	_, err = r.Seek(-100, io.SeekStart)
	suite.Require().Equal(circular.ErrSeekBeforeStart, err)

	_, err = r.Read(nil)
	suite.Require().Equal(circular.ErrOutOfSync, err)
}

func TestCircularSuite(t *testing.T) {
	suite.Run(t, new(CircularSuite))
}
