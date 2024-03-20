// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1"
)

type testSequenceNumber int

func (candidate testSequenceNumber) CanTakeOver(running testSequenceNumber) bool {
	return candidate > running
}

func TestPriorityLock(t *testing.T) {
	require := require.New(t)

	lock := v1alpha1.NewPriorityLock[testSequenceNumber]()
	ctx := context.Background()

	ctx1, err := lock.Lock(ctx, time.Second, 2)
	require.NoError(err)

	select {
	case <-ctx1.Done():
		require.FailNow("should not be canceled")
	default:
	}

	_, err = lock.Lock(ctx, time.Millisecond, 1)
	require.Error(err)
	require.EqualError(err, runtime.ErrLocked.Error())

	errCh := make(chan error)

	go func() {
		_, err2 := lock.Lock(ctx, time.Second, 3)
		errCh <- err2
	}()

	select {
	case <-ctx1.Done():
	case <-time.After(time.Second):
		require.FailNow("should be canceled")
	}

	select {
	case <-errCh:
		require.FailNow("should not be reached")
	default:
	}

	lock.Unlock()

	select {
	case err = <-errCh:
		require.NoError(err)
	case <-time.After(time.Second):
		require.FailNow("should be canceled")
	}
}

func TestPriorityLockSequential(t *testing.T) {
	require := require.New(t)

	lock := v1alpha1.NewPriorityLock[testSequenceNumber]()
	ctx := context.Background()

	_, err := lock.Lock(ctx, time.Second, 2)
	require.NoError(err)

	lock.Unlock()

	_, err = lock.Lock(ctx, time.Second, 1)
	require.NoError(err)

	lock.Unlock()
}

//nolint:gocyclo
func TestPriorityLockConcurrent(t *testing.T) {
	require := require.New(t)

	lock := v1alpha1.NewPriorityLock[testSequenceNumber]()

	globalCtx, globalCtxCancel := context.WithCancel(context.Background())
	defer globalCtxCancel()

	var eg errgroup.Group

	sequenceCh := make(chan testSequenceNumber)

	for seq := testSequenceNumber(1); seq <= 20; seq++ {
		eg.Go(func() error {
			ctx, err := lock.Lock(globalCtx, time.Second, seq)
			if errors.Is(err, runtime.ErrLocked) {
				return nil
			}

			if err != nil {
				return err
			}

			select {
			case sequenceCh <- seq:
				<-ctx.Done()
			case <-ctx.Done():
			}

			lock.Unlock()

			return nil
		})
	}

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	var prevSeq testSequenceNumber

	for {
		select {
		case <-timer.C:
			require.FailNow("timeout")
		case seq := <-sequenceCh:
			t.Logf("sequence running: %d", seq)

			if prevSeq >= seq {
				require.FailNowf("can take over inversion", "sequence %d should be greater than %d", seq, prevSeq)
			}

			prevSeq = seq
		}

		if prevSeq == 20 {
			globalCtxCancel()

			break
		}
	}

	require.NoError(eg.Wait())
}
