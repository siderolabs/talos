// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/install"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// fakeMeta records the operations performed on the META.
type fakeMeta struct {
	calls []string
}

func (m *fakeMeta) ReadTag(uint8) (string, bool)                             { return "", false }
func (m *fakeMeta) ReadTagBytes(uint8) ([]byte, bool)                        { return nil, false }
func (m *fakeMeta) SetTag(context.Context, uint8, string) (bool, error)      { return false, nil }
func (m *fakeMeta) SetTagBytes(context.Context, uint8, []byte) (bool, error) { return false, nil }
func (m *fakeMeta) DeleteTag(context.Context, uint8) (bool, error)           { return false, nil }

func (m *fakeMeta) Reload(context.Context) error {
	m.calls = append(m.calls, "reload")

	return nil
}

func (m *fakeMeta) Flush() error {
	m.calls = append(m.calls, "flush")

	return nil
}

func stateWithMetaVolume(t *testing.T, ctx context.Context, phase blockres.VolumePhase) state.State { //nolint:revive
	t.Helper()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	volumeStatus := blockres.NewVolumeStatus(blockres.NamespaceName, constants.MetaPartitionLabel)
	volumeStatus.TypedSpec().Phase = phase

	require.NoError(t, st.Create(ctx, volumeStatus))

	return st
}

func TestReloadMeta(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	meta := &fakeMeta{}

	require.NoError(t, install.ReloadMeta(ctx, stateWithMetaVolume(t, ctx, blockres.VolumePhaseReady), meta))
	require.Equal(t, []string{"reload"}, meta.calls)
}

func TestSyncMeta(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	meta := &fakeMeta{}

	require.NoError(t, install.SyncMeta(ctx, stateWithMetaVolume(t, ctx, blockres.VolumePhaseReady), meta))
	require.Equal(t, []string{"flush"}, meta.calls)
}

func TestMetaVolumeNotReady(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	st := stateWithMetaVolume(t, ctx, blockres.VolumePhaseMissing)

	reloadMeta := &fakeMeta{}
	require.Error(t, install.ReloadMeta(ctx, st, reloadMeta))
	require.Empty(t, reloadMeta.calls)

	syncMeta := &fakeMeta{}
	require.Error(t, install.SyncMeta(ctx, st, syncMeta))
	require.Empty(t, syncMeta.calls)
}
