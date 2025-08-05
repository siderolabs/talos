// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rootfs_test

import (
	"debug/buildinfo"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestPkgsGoVersionMatchesTalos(t *testing.T) {
	t.Parallel()

	if hostname, _ := os.Hostname(); hostname != "buildkitsandbox" { //nolint:errcheck
		t.Skipf("skipping test; only run on buildkitsandbox, got %s", hostname)
	}

	const sampleBinaryPath = "/usr/bin/containerd"

	info, err := buildinfo.ReadFile(sampleBinaryPath)
	if err != nil {
		t.Fatalf("failed to read build info from %s: %v", sampleBinaryPath, err)
	}

	binaryGoVersion := info.GoVersion
	runtimeGoVersion := runtime.Version()

	assert.Equal(t, runtimeGoVersion, binaryGoVersion)
	assert.Equal(t, runtimeGoVersion, constants.GoVersion)
}
