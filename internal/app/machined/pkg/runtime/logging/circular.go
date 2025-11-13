// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"sync"
	"time"

	corezstd "github.com/klauspost/compress/zstd"
	"github.com/siderolabs/go-circular"
	"github.com/siderolabs/go-circular/zstd"
	"github.com/siderolabs/go-debug"
	"github.com/siderolabs/go-tail"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
)

// These constants should some day move to config.
const (
	// Overall capacity of the log buffer (in raw bytes, memory size will be smaller due to compression).
	DesiredCapacity = 1048576
	// Some logs are tiny, no need to reserve too much memory.
	InitialCapacity = 16384
	// Chunk capacity is the length of each chunk, it should be
	// big enough for the compression to be efficient.
	ChunkCapacity = 65536
	// Number of zstd-compressed chunks to keep.
	NumCompressedChunks = (DesiredCapacity / ChunkCapacity) - 1
	// Safety gap to avoid buffer overruns, can be lowered as with compression we don't need much.
	SafetyGap = 1
)

// CircularBufferLoggingManager implements logging to circular fixed size buffer.
type CircularBufferLoggingManager struct {
	fallbackLogger *log.Logger

	buffers    sync.Map
	compressor circular.Compressor

	sendersRW      sync.RWMutex
	senders        []runtime.LogSender
	sendersChanged chan struct{}
}

// NewCircularBufferLoggingManager initializes new CircularBufferLoggingManager.
func NewCircularBufferLoggingManager(fallbackLogger *log.Logger) *CircularBufferLoggingManager {
	compressor, err := zstd.NewCompressor(
		corezstd.WithEncoderConcurrency(1),
		corezstd.WithWindowSize(2*corezstd.MinWindowSize),
	)
	if err != nil {
		// should not happen
		panic(fmt.Sprintf("failed to create zstd compressor: %s", err))
	}

	return &CircularBufferLoggingManager{
		fallbackLogger: fallbackLogger,
		sendersChanged: make(chan struct{}),
		compressor:     compressor,
	}
}

// ServiceLog implements runtime.LoggingManager interface.
func (manager *CircularBufferLoggingManager) ServiceLog(id string) runtime.LogHandler {
	return &circularHandler{
		manager: manager,
		id:      id,
		fields: map[string]any{
			// use field name that is not used by anything else
			"talos-service": id,
		},
	}
}

// SetSenders implements runtime.LoggingManager interface.
func (manager *CircularBufferLoggingManager) SetSenders(senders []runtime.LogSender) []runtime.LogSender {
	manager.sendersRW.Lock()

	prevChanged := manager.sendersChanged
	manager.sendersChanged = make(chan struct{})

	prevSenders := manager.senders
	manager.senders = senders

	manager.sendersRW.Unlock()

	close(prevChanged)

	return prevSenders
}

// getSenders waits for senders to be set and returns them.
func (manager *CircularBufferLoggingManager) getSenders() []runtime.LogSender {
	for {
		manager.sendersRW.RLock()

		senders, changed := manager.senders, manager.sendersChanged

		manager.sendersRW.RUnlock()

		if len(senders) > 0 {
			return senders
		}

		<-changed
	}
}

func (manager *CircularBufferLoggingManager) getBuffer(id string, create bool) (*circular.Buffer, bool, error) {
	buf, ok := manager.buffers.Load(id)
	if ok {
		return buf.(*circular.Buffer), false, nil
	}

	if !create {
		return nil, false, nil
	}

	b, err := circular.NewBuffer(
		circular.WithInitialCapacity(InitialCapacity),
		circular.WithMaxCapacity(ChunkCapacity),
		circular.WithNumCompressedChunks(NumCompressedChunks, manager.compressor),
		circular.WithSafetyGap(SafetyGap))
	if err != nil {
		return nil, false, err // only configuration issue might raise error
	}

	buf, _ = manager.buffers.LoadOrStore(id, b)

	return buf.(*circular.Buffer), true, nil
}

// RegisteredLogs implements runtime.LoggingManager interface.
func (manager *CircularBufferLoggingManager) RegisteredLogs() []string {
	var result []string

	manager.buffers.Range(func(key, val any) bool {
		result = append(result, key.(string))

		return true
	})

	return result
}

