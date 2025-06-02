// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kernel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
)

func TestParamPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		param *kernel.Param
		want  string
	}{
		{
			name: "Test Sysfs Path",
			param: &kernel.Param{
				Key: kernel.Sysfs + ".block.sda.queue.scheduler",
			},
			want: "/sys/block/sda/queue/scheduler",
		},
		{
			name: "Test Sysctl Path",
			param: &kernel.Param{
				Key: kernel.Sysctl + ".net.ipv6.conf.eth0.accept_ra",
			},
			want: "/proc/sys/net/ipv6/conf/eth0/accept_ra",
		},
		{
			name: "Test Sysctl Path with vlan interface untouched",
			param: &kernel.Param{
				Key: kernel.Sysctl + ".net/ipv6/conf/eth0.103/disable_ipv6",
			},
			want: "/proc/sys/net/ipv6/conf/eth0.103/disable_ipv6",
		},
		{
			name: "Test Sysctl Path with vlan interface inverted",
			param: &kernel.Param{
				Key: kernel.Sysctl + ".net.ipv6.conf.eth0/103.disable_ipv6",
			},
			want: "/proc/sys/net/ipv6/conf/eth0.103/disable_ipv6",
		},
		{
			name: "Test Sysctl Path with invalid symbols which translate to '..'",
			param: &kernel.Param{
				Key: kernel.Sysctl + ".net.ipv6.conf.eth0/103.//.disable_ipv6",
			},
			want: "/proc/sys/net/ipv6/conf/disable_ipv6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.param.Path(); got != tt.want {
				t.Errorf("Param.Path() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultKernelArgs(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		quirks quirks.Quirks

		expected []string
	}{
		{
			name: "latest",

			expected: []string{
				"init_on_alloc=1",
				"slab_nomerge=",
				"pti=on",
				"consoleblank=0",
				"nvme_core.io_timeout=4294967295",
				"printk.devkmsg=on",
				"selinux=1",
			},
		},
		{
			name: "v1.9",

			quirks: quirks.New("v1.9.0"),

			expected: []string{
				"init_on_alloc=1",
				"slab_nomerge=",
				"pti=on",
				"consoleblank=0",
				"nvme_core.io_timeout=4294967295",
				"printk.devkmsg=on",
				"ima_template=ima-ng",
				"ima_appraise=fix",
				"ima_hash=sha512",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, kernel.DefaultArgs(test.quirks))
		})
	}
}

func TestSecureBootArgs(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		quirks quirks.Quirks

		expected []string
	}{
		{
			name: "latest",

			expected: []string{
				"lockdown=confidentiality",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, kernel.SecureBootArgs(test.quirks))
		})
	}
}
