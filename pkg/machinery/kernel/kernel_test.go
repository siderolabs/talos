// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kernel_test

import (
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/kernel"
)

func TestParamPath(t *testing.T) {
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
			if got := tt.param.Path(); got != tt.want {
				t.Errorf("Param.Path() = %v, want %v", got, tt.want)
			}
		})
	}
}
