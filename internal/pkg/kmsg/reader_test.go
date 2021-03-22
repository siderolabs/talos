// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kmsg_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/internal/pkg/kmsg"
)

//nolint:thelper
func skipIfNoKmsg(t *testing.T) {
	f, err := os.OpenFile("/dev/kmsg", os.O_RDONLY, 0)
	if err != nil {
		t.Skip("/dev/kmsg is not available", err.Error())
	}

	f.Close() //nolint:errcheck
}

func TestReaderNoFollow(t *testing.T) {
	skipIfNoKmsg(t)

	r, err := kmsg.NewReader()
	assert.NoError(t, err)

	defer r.Close() //nolint:errcheck

	messageCount := 0

	for packet := range r.Scan(context.Background()) {
		assert.NoError(t, packet.Err)

		messageCount++
	}

	assert.Greater(t, messageCount, 0)

	assert.NoError(t, r.Close())
}

func TestReaderFollow(t *testing.T) {
	testReaderFollow(t, true)
}

func TestReaderFollowTail(t *testing.T) {
	testReaderFollow(t, false, kmsg.FromTail())
}

//nolint:thelper
func testReaderFollow(t *testing.T, expectMessages bool, options ...kmsg.Option) {
	skipIfNoKmsg(t)

	r, err := kmsg.NewReader(append([]kmsg.Option{kmsg.Follow()}, options...)...)
	assert.NoError(t, err)

	defer r.Close() //nolint:errcheck

	messageCount := 0

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	ch := r.Scan(ctx)

	var closed bool

LOOP:
	for {
		select {
		case packet, ok := <-ch:
			if !ok {
				if !closed {
					assert.Fail(t, "channel closed before cancel")
				}

				break LOOP
			}

			if closed && errors.Is(packet.Err, os.ErrClosed) {
				// ignore 'file already closed' error as it might happen
				// from the branch below depending on whether context cancel or
				// read() finishes first
				continue
			}

			assert.NoError(t, packet.Err)

			messageCount++
		case <-time.After(100 * time.Millisecond):
			// abort
			closed = true
			ctxCancel()
			assert.NoError(t, r.Close())
		}
	}

	if expectMessages {
		assert.Greater(t, messageCount, 0)
	}
}
