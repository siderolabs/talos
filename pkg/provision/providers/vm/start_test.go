// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

func TestIsProcessRunning(t *testing.T) {
	t.Run("non-existent pid file", func(t *testing.T) {
		assert.False(t, vm.IsProcessRunning("/nonexistent/path/to/pid"))
	})

	t.Run("invalid pid in file", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidPath := filepath.Join(tmpDir, "test.pid")

		err := os.WriteFile(pidPath, []byte("not-a-number"), 0o644)
		assert.NoError(t, err)

		assert.False(t, vm.IsProcessRunning(pidPath))
	})

	t.Run("non-existent process", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidPath := filepath.Join(tmpDir, "test.pid")

		// Use a very high PID that's unlikely to exist
		err := os.WriteFile(pidPath, []byte("999999999"), 0o644)
		assert.NoError(t, err)

		assert.False(t, vm.IsProcessRunning(pidPath))
	})

	t.Run("running process", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidPath := filepath.Join(tmpDir, "test.pid")

		// Use current process PID
		err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o644)
		assert.NoError(t, err)

		assert.True(t, vm.IsProcessRunning(pidPath))
	})
}
