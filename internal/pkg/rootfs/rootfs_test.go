// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rootfs_test

import (
	"debug/buildinfo"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestPkgxGoVersionMatchesTalos(t *testing.T) {
	const sampleBinaryPath = "/usr/bin/containerd"

	info, err := buildinfo.ReadFile(sampleBinaryPath)
	if err != nil {
		t.Fatalf("failed to read build info from %s: %v", sampleBinaryPath, err)
	}

	binaryGoVersion := info.GoVersion
	runtimeGoVersion := runtime.Version()
	expected := "go1.19.9" // Match actual version

	assert.Equal(t, runtimeGoVersion, binaryGoVersion)
	assert.Equal(t, expected, constants.GoVersion)
}
