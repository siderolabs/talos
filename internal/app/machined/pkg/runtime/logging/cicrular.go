// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/pkg/circular"
	"github.com/talos-systems/talos/pkg/tail"
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

	senderChanged chan struct{}

	senderRW sync.RWMutex
	sender   runtime.LogSender
}

// NewCircularBufferLoggingManager initializes new CircularBufferLoggingManager.
func NewCircularBufferLoggingManager(fallbackLogger *log.Logger) *CircularBufferLoggingManager {
	return &CircularBufferLoggingManager{
		fallbackLogger: fallbackLogger,
		senderChanged:  make(chan struct{}, 1),
	}
}

// ServiceLog implements runtime.LoggingManager interface.
func (manager *CircularBufferLoggingManager) ServiceLog(id string) runtime.LogHandler {
	return &circularHandler{
		manager: manager,
		id:      id,
		fields: map[string]interface{}{
			"component": id,
		},
	}
}

// SetSender implements runtime.LoggingManager interface.
func (manager *CircularBufferLoggingManager) SetSender(sender runtime.LogSender) {
	manager.senderRW.Lock()
	manager.sender = sender
	manager.senderRW.Unlock()

	select {
	case manager.senderChanged <- struct{}{}:
	default:
	}
}

func (manager *CircularBufferLoggingManager) getSender() (runtime.LogSender, <-chan struct{}) {
	manager.senderRW.RLock()
	defer manager.senderRW.RUnlock()

	return manager.sender, manager.senderChanged
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
			if err := handler.runSender(); err != nil {
				handler.manager.fallbackLogger.Printf("log sender stopped: %s", err)
			}
		}()
	}

	return nopCloser{handler.buf}, nil
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

func (handler *circularHandler) runSender() error {
	r, err := handler.Reader(runtime.WithFollow())
	if err != nil {
		return err
	}
	defer r.Close() //nolint:errcheck

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		l := strings.TrimSpace(scanner.Text())
		if l == "" {
			continue
		}

		handler.resend(&runtime.LogEvent{
			Msg:    l,
			Fields: handler.fields,
		})
	}

	return fmt.Errorf("scanner: %w", scanner.Err())
}

// resend sends and resends given event until success or ErrDontRetry error.
func (handler *circularHandler) resend(e *runtime.LogEvent) {
	for {
		var sender runtime.LogSender

		// wait for sender to be set
		for {
			var changed <-chan struct{}

			sender, changed = handler.manager.getSender()
			if sender != nil {
				break
			}

			handler.manager.fallbackLogger.Printf("waiting for sender at %s", time.Now())
			<-changed
		}

		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)

		err := sender.Send(ctx, e)

		cancel()

		if err == nil {
			return
		}

		handler.manager.fallbackLogger.Print(err)

		if errors.Is(err, runtime.ErrDontRetry) {
			return
		}

		time.Sleep(time.Second)
	}
}
