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
				devname: "/dev/hda1",
			},
			want: "1",
		},
		{
			name: "hda10",
			args: args{
				devname: "/dev/hda10",
			},
			want: "10",
		},
		{
			name: "sda1",
			args: args{
				devname: "/dev/sda1",
			},
			want: "1",
		},
		{
			name: "sda10",
			args: args{
				devname: "/dev/sda10",
			},
			want: "10",
		},
		{
			name: "nvme1n2p2",
			args: args{
				devname: "/dev/nvme1n2p2",
			},
			want: "2",
		},
		{
			name: "nvme1n2p11",
			args: args{
				devname: "/dev/nvme1n2p11",
			},
			want: "11",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PartNo(tt.args.devname); got != tt.want {
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
				devname: "/dev/hda1",
				partno:  PartNo("/dev/hda1"),
			},
			want: "/dev/hda",
		},
		{
			name: "nvme1n2p11",
			args: args{
				devname: "/dev/nvme1n2p11",
				partno:  PartNo("/dev/nvme1n2p11"),
			},
			want: "/dev/nvme1n2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DevnameFromPartname(tt.args.devname, tt.args.partno); got != tt.want {
				t.Errorf("DevnameFromPartname() = %v, want %v", got, tt.want)
			}
		})
	}
}
