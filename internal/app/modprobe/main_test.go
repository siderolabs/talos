// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package modprobe_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/modprobe"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestModuleNameFromArgs(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		args []string

		expectedName string
		expectedOk   bool
	}{
		{
			name:         "kernel",
			args:         []string{"/sbin/modprobe", "-q", "--", "tc90522"},
			expectedName: "tc90522",
			expectedOk:   true,
		},
		{
			name:         "no options",
			args:         []string{"/sbin/modprobe", "drbd_transport_tcp"},
			expectedName: "drbd_transport_tcp",
			expectedOk:   true,
		},
		{
			name:         "options",
			args:         []string{"/sbin/modprobe", "-q", "-v", "dm-thin-pool", "extra"},
			expectedName: "dm-thin-pool",
			expectedOk:   true,
		},
		{
			name: "no name",
			args: []string{"/sbin/modprobe", "-q", "--"},
		},
		{
			name: "only options",
			args: []string{"/sbin/modprobe", "-q"},
		},
		{
			name: "empty",
			args: []string{"/sbin/modprobe"},
		},
		{
			name: "nil",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			name, ok := modprobe.ModuleNameFromArgs(test.args)
			assert.Equal(t, test.expectedOk, ok)
			assert.Equal(t, test.expectedName, name)
		})
	}
}

func TestQuietFromArgs(t *testing.T) {
	t.Parallel()

	assert.True(t, modprobe.QuietFromArgs([]string{"/sbin/modprobe", "-q", "--", "tc90522"}))
	assert.True(t, modprobe.QuietFromArgs([]string{"/sbin/modprobe", "-v", "--quiet", "tc90522"}))
	assert.False(t, modprobe.QuietFromArgs([]string{"/sbin/modprobe", "tc90522"}))
	assert.False(t, modprobe.QuietFromArgs([]string{"/sbin/modprobe", "--", "-q"}))
	assert.False(t, modprobe.QuietFromArgs(nil))
}

func TestLookupModule(t *testing.T) {
	t.Parallel()

	modulesDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(modulesDir, "modules.builtin"), []byte(
		"kernel/fs/xfs/xfs.ko\nkernel/drivers/md/dm-mod.ko\n",
	), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(modulesDir, "modules.dep"), []byte(
		"kernel/drivers/md/dm-thin-pool.ko: kernel/drivers/md/persistent-data/dm-persistent-data.ko kernel/drivers/md/dm-bio-prison.ko\n"+
			"kernel/drivers/md/dm-bio-prison.ko:\n"+
			"kernel/drivers/md/persistent-data/dm-persistent-data.ko:\n"+
			"extras/drbd_transport_tcp.ko.zst: extras/drbd.ko.zst\n"+
			"extras/drbd.ko.zst:\n"+
			"kernel/fs/btrfs/btrfs.ko:\n",
	), 0o644))

	for name, expected := range map[string]modprobe.Lookup{
		"dm-thin-pool":       modprobe.LookupLoadable,
		"dm_thin_pool":       modprobe.LookupLoadable,
		"dm-persistent-data": modprobe.LookupLoadable,
		"drbd_transport_tcp": modprobe.LookupLoadable,
		"drbd-transport-tcp": modprobe.LookupLoadable,
		"btrfs":              modprobe.LookupLoadable,
		"xfs":                modprobe.LookupBuiltin,
		"dm_mod":             modprobe.LookupBuiltin,
		// aliases are not resolved
		"fs-btrfs":   modprobe.LookupNotFound,
		"dm-mod":     modprobe.LookupBuiltin,
		"net-pf-10":  modprobe.LookupNotFound,
		"crypto-aes": modprobe.LookupNotFound,
		"kernel":     modprobe.LookupNotFound,
		"extras":     modprobe.LookupNotFound,
		"":           modprobe.LookupNotFound,
	} {
		lookup, err := modprobe.LookupModule(modulesDir, name)
		require.NoError(t, err)

		assert.Equal(t, expected, lookup, "name %q", name)
	}

	// no modules.builtin is fine, no modules.dep is not
	require.NoError(t, os.Remove(filepath.Join(modulesDir, "modules.builtin")))

	lookup, err := modprobe.LookupModule(modulesDir, "btrfs")
	require.NoError(t, err)
	assert.Equal(t, modprobe.LookupLoadable, lookup)

	_, err = modprobe.LookupModule(t.TempDir(), "btrfs")
	require.Error(t, err)
}

