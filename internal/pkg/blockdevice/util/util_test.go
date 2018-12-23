//nolint: scopelint
package util

import (
	"testing"
)

func Test_PartNo(t *testing.T) {
	type args struct {
		devname string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "hda1",
			args: args{
				devname: "hda1",
			},
			want: "1",
		},
		{
			name: "hda10",
			args: args{
				devname: "hda10",
			},
			want: "10",
		},
		{
			name: "sda1",
			args: args{
				devname: "sda1",
			},
			want: "1",
		},
		{
			name: "sda10",
			args: args{
				devname: "sda10",
			},
			want: "10",
		},
		{
			name: "nvme1n2p2",
			args: args{
				devname: "nvme1n2p2",
			},
			want: "2",
		},
		{
			name: "nvme1n2p11",
			args: args{
				devname: "nvme1n2p11",
			},
			want: "11",
		},
		{
			name: "vda1",
			args: args{
				devname: "vda1",
			},
			want: "1",
		},
		{
			name: "vda10",
			args: args{
				devname: "vda10",
			},
			want: "10",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// nolint: errcheck
			if got, _ := PartNo(tt.args.devname); got != tt.want {
				t.Errorf("PartNo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_DevnameFromPartname(t *testing.T) {
	type args struct {
		devname string
		partno  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "hda1",
			args: args{
				devname: "hda1",
				partno:  "1",
			},
			want: "hda",
		},
		{
			name: "sda1",
			args: args{
				devname: "sda1",
				partno:  "1",
			},
			want: "sda",
		},
		{
			name: "vda1",
			args: args{
				devname: "vda1",
				partno:  "1",
			},
			want: "vda",
		},
		{
			name: "nvme1n2p11",
			args: args{
				devname: "nvme1n2p11",
				partno:  "11",
			},
			want: "nvme1n2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// nolint: errcheck
			if got, _ := DevnameFromPartname(tt.args.devname); got != tt.want {
				t.Errorf("DevnameFromPartname() = %v, want %v", got, tt.want)
			}
		})
	}
}
