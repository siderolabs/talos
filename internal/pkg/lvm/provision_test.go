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

func TestLVCreateRejectsEmptyVGorLV(t *testing.T) {
	l := newLVM(t)

	err := l.LVCreate(context.Background(), "", "lv0", lvm.LVCreateOptions{SizeBytes: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "vg and lv must be non-empty")

	err = l.LVCreate(context.Background(), "vg0", "", lvm.LVCreateOptions{SizeBytes: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "vg and lv must be non-empty")
}

func TestLVCreateRejectsUnknownType(t *testing.T) {
	l := newLVM(t)

	err := l.LVCreate(context.Background(), "vg0", "lv0", lvm.LVCreateOptions{Type: "thin", SizeBytes: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported logical volume type")
}

func TestLVCreateRejectsNoSize(t *testing.T) {
	l := newLVM(t)

	err := l.LVCreate(context.Background(), "vg0", "lv0", lvm.LVCreateOptions{Type: "linear"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "either SizeBytes or SizePercentVG must be set")
}

func TestLVCreateRejectsBadRAIDParams(t *testing.T) {
	l := newLVM(t)

	err := l.LVCreate(context.Background(), "vg0", "lv0", lvm.LVCreateOptions{Type: "raid0", Stripes: 1, SizeBytes: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "raid0 requires at least 2 stripes")

	err = l.LVCreate(context.Background(), "vg0", "lv0", lvm.LVCreateOptions{Type: "raid1", SizeBytes: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "raid1 requires at least 1 mirror")

	err = l.LVCreate(context.Background(), "vg0", "lv0", lvm.LVCreateOptions{Type: "raid10", Mirrors: 1, Stripes: 1, SizeBytes: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "raid10 requires at least 2 stripes")
}

func TestLVExtendRejectsEmptyVGorLV(t *testing.T) {
	l := newLVM(t)

	err := l.LVExtend(context.Background(), "", "lv0", lvm.LVExtendOptions{SizeBytes: 1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "vg and lv must be non-empty")
}

func TestLVExtendRejectsNoSize(t *testing.T) {
	l := newLVM(t)

	err := l.LVExtend(context.Background(), "vg0", "lv0", lvm.LVExtendOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "either SizeBytes or SizePercentVG must be set")
}
