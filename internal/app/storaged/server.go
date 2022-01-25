// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package internal

import (
	"context"

	"github.com/talos-systems/go-blockdevice/blockdevice/util/disk"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/talos-systems/talos/pkg/machinery/api/storage"
)

// Server implements storage.StorageService.
// TODO: this is not a full blown service yet, it's used as the common base in the machine and the maintenance services.
type Server struct {
	storage.UnimplementedStorageServiceServer
}

// Disks implements storage.StorageService.
func (s *Server) Disks(ctx context.Context, in *emptypb.Empty) (reply *storage.DisksResponse, err error) {
	disks, err := disk.List()
	if err != nil {
		return nil, err
	}

	diskList := make([]*storage.Disk, len(disks))

	for i, disk := range disks {
		diskList[i] = &storage.Disk{
			DeviceName: disk.DeviceName,
			Model:      disk.Model,
			Size:       disk.Size,
			Name:       disk.Name,
			Serial:     disk.Serial,
			Modalias:   disk.Modalias,
			Type:       storage.Disk_DiskType(disk.Type),
			BusPath:    disk.BusPath,
		}
	}

	reply = &storage.DisksResponse{
		Messages: []*storage.Disks{
			{
				Disks: diskList,
			},
		},
	}

	return reply, nil
}