func TestParametersFor(t *testing.T) {
	t.Parallel()

	newSpec := func(name string, parameters ...string) *runtime.KernelModuleSpec {
		spec := runtime.NewKernelModuleSpec(runtime.NamespaceName, name)
		spec.TypedSpec().Name = name
		spec.TypedSpec().Parameters = parameters

		return spec
	}

	specs := []*runtime.KernelModuleSpec{
		newSpec("dm-thin-pool", "foo=bar", "baz=1"),
		newSpec("tc90522"),
	}

	assert.Equal(t, []string{"foo=bar", "baz=1"}, modprobe.ParametersFor(specs, "dm-thin-pool"))
	assert.Equal(t, []string{"foo=bar", "baz=1"}, modprobe.ParametersFor(specs, "dm_thin_pool"))
	assert.Empty(t, modprobe.ParametersFor(specs, "tc90522"))
	assert.Empty(t, modprobe.ParametersFor(specs, "drbd_transport_tcp"))
	assert.Empty(t, modprobe.ParametersFor(nil, "dm-thin-pool"))
}

func TestValidModuleName(t *testing.T) {
	t.Parallel()

	for name, expected := range map[string]bool{
		"tc90522":            true,
		"drbd_transport_tcp": true,
		"dm-thin-pool":       true,
		"net-pf-10":          true, // syntactically a name, refused by the lookup
		"":                   false,
		"devname:fuse":       false,
		"crypto-aes cbc":     false,
		"../evil":            false,
		"fs-nfs\n":           false,
	} {
		assert.Equal(t, expected, modprobe.ValidModuleName(name), "name %q", name)
	}
}

func TestVerifyKernelParent(t *testing.T) {
	t.Parallel()

	procfs := t.TempDir()

	writeStat := func(pid int, contents string) {
		require.NoError(t, os.MkdirAll(filepath.Join(procfs, strconv.Itoa(pid)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(procfs, strconv.Itoa(pid), "stat"), []byte(contents), 0o644))
	}

	// kthreadd: PF_KTHREAD (0x00200000) is set
	writeStat(2, "2 (kthreadd) S 0 0 0 0 -1 2129984 0 0 0 0 0 0 0 0 20 0 1 0 2 0 0 18446744073709551615 0 0 0 0 0 0 0 2147483647 0 0 0 0 17 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n")
	// kworker (with a space in the name)
	writeStat(60, "60 (kworker/u8:1 x) I 2 0 0 0 -1 69238880 0 0 0 0 0 0 0 0 20 0 1 0 2 0 0 18446744073709551615 0 0 0 0 0 0 0 2147483647 0 0 0 0 17 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n")
	// a userspace process with a tricky name
	writeStat(1000, "1000 (a ) b (c) S 1 1000 1000 0 -1 4194560 100 0 0 0 1 0 0 0 20 0 1 0 100 1000 100 18446744073709551615 0 0 0 0 0 0 0 0 0 0 0 0 17 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n")
	// malformed
	writeStat(2000, "2000 broken\n")

	assert.NoError(t, modprobe.VerifyKernelParent(2, procfs))
	assert.NoError(t, modprobe.VerifyKernelParent(60, procfs))

	assert.ErrorContains(t, modprobe.VerifyKernelParent(1000, procfs), "is not a kernel thread")
	assert.ErrorContains(t, modprobe.VerifyKernelParent(2000, procfs), "error parsing parent process status")
	assert.ErrorContains(t, modprobe.VerifyKernelParent(3000, procfs), "error reading parent process status")
	assert.ErrorContains(t, modprobe.VerifyKernelParent(1, procfs), "is not a kernel thread")
	assert.ErrorContains(t, modprobe.VerifyKernelParent(0, procfs), "is not a kernel thread")
}
