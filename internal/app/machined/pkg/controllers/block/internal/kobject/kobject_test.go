// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kobject_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/kobject"
)

func TestWatcher(t *testing.T) {
	watcher, err := kobject.NewWatcher()
	require.NoError(t, err)

	evCh := watcher.Run(zaptest.NewLogger(t))

	require.NoError(t, watcher.Close())

	// the evCh should be closed
	for range evCh { //nolint:revive
	}
}
