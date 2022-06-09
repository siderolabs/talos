// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package internal

import (
	"context"

	"github.com/talos-systems/go-blockdevice/blockdevice/util/disk"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/talos-systems/talos/pkg/machinery/api/storage"
	"github.com/talos-systems/talos/pkg/machinery/generic/slices"
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

	diskConv := func(d *disk.Disk) *storage.Disk {
		return &storage.Disk{
			DeviceName: d.DeviceName,
			Model:      d.Model,
			Size:       d.Size,
			Name:       d.Name,
			Serial:     d.Serial,
			Modalias:   d.Modalias,
			Type:       storage.Disk_DiskType(d.Type),
			BusPath:    d.BusPath,
		}
	}

	diskList := slices.Map(disks, diskConv)

	reply = &storage.DisksResponse{
		Messages: []*storage.Disks{
			{
				Disks: diskList,
			},
		},
	}

	return reply, nil
}
