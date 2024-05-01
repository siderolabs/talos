// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"fmt"
	"time"

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

	defer session.Close() //nolint:errcheck

	mutex := concurrency.NewMutex(session, key)

	logger.Debug("waiting for mutex", zap.String("key", key))

	if err = mutex.Lock(ctx); err != nil {
		return fmt.Errorf("error acquiring mutex for key %s: %w", key, err)
	}

	logger.Debug("mutex acquired", zap.String("key", key))

	defer func() {
		logger.Debug("releasing mutex", zap.String("key", key))

		unlockCtx, unlockCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer unlockCancel()

		if err = mutex.Unlock(unlockCtx); err != nil {
			logger.Error("error releasing mutex", zap.String("key", key), zap.Error(err))
		}
	}()

	return f()
}
