// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package internal contains server implementation.
package internal

import (
	"context"

	"github.com/siderolabs/gen/slices"
	bddisk "github.com/siderolabs/go-blockdevice/blockdevice/util/disk"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
)

// Server implements storage.StorageService.
// TODO: this is not a full blown service yet, it's used as the common base in the machine and the maintenance services.
type Server struct {
	storage.UnimplementedStorageServiceServer
	Controller runtime.Controller
}

// Disks implements storage.StorageService.
func (s *Server) Disks(ctx context.Context, in *emptypb.Empty) (reply *storage.DisksResponse, err error) {
	disks, err := bddisk.List()
	if err != nil {
		return nil, err
	}

	systemDisk := s.Controller.Runtime().State().Machine().Disk()

	diskConv := func(d *bddisk.Disk) *storage.Disk {
		return &storage.Disk{
			DeviceName: d.DeviceName,
			Model:      d.Model,
			Size:       d.Size,
			Name:       d.Name,
			Serial:     d.Serial,
			Modalias:   d.Modalias,
			Uuid:       d.UUID,
			Wwid:       d.WWID,
			Type:       storage.Disk_DiskType(d.Type),
			BusPath:    d.BusPath,
			SystemDisk: systemDisk != nil && d.DeviceName == systemDisk.Device().Name(),
			Subsystem:  d.SubSystem,
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
