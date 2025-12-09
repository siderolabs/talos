// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package flags_test

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"

	flags "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/flags"
	"github.com/siderolabs/talos/pkg/bytesize"
)

func TestDisksFlag_ExtraOpts(t *testing.T) {
	t.Parallel()

	var d flags.Disks

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Var(&d, "disks", "")

	args := []string{
		"--disks", "virtio:1GiB:serial=test-1",
		"--disks", "virtiofs:1GiB:tag=foo,virtiofs:1GiB:tag=bar",
	}

	err := fs.Parse(args)
	assert.NoError(t, err)

	reqs := d.Requests()
	assert.Len(t, reqs, 3)

	toBytes := func(s string) uint64 {
		bs := bytesize.WithDefaultUnit("MiB")
		assert.NoError(t, bs.Set(s))

		return bs.Bytes()
	}

	assert.Equal(t, "virtio", reqs[0].Driver)
	assert.Equal(t, toBytes("1GiB"), reqs[0].Size.Bytes())
	assert.Equal(t, "test-1", reqs[0].Serial)
	assert.Equal(t, "", reqs[0].Tag)

	assert.Equal(t, "virtiofs", reqs[1].Driver)
	assert.Equal(t, toBytes("1GiB"), reqs[1].Size.Bytes())
	assert.Equal(t, "", reqs[1].Serial)
	assert.Equal(t, "foo", reqs[1].Tag)

	assert.Equal(t, "virtiofs", reqs[2].Driver)
	assert.Equal(t, toBytes("1GiB"), reqs[2].Size.Bytes())
	assert.Equal(t, "", reqs[2].Serial)
	assert.Equal(t, "bar", reqs[2].Tag)

	// Type should be stable
	assert.Equal(t, "disks", d.Type())

	assert.Equal(t, "virtio:1GiB:serial=test-1,virtiofs:1GiB:tag=foo,virtiofs:1GiB:tag=bar", d.String())
}

func TestDisksFlag_AccumulatesAndRequests(t *testing.T) {
	t.Parallel()

	var d flags.Disks

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Var(&d, "disks", "")

	args := []string{
		"--disks", "virtio:1GiB",
		"--disks", "nvme:10GiB,sata:512MiB",
	}

	err := fs.Parse(args)
	assert.NoError(t, err)

	reqs := d.Requests()
	assert.Len(t, reqs, 3)

	toBytes := func(s string) uint64 {
		bs := bytesize.WithDefaultUnit("MiB")
		assert.NoError(t, bs.Set(s))

		return bs.Bytes()
	}

	assert.Equal(t, "virtio", reqs[0].Driver)
	assert.Equal(t, toBytes("1GiB"), reqs[0].Size.Bytes())

	assert.Equal(t, "nvme", reqs[1].Driver)
	assert.Equal(t, toBytes("10GiB"), reqs[1].Size.Bytes())

	assert.Equal(t, "sata", reqs[2].Driver)
	assert.Equal(t, toBytes("512MiB"), reqs[2].Size.Bytes())

	// Type should be stable
	assert.Equal(t, "disks", d.Type())

	assert.Equal(t, "virtio:1GiB,nvme:10GiB,sata:512MiB", d.String())
}

func TestDisksFlag_SetInvalid(t *testing.T) {
	t.Parallel()

	var d flags.Disks

	err := d.Set("invalid-no-colon")
	assert.Error(t, err)
}
