// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create //nolint:testpackage

import (
	"context"
	"testing"

	"github.com/docker/cli/opts"
	"github.com/stretchr/testify/assert"
)

func TestNodeImageParam(t *testing.T) {
	cm := getTestClustermaker()

	err := _createDockerCluster(context.Background(), dockerOps{nodeImage: "test-image"}, &cm)
	assert.NoError(t, err)

	assert.Equal(t, "test-image", cm.finalReq.Image)
}

func TestHostIpParam(t *testing.T) {
	cm := getTestClustermaker()

	err := _createDockerCluster(context.Background(), dockerOps{dockerHostIP: "1.1.1.1"}, &cm)
	assert.NoError(t, err)
	result, err := cm.getProvisionOpts()
	assert.NoError(t, err)

	assert.Equal(t, "1.1.1.1", result.DockerPortsHostIP)
}

func TestPortsParam(t *testing.T) {
	cm := getTestClustermaker()

	err := _createDockerCluster(context.Background(), dockerOps{ports: "20:20,33:30"}, &cm)
	assert.NoError(t, err)
	result, err := cm.getProvisionOpts()
	assert.NoError(t, err)

	assert.Equal(t, []string{"20:20", "33:30"}, result.DockerPorts)
}

func TestMountsParam(t *testing.T) {
	cm := getTestClustermaker()
	mount := opts.MountOpt{}
	err := mount.Set("type=tmpfs,destination=/run")
	assert.NoError(t, err)

	err = _createDockerCluster(context.Background(), dockerOps{mountOpts: mount}, &cm)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(cm.finalReq.Nodes[0].Mounts))
	assert.Equal(t, "/run", cm.finalReq.Nodes[0].Mounts[0].Target)
	assert.Equal(t, 1, len(cm.finalReq.Nodes[3].Mounts))
	assert.Equal(t, "/run", cm.finalReq.Nodes[3].Mounts[0].Target)
}

func TestDisableIPv6Param(t *testing.T) {
	cm := getTestClustermaker()

	err := _createDockerCluster(context.Background(), dockerOps{dockerDisableIPv6: true}, &cm)
	assert.NoError(t, err)

	assert.Equal(t, true, cm.finalReq.Network.DockerDisableIPv6)
}
