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
	"sync"
	"time"

	"github.com/siderolabs/go-circular"
	"github.com/siderolabs/go-debug"
	"github.com/siderolabs/go-tail"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
)

// These constants should some day move to config.
const (
	// Some logs are tiny, no need to reserve too much memory.
	InitialCapacity = 16384
	// Cap each log at 1M.
	MaxCapacity = 1048576
	// Safety gap to avoid buffer overruns.
	SafetyGap = 2048
)

// CircularBufferLoggingManager implements logging to circular fixed size buffer.
type CircularBufferLoggingManager struct {
	fallbackLogger *log.Logger

	buffers sync.Map

	sendersRW      sync.RWMutex
	senders        []runtime.LogSender
	sendersChanged chan struct{}
}

// NewCircularBufferLoggingManager initializes new CircularBufferLoggingManager.
func NewCircularBufferLoggingManager(fallbackLogger *log.Logger) *CircularBufferLoggingManager {
	return &CircularBufferLoggingManager{
		fallbackLogger: fallbackLogger,
		sendersChanged: make(chan struct{}),
	}
}

// ServiceLog implements runtime.LoggingManager interface.
func (manager *CircularBufferLoggingManager) ServiceLog(id string) runtime.LogHandler {
	return &circularHandler{
		manager: manager,
		id:      id,
		fields: map[string]interface{}{
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

func (manager *CircularBufferLoggingManager) getBuffer(id string, create bool) (*circular.Buffer, error) {
	buf, ok := manager.buffers.Load(id)
	if !ok {
		if !create {
			return nil, nil
		}

		b, err := circular.NewBuffer(
			circular.WithInitialCapacity(InitialCapacity),
			circular.WithMaxCapacity(MaxCapacity),
			circular.WithSafetyGap(SafetyGap))
		if err != nil {
			return nil, err // only configuration issue might raise error
		}

		buf, _ = manager.buffers.LoadOrStore(id, b)
	}

	return buf.(*circular.Buffer), nil
}

type circularHandler struct {
	manager *CircularBufferLoggingManager
	id      string
	fields  map[string]interface{}

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
		var err error

		handler.buf, err = handler.manager.getBuffer(handler.id, true)
		if err != nil {
			return nil, err
		}

		go func() {
			if err := handler.runSenders(); err != nil {
				handler.manager.fallbackLogger.Printf("log senders stopped: %s", err)
			}
		}()
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

		handler.buf, err = handler.manager.getBuffer(handler.id, false)
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
			for k, v := range handler.fields {
				e.Fields[k] = v
			}
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
			sender := sender

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
	w io.Writer
}

// Write implements the io.Writer interface.
func (t *timeStampWriter) Write(p []byte) (int, error) {
	// Current log.Logger implementation always adds a newline to the message, so we don't need to wait for it.
	var buf bytes.Buffer

	buf.WriteString(time.Now().Format("2006/01/02 15:04:05.000000"))
	buf.WriteByte(' ')
	buf.Write(p)

	return t.w.Write(buf.Bytes())
}

// Close implements the io.Closer interface.
func (t *timeStampWriter) Close() error {
	if c, ok := t.w.(io.Closer); ok {
		return c.Close()
	}

	return nil
}