type circularHandler struct {
	manager *CircularBufferLoggingManager
	id      string
	fields  map[string]any

	buf *circular.Buffer
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error {
	return nil
}

// Writer implements runtime.LogHandler interface.
func (handler *circularHandler) Writer() (io.WriteCloser, error) {
	if handler.buf == nil {
		var (
			created bool
			err     error
		)

		handler.buf, created, err = handler.manager.getBuffer(handler.id, true)
		if err != nil {
			return nil, err
		}

		if created {
			go func() {
				defer func() {
					if r := recover(); r != nil {
						handler.manager.fallbackLogger.Printf("log sender panic: %v", r)
					}
				}()

				if err := handler.runSenders(); err != nil {
					handler.manager.fallbackLogger.Printf("log senders stopped: %s", err)
				}
			}()
		}
	}

	switch handler.id {
	case "machined":
		return &timeStampWriter{w: handler.buf}, nil
	default:
		return nopCloser{handler.buf}, nil
	}
}

// Reader implements runtime.LogHandler interface.
func (handler *circularHandler) Reader(opts ...runtime.LogOption) (io.ReadCloser, error) {
	if handler.buf == nil {
		var err error

		handler.buf, _, err = handler.manager.getBuffer(handler.id, false)
		if err != nil {
			return nil, err
		}

		if handler.buf == nil {
			// only Writer() operation creates new buffers
			return nil, fmt.Errorf("log %q was not registered", handler.id)
		}
	}

	var opt runtime.LogOptions

	for _, o := range opts {
		if err := o(&opt); err != nil {
			return nil, err
		}
	}

	var r interface {
		io.ReadCloser
		io.Seeker
	}

	if opt.Follow {
		r = handler.buf.GetStreamingReader()
	} else {
		r = handler.buf.GetReader()
	}

	if opt.TailLines != nil {
		err := tail.SeekLines(r, *opt.TailLines)
		if err != nil {
			r.Close() //nolint:errcheck

			return nil, fmt.Errorf("error tailing log: %w", err)
		}
	}

	return r, nil
}

func (handler *circularHandler) runSenders() error {
	r, err := handler.Reader(runtime.WithFollow())
	if err != nil {
		return err
	}
	defer r.Close() //nolint:errcheck

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		l := bytes.TrimSpace(scanner.Bytes())
		if len(l) == 0 {
			continue
		}

		e := parseLogLine(l, time.Now())
		if e.Fields == nil {
			e.Fields = handler.fields
		} else {
			maps.Copy(e.Fields, handler.fields)
		}

		handler.resend(e)
	}

	return fmt.Errorf("scanner: %w", scanner.Err())
}

// resend sends and resends given event until success or ErrDontRetry error.
func (handler *circularHandler) resend(e *runtime.LogEvent) {
	for {
		senders := handler.manager.getSenders()

		sendCtx, sendCancel := context.WithTimeout(context.TODO(), 5*time.Second)
		sendErrors := make(chan error, len(senders))

		for _, sender := range senders {
			go func() {
				sendErrors <- sender.Send(sendCtx, e)
			}()
		}

		var dontRetry bool

		for range senders {
			err := <-sendErrors

			// don't retry if at least one sender succeed to avoid implementing per-sender queue, etc
			if err == nil {
				dontRetry = true

				continue
			}

			if debug.Enabled {
				handler.manager.fallbackLogger.Print(err)
			}

			if errors.Is(err, runtime.ErrDontRetry) {
				dontRetry = true
			}
		}

		sendCancel()

		if dontRetry {
			return
		}

		time.Sleep(time.Second)
	}
}

// timeStampWriter is a writer that adds a timestamp to each line.
type timeStampWriter struct {
	w io.WriteCloser
}

// Write implements the io.Writer interface.
func (t *timeStampWriter) Write(p []byte) (int, error) {
	buf := make([]byte, 0, len(p)+27)

	// Current log.Logger implementation always adds a newline to the message, so we don't need to wait for it.
	buf = time.Now().AppendFormat(buf, "2006/01/02 15:04:05.000000")
	buf = append(buf, ' ')
	buf = append(buf, p...)

	n, err := t.w.Write(buf)

	switch {
	case err == nil && n == len(buf):
		return len(p), nil // success, return original length
	case err == nil && n != len(buf):
		return n, fmt.Errorf("time stamp writer error: %w", io.ErrShortWrite)
	default:
		return n, fmt.Errorf("time stamp writer internal error: %w", err)
	}
}

// Close implements the io.Closer interface.
func (t *timeStampWriter) Close() error { return t.w.Close() }
