// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package flags_test

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"

	flags "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/flags"
)

func TestVirtiofsFlag_AccumulatesAndRequests(t *testing.T) {
	t.Parallel()

	var d flags.Virtiofs

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Var(&d, "virtiofs", "")

	args := []string{
		"--virtiofs", "/mnt/shared/1:/tmp/mnt-shared-1.sock,/mnt/shared/2:/tmp/mnt-shared-2.sock,/mnt/shared/3:/tmp/mnt-shared-3.sock",
	}

	err := fs.Parse(args)
	assert.NoError(t, err)

	reqs := d.Requests()
	assert.Len(t, reqs, 3)

	assert.Equal(t, "/mnt/shared/1", reqs[0].SharedDir)
	assert.Equal(t, "/tmp/mnt-shared-1.sock", reqs[0].SocketPath)

	assert.Equal(t, "/mnt/shared/2", reqs[1].SharedDir)
	assert.Equal(t, "/tmp/mnt-shared-2.sock", reqs[1].SocketPath)

	assert.Equal(t, "/mnt/shared/3", reqs[2].SharedDir)
	assert.Equal(t, "/tmp/mnt-shared-3.sock", reqs[2].SocketPath)

	// Type should be stable
	assert.Equal(t, "virtiofs", d.Type())

	assert.Equal(t, "/mnt/shared/1:/tmp/mnt-shared-1.sock,/mnt/shared/2:/tmp/mnt-shared-2.sock,/mnt/shared/3:/tmp/mnt-shared-3.sock", d.String())
}

func TestVirtiofsFlag_SetInvalid(t *testing.T) {
	t.Parallel()

	var f flags.Virtiofs

	err := f.Set("/mnt/shared/1:/tmp/mnt-shared-1.sock")
	assert.NoError(t, err)

	assert.Equal(t, "/mnt/shared/1:/tmp/mnt-shared-1.sock", f.String())

	err = f.Set("/mnt/shared/1:/tmp/mnt-shared-1.sock,/mnt/shared/2:/tmp/mnt-shared-2.sock")
	assert.NoError(t, err)

	assert.Equal(t, "/mnt/shared/1:/tmp/mnt-shared-1.sock,/mnt/shared/2:/tmp/mnt-shared-2.sock", f.String())

	err = f.Set("invalid-no-colon")
	assert.Error(t, err)
}
