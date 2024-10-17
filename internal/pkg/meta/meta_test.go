// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package meta provides access to META partition: key-value partition persisted across reboots.
package meta_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/meta"
	metaconsts "github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func setupTest(t *testing.T) (*meta.Meta, string, state.State) {
	t.Helper()

	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "meta")

	f, err := os.Create(path)
	require.NoError(t, err)

	require.NoError(t, f.Truncate(1024*1024))

	require.NoError(t, f.Close())

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	m, err := meta.New(context.Background(), st, meta.WithFixedPath(path))
	require.NoError(t, err)

	return m, path, st
}

func TestFlow(t *testing.T) {
	t.Parallel()

	m, path, st := setupTest(t)

	ctx := context.Background()

	ok, err := m.SetTag(ctx, metaconsts.Upgrade, "1.2.3")
	require.NoError(t, err)
	assert.True(t, ok)

	val, ok := m.ReadTag(metaconsts.Upgrade)
	assert.True(t, ok)
	assert.Equal(t, "1.2.3", val)

	_, ok = m.ReadTag(metaconsts.StagedUpgradeImageRef)
	assert.False(t, ok)

	ok, err = m.DeleteTag(ctx, metaconsts.Upgrade)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = m.SetTag(ctx, metaconsts.StagedUpgradeInstallOptions, "install-fast")
	require.NoError(t, err)
	assert.True(t, ok)

	assert.NoError(t, m.Flush())

	assert.NoError(t, m.Reload(ctx))

	val, ok = m.ReadTag(metaconsts.StagedUpgradeInstallOptions)
	assert.True(t, ok)
	assert.Equal(t, "install-fast", val)

	m2, err := meta.New(ctx, st, meta.WithFixedPath(path))
	require.NoError(t, err)

	_, ok = m2.ReadTag(metaconsts.Upgrade)
	assert.False(t, ok)

	val, ok = m2.ReadTag(metaconsts.StagedUpgradeInstallOptions)
	assert.True(t, ok)
	assert.Equal(t, "install-fast", val)

	list, err := safe.StateList[*runtime.MetaKey](ctx, st, runtime.NewMetaKey(runtime.NamespaceName, "").Metadata())
	require.NoError(t, err)

	for res := range list.All() {
		assert.Equal(t, "0x08", res.Metadata().ID())
		assert.Equal(t, "install-fast", res.TypedSpec().Value)
	}
}
