// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package check provides set of checks to verify cluster readiness.
package check

import (
	"context"
	"time"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/conditions"
)

const updateInterval = 100 * time.Millisecond

// ClusterInfo is interface requires by checks.
type ClusterInfo interface {
	cluster.ClientProvider
	cluster.K8sProvider
	cluster.Info
}

// ClusterCheck implements a function which returns condition based on ClusterAccess.
type ClusterCheck func(ClusterInfo) conditions.Condition

// Reporter presents wait progress.
//
// It is supposed that reporter drops duplicate messages.
type Reporter interface {
	Update(check conditions.Condition)
}

// Wait run the checks against the cluster and waits for the full set to succeed.
//
// Context ctx might have a timeout set to limit overall wait time.
// Each check might define its own timeout.
func Wait(ctx context.Context, cluster ClusterInfo, checks []ClusterCheck, reporter Reporter) error {
	for _, check := range checks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		condition := check(cluster)

		errCh := make(chan error, 1)

		go func(condition conditions.Condition) {
			errCh <- condition.Wait(ctx)
		}(condition)

		var err error

		func() {
			ticker := time.NewTicker(updateInterval)
			defer ticker.Stop()

			// report initial state
			reporter.Update(condition)

			// report last state
			defer reporter.Update(condition)

			for {
				select {
				case err = <-errCh:
					return
				case <-ticker.C:
					reporter.Update(condition)
				}
			}
		}()

		if err != nil {
			return err
		}
	}

	return nil
}
