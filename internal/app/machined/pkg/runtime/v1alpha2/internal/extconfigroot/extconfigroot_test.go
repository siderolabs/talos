// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package extconfigroot_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha2/internal/extconfigroot"
	mountv3 "github.com/siderolabs/talos/internal/pkg/mount/v3"
	"github.com/siderolabs/talos/pkg/xfs"
)

func TestExtensionServiceConfigRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root for mount operations")
	}

	target := filepath.Join(t.TempDir(), "extension-config")

	root, unmount, err := extconfigroot.Setup(target, zap.NewNop())
	require.NoError(t, err)

	unmounted := false

	t.Cleanup(func() {
		if !unmounted {
			unmount() //nolint:errcheck
		}
	})

	require.NoError(t, xfs.MkdirAll(root, "test-service", 0o755))
	require.NoError(t, xfs.WriteFile(root, "test-service/etc-test", []byte("secret"), 0o600))

	stableSource := filepath.Join(target, "test-service/etc-test")
	contents, err := os.ReadFile(stableSource)
	require.NoError(t, err)
	assert.Equal(t, "secret", string(contents))

	err = os.WriteFile(filepath.Join(target, "blocked"), []byte("blocked"), 0o600)
	assert.True(t, errors.Is(err, unix.EROFS), "stable view should be read-only: %v", err)

	bindTarget := filepath.Join(t.TempDir(), "bound-config")
	require.NoError(t, os.WriteFile(bindTarget, nil, 0o600))
	require.NoError(t, mountv3.BindReadonly(stableSource, bindTarget))

	t.Cleanup(func() {
		unix.Unmount(bindTarget, unix.MNT_DETACH) //nolint:errcheck
	})

	boundContents, err := os.ReadFile(bindTarget)
	require.NoError(t, err)
	assert.Equal(t, "secret", string(boundContents))

	require.NoError(t, unix.Unmount(bindTarget, unix.MNT_DETACH))
	require.NoError(t, unmount())

	unmounted = true

	_, err = os.Stat(stableSource)
	assert.True(t, os.IsNotExist(err))
}
