/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package net

import (
	"net"
	"reflect"
	"testing"

	"gotest.tools/assert"
)

func TestEmpty(t *testing.T) {
	// added for accurate coverage estimation
	//
	// please remove it once any unit-test is added
	// for this package
}

func TestFormatAddress(t *testing.T) {
	assert.Equal(t, FormatAddress("2001:db8::1"), "[2001:db8::1]")
	assert.Equal(t, FormatAddress("[2001:db8::1]"), "[2001:db8::1]")
	assert.Equal(t, FormatAddress("192.168.1.1"), "192.168.1.1")
	assert.Equal(t, FormatAddress("alpha.beta.gamma.com"), "alpha.beta.gamma.com")
}

// nolint: scopelint
func TestNthIPInNetwork(t *testing.T) {
	type args struct {
		network *net.IPNet
		n       int
	}

	tests := []struct {
		name string
		args args
		want net.IP
	}{
		{
			name: "increment IPv4 by 1",
			args: args{
				network: &net.IPNet{
					IP:   net.IP{10, 96, 0, 0},
					Mask: net.IPMask{255, 255, 255, 0},
				},
				n: 1,
			},
			want: net.IP{10, 96, 0, 1},
		},
		{
			name: "increment IPv4 by 10",
			args: args{
				network: &net.IPNet{
					IP:   net.IP{10, 96, 0, 0},
					Mask: net.IPMask{255, 255, 255, 0},
				},
				n: 10,
			},
			want: net.IP{10, 96, 0, 10},
		},
		{
			name: "increment IPv6 by 1",
			args: args{
				network: &net.IPNet{
					IP:   net.ParseIP("2001:db8:a0b:12f0::1"),
					Mask: net.IPMask{255, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				},
				n: 1,
			},
			want: net.ParseIP("2001:db8:a0b:12f0::2"),
		},
		{
			name: "increment IPv6 by 10",
			args: args{
				network: &net.IPNet{
					IP:   net.ParseIP("2001:db8:a0b:12f0::1"),
					Mask: net.IPMask{255, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				},
				n: 10,
			},
			want: net.ParseIP("2001:db8:a0b:12f0::b"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NthIPInNetwork(tt.args.network, tt.args.n)
			if err != nil {
				t.Errorf("%v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NthFromIP() = %v, want %v", got, tt.want)
			}
		})
	}
}
