// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging

import (
	"fmt"
	"io"
	"sync"

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
	buffers sync.Map
}

// NewCircularBufferLoggingManager initializes new CircularBufferLoggingManager.
func NewCircularBufferLoggingManager() *CircularBufferLoggingManager {
	return &CircularBufferLoggingManager{}
}

// ServiceLog implements runtime.LoggingManager interface.
func (manager *CircularBufferLoggingManager) ServiceLog(id string) runtime.LogHandler {
	return &circularHandler{
		manager: manager,
		id:      id,
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

	buf *circular.Buffer
}

type nopCloser struct {
	io.Writer
}

func (c nopCloser) Close() error {
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
