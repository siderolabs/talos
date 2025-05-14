// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create_test

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
)

func runCmd(cmd *cobra.Command, args ...string) (*cobra.Command, string, error) { //nolint:unparam
	outBuf := bytes.NewBufferString("")
	cmd.SetOut(outBuf)
	cmd.SetErr(outBuf)
	cmd.SetArgs(args)
	c, err := cmd.ExecuteC()

	return c, outBuf.String(), err
}

func TestCreateCommandInvalidProvisioner(t *testing.T) {
	_, _, err := runCmd(cluster.Cmd, "create", "--provisioner=asd")
	assert.ErrorContains(t, err, "unsupported provisioner")
}

func TestCreateCommandInvalidProvisionerFlagQemu(t *testing.T) {
	_, _, err := runCmd(cluster.Cmd, "create", "--provisioner=qemu", "--docker-disable-ipv6=true")
	assert.ErrorContains(t, err, "docker-disable-ipv6 flag has been set but has no effect with the qemu provisioner")
}

func TestCreateCommandInvalidProvisionerFlagDocker(t *testing.T) {
	_, _, err := runCmd(cluster.Cmd, "create", "--provisioner=docker", "--with-network-chaos=true")
	assert.ErrorContains(t, err, "with-network-chaos flag has been set but has no effect with the docker provisioner")
}
