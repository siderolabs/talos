// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvm_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/lvm"
)

// These tests cover argument validation in the provisioning wrappers
// (PVCreate / VGCreate / VGExtend / VGReduce). Execution against the actual
// lvm binary is exercised by the integration suite; the unit tests here only
// check guards that don't shell out.

func newLVM(t *testing.T) *lvm.LVM {
	t.Helper()

	l, err := lvm.New(lvm.WithSELinuxLabel(""))
	require.NoError(t, err)

	return l
}

func TestPVCreateRejectsEmptyDevice(t *testing.T) {
	l := newLVM(t)

	err := l.PVCreate(context.Background(), "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "device must be non-empty")
}

func TestVGCreateRejectsEmptyVG(t *testing.T) {
	l := newLVM(t)

	err := l.VGCreate(context.Background(), "", "/dev/sda")
	require.Error(t, err)
	require.Contains(t, err.Error(), "vg must be non-empty")
}

func TestVGCreateRejectsNoPVs(t *testing.T) {
	l := newLVM(t)

	err := l.VGCreate(context.Background(), "vg0")
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one physical volume is required")
}

func TestVGExtendRejectsEmptyVG(t *testing.T) {
	l := newLVM(t)

	err := l.VGExtend(context.Background(), "", "/dev/sda")
	require.Error(t, err)
	require.Contains(t, err.Error(), "vg must be non-empty")
}

func TestVGExtendRejectsNoPVs(t *testing.T) {
	l := newLVM(t)

	err := l.VGExtend(context.Background(), "vg0")
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one physical volume is required")
}

func TestVGReduceRejectsEmptyVG(t *testing.T) {
	l := newLVM(t)

	err := l.VGReduce(context.Background(), "", "/dev/sda")
	require.Error(t, err)
	require.Contains(t, err.Error(), "vg must be non-empty")
}

func TestVGReduceRejectsNoPVs(t *testing.T) {
	l := newLVM(t)

	err := l.VGReduce(context.Background(), "vg0")
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one physical volume is required")
}
