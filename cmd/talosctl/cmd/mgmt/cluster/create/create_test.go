// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create //nolint:testpackage

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/provision"
)

func TestGetDisks(t *testing.T) {
	type args struct {
		disks            []string
		preallocateDisks bool
		diskBlockSize    uint
	}

	tests := []struct {
		name            string
		args            args
		wantPrimary     []*provision.Disk
		wantWorkerExtra []*provision.Disk
		wantErr         bool
	}{
		{
			name: "single disk",
			args: args{
				disks:            []string{"virtio:4096"},
				preallocateDisks: true,
				diskBlockSize:    4096,
			},
			wantPrimary: []*provision.Disk{
				{
					Size:            4096 * 1024 * 1024,
					SkipPreallocate: false,
					Driver:          "virtio",
					BlockSize:       4096,
				},
			},
			wantWorkerExtra: nil,
			wantErr:         false,
		},
		{
			name: "multiple disks",
			args: args{
				disks:            []string{"virtio:4096", "sata:2048", "nvme:1024"},
				preallocateDisks: false,
				diskBlockSize:    8192,
			},
			wantPrimary: []*provision.Disk{
				{
					Size:            4096 * 1024 * 1024,
					SkipPreallocate: true,
					Driver:          "virtio",
					BlockSize:       8192,
				},
			},
			wantWorkerExtra: []*provision.Disk{
				{
					Size:            2048 * 1024 * 1024,
					SkipPreallocate: true,
					Driver:          "sata",
					BlockSize:       8192,
				},
				{
					Size:            1024 * 1024 * 1024,
					SkipPreallocate: true,
					Driver:          "nvme",
					BlockSize:       8192,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid disk format",
			args: args{
				disks:            []string{"badformat"},
				preallocateDisks: false,
				diskBlockSize:    512,
			},
			wantPrimary:     nil,
			wantWorkerExtra: nil,
			wantErr:         true,
		},
		{
			name: "invalid size in disk spec",
			args: args{
				disks:            []string{"virtio:notanumber"},
				preallocateDisks: true,
				diskBlockSize:    512,
			},
			wantPrimary:     nil,
			wantWorkerExtra: nil,
			wantErr:         true,
		},
		{
			name: "no disks specified",
			args: args{
				disks:            []string{},
				preallocateDisks: true,
				diskBlockSize:    512,
			},
			wantPrimary:     nil,
			wantWorkerExtra: nil,
			wantErr:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qOps := qemuOps{
				disks:            tt.args.disks,
				preallocateDisks: tt.args.preallocateDisks,
				diskBlockSize:    tt.args.diskBlockSize,
			}

			gotPrimary, gotWorkerExtra, err := getDisks(qOps)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, gotPrimary, tt.wantPrimary)
			assert.Equal(t, gotWorkerExtra, tt.wantWorkerExtra)
		})
	}
}
