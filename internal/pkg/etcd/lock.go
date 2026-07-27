// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"fmt"

	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
)

// WithLock executes the given function exclusively by acquiring an Etcd lock with the given key.
func WithLock(ctx context.Context, key string, logger *zap.Logger, f func() error) error {
	etcdClient, err := NewLocalClient(ctx)
	if err != nil {
		return fmt.Errorf("error creating etcd client: %w", err)
	}

	defer etcdClient.Close() //nolint:errcheck

	session, err := concurrency.NewSession(etcdClient.Client)
	if err != nil {
		return fmt.Errorf("error creating etcd session: %w", err)
	}

	// Revoke the session lease to release the lock, instead of dropping the mutex key with
	// `Mutex.Unlock`.
	//
	// `Mutex.Unlock` is a no-op unless the client-side bookkeeping says the key was written, and
	// that bookkeeping is unreliable exactly when it matters: `Mutex.tryAcquire` assigns the
	// revision only after its transaction returns, so a transaction which is applied by etcd but
	// whose response never reaches a canceled client leaves the key in place while `Unlock`
	// reports `ErrLockReleased` and deletes nothing. The mutex is a FIFO queue ordered by create
	// revision, so such a key blocks every other waiter until its lease expires (60s).
	//
	// The lease is what the key hangs off, so revoking it drops the key whatever the client
	// believes, and it is a single round-trip for the common path too.
	defer func() {
		logger.Debug("releasing mutex", zap.String("key", key))

		if err := etcdClient.RevokeSession(ctx, session); err != nil {
			level := zap.ErrorLevel
			if ctx.Err() != nil {
				// etcd is expected to be unreachable while the machine is shutting down
				level = zap.DebugLevel
			}

			logger.Log(level, "error releasing mutex", zap.String("key", key), zap.Error(err))
		}
	}()

	mutex := concurrency.NewMutex(session, key)

	logger.Debug("waiting for mutex", zap.String("key", key))

	if err = mutex.Lock(ctx); err != nil {
		return fmt.Errorf("error acquiring mutex for key %s: %w", key, err)
	}

	logger.Debug("mutex acquired", zap.String("key", key))

	return f()
}
