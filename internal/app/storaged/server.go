// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package internal contains server implementation.
package internal

import (
	"context"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// Server implements storage.StorageService.
//
// It is only kept here for compatibility purposes, proper API is to query `block.Disk` resources.
type Server struct {
	storage.UnimplementedStorageServiceServer
	Controller runtime.Controller
}

// Disks implements storage.StorageService.
func (s *Server) Disks(ctx context.Context, in *emptypb.Empty) (reply *storage.DisksResponse, err error) {
	st := s.Controller.Runtime().State().V1Alpha2().Resources()

	systemDisk, err := safe.StateGetByID[*block.SystemDisk](ctx, st, block.SystemDiskID)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	disks, err := safe.StateListAll[*block.Disk](ctx, st)
	if err != nil {
		return nil, err
	}

	diskConv := func(d *block.Disk) *storage.Disk {
		var diskType storage.Disk_DiskType

		switch {
		case d.TypedSpec().CDROM:
			diskType = storage.Disk_CD
		case d.TypedSpec().Transport == "nvme":
			diskType = storage.Disk_NVME
		case d.TypedSpec().Transport == "mmc":
			diskType = storage.Disk_SD
		case d.TypedSpec().Rotational:
			diskType = storage.Disk_HDD
		case d.TypedSpec().Transport != "":
			diskType = storage.Disk_SSD
		}

		return &storage.Disk{
			DeviceName: filepath.Join("/dev", d.Metadata().ID()),
			Model:      d.TypedSpec().Model,
			Size:       d.TypedSpec().Size,
			Serial:     d.TypedSpec().Serial,
			Modalias:   d.TypedSpec().Modalias,
			Wwid:       d.TypedSpec().WWID,
			Type:       diskType,
			BusPath:    d.TypedSpec().BusPath,
			SystemDisk: systemDisk != nil && d.Metadata().ID() == systemDisk.TypedSpec().DiskID,
			Subsystem:  d.TypedSpec().SubSystem,
			Readonly:   d.TypedSpec().Readonly,
		}
	}

	reply = &storage.DisksResponse{
		Messages: []*storage.Disks{
			{
				Disks: safe.ToSlice(disks, diskConv),
			},
		},
	}

	return reply, nil
}
