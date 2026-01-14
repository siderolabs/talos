// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package internal contains server implementation.
package internal

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"slices"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	blockdev "github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// Server implements storage.StorageService.
//
// It is only kept here for compatibility purposes, proper API is to query `block.Disk` resources.
type Server struct {
	storage.UnimplementedStorageServiceServer

	Controller      runtime.Controller
	MaintenanceMode bool
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
			Uuid:       d.TypedSpec().UUID,
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

// BlockDeviceWipe implements storage.StorageService.
//
// It allows to wipe unused block devices, for blockdevices in use (volumes), use a different method.
func (s *Server) BlockDeviceWipe(ctx context.Context, req *storage.BlockDeviceWipeRequest) (*storage.BlockDeviceWipeResponse, error) {
	// the storage server is included both into machined and maintenance service
	// in apid/machined mode, the normal authz checks are used before reaching this method
	// in maintenance mode, we allow this method to be accessible, as it only allows to wipe block devices
	//
	// validate the list of devices
	for _, deviceRequest := range req.GetDevices() {
		if err := s.validateDeviceForWipe(ctx, deviceRequest.GetDevice(), deviceRequest.GetSkipVolumeCheck(), deviceRequest.GetSkipSecondaryCheck()); err != nil {
			return nil, err
		}
	}

	// perform the actual wipe
	for _, deviceRequest := range req.GetDevices() {
		if err := s.wipeDevice(deviceRequest.GetDevice(), deviceRequest.GetMethod(), deviceRequest.GetDropPartition()); err != nil {
			return nil, err
		}
	}

	return &storage.BlockDeviceWipeResponse{
		Messages: []*storage.BlockDeviceWipe{
			{},
		},
	}, nil
}

//nolint:gocyclo,cyclop
func (s *Server) validateDeviceForWipe(ctx context.Context, deviceName string, skipVolumeCheck, skipSecondaryCheck bool) error {
	// first, resolve the blockdevice and figure out what type it is
	st := s.Controller.Runtime().State().V1Alpha2().Resources()

	blockdevice, err := safe.StateGetByID[*block.Device](ctx, st, deviceName)
	if err != nil {
		if state.IsNotFoundError(err) {
			return status.Errorf(codes.NotFound, "blockdevice %q not found", deviceName)
		}

		return err
	}

	var parent string

	deviceType := blockdevice.TypedSpec().Type

	switch deviceType {
	case block.DeviceTypeDisk: // supported
	case block.DeviceTypePartition: // supported
		parent = blockdevice.TypedSpec().Parent
	default:
		return status.Errorf(codes.InvalidArgument, "blockdevice %q is of unsupported type %q", deviceName, deviceType)
	}

	// check the disk (or parent)
	var disk *block.Disk

	if parent != "" {
		disk, err = safe.StateGetByID[*block.Disk](ctx, st, parent)
	} else {
		disk, err = safe.StateGetByID[*block.Disk](ctx, st, deviceName)
	}

	if err != nil {
		return fmt.Errorf("failed to get disk (or parent) for %q: %w", deviceName, err)
	}

	if disk.TypedSpec().Readonly {
		return status.Errorf(codes.FailedPrecondition, "blockdevice %q is read-only", deviceName)
	}

	if disk.TypedSpec().CDROM {
		return status.Errorf(codes.FailedPrecondition, "blockdevice %q is a CD-ROM", deviceName)
	}

	// secondaries check
	if !skipSecondaryCheck {
		switch deviceType {
		case block.DeviceTypeDisk: // for disks, check secondaries even if the partition is used as secondary (track via Disk resource)
			disks, err := safe.StateListAll[*block.Disk](ctx, st)
			if err != nil {
				return err
			}

			for disk := range disks.All() {
				if slices.Index(disk.TypedSpec().SecondaryDisks, deviceName) != -1 {
					return status.Errorf(codes.FailedPrecondition, "blockdevice %q is in use by disk %q", deviceName, disk.Metadata().ID())
				}
			}
		case block.DeviceTypePartition: // for partitions, check secondaries only if the partition is used as a secondary
			blockdevices, err := safe.StateListAll[*block.Device](ctx, st)
			if err != nil {
				return err
			}

			for blockdevice := range blockdevices.All() {
				if slices.Index(blockdevice.TypedSpec().Secondaries, deviceName) != -1 {
					return status.Errorf(codes.FailedPrecondition, "blockdevice %q is in use by blockdevice %q", deviceName, blockdevice.Metadata().ID())
				}
			}
		}
	}

	// volume in use checks
	if !skipVolumeCheck {
		volumeStatuses, err := safe.StateListAll[*block.VolumeStatus](ctx, st)
		if err != nil {
			return err
		}

		for volumeStatus := range volumeStatuses.All() {
			for _, location := range []string{
				filepath.Base(volumeStatus.TypedSpec().Location),
				filepath.Base(volumeStatus.TypedSpec().MountLocation),
			} {
				for _, dev := range []string{deviceName, parent} {
					if dev == "" || location == "" {
						continue
					}

					if location == dev {
						return status.Errorf(codes.FailedPrecondition, "blockdevice %q is in use by volume %q", dev, volumeStatus.Metadata().ID())
					}
				}
			}

			if filepath.Base(volumeStatus.TypedSpec().ParentLocation) == deviceName {
				return status.Errorf(codes.FailedPrecondition, "blockdevice %q is in use by volume %q", deviceName, volumeStatus.Metadata().ID())
			}
		}
	}

	return nil
}

func (s *Server) findParentDevice(deviceName string) (string, int, error) {
	st := s.Controller.Runtime().State().V1Alpha2().Resources()

	blockdevice, err := safe.StateGetByID[*block.Device](context.Background(), st, deviceName)
	if err != nil {
		return "", 0, err
	}

	if blockdevice.TypedSpec().Type == block.DeviceTypePartition {
		return blockdevice.TypedSpec().Parent, blockdevice.TypedSpec().PartitionNumber, nil
	}

	return "", 0, nil
}

// wipeDevice wipes the block device with the given method.
//
//nolint:gocyclo,cyclop
func (s *Server) wipeDevice(deviceName string, method storage.BlockDeviceWipeDescriptor_Method, dropPartition bool) error {
	parentName, partitionNumber, err := s.findParentDevice(deviceName)
	if err != nil {
		return err
	}

	var parentBd *blockdev.Device

	if parentName != "" {
		parentBd, err = blockdev.NewFromPath(filepath.Join("/dev", parentName), blockdev.OpenForWrite())
		if err != nil {
			return status.Errorf(codes.Internal, "failed to open block device %q: %v", parentName, err)
		}

		defer parentBd.Close() //nolint:errcheck
	}

	bd, err := blockdev.NewFromPath(filepath.Join("/dev", deviceName), blockdev.OpenForWrite())
	if err != nil {
		return status.Errorf(codes.Internal, "failed to open block device %q: %v", deviceName, err)
	}

	defer bd.Close() //nolint:errcheck

	// lock the parent device always (if available)
	if parentBd != nil {
		log.Printf("locking parent block device %q", parentName)

		if err = parentBd.Lock(true); err != nil {
			return status.Errorf(codes.Internal, "failed to lock parent block device %q: %v", parentName, err)
		}

		defer parentBd.Unlock() //nolint:errcheck
	} else {
		log.Printf("locking block device %q", deviceName)

		if err = bd.Lock(true); err != nil {
			return status.Errorf(codes.Internal, "failed to lock block device %q: %v", deviceName, err)
		}

		defer bd.Unlock() //nolint:errcheck
	}

	switch method {
	case storage.BlockDeviceWipeDescriptor_ZEROES:
		log.Printf("wiping block device %q with zeroes", deviceName)

		method, err := bd.Wipe()
		if err != nil {
			return status.Errorf(codes.Internal, "failed to wipe block device %q: %v", deviceName, err)
		}

		log.Printf("block device %q wiped with method %q", deviceName, method)
	case storage.BlockDeviceWipeDescriptor_FAST:
		log.Printf("wiping block device %q with fast method", deviceName)

		if err = partition.WipeWithSignatures(bd, deviceName, log.Printf); err != nil {
			return status.Error(codes.Internal, err.Error())
		}
	default:
		return status.Errorf(codes.InvalidArgument, "unsupported wipe method %s", method)
	}

	if dropPartition && parentBd != nil && partitionNumber != 0 {
		// first, close the blockdevice, otherwise the partition table cannot be modified
		if err = bd.Close(); err != nil {
			return status.Errorf(codes.Internal, "failed to close block device %q: %v", deviceName, err)
		}

		gptdev, err := gpt.DeviceFromBlockDevice(parentBd)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to get GPT device: %v", err)
		}

		pt, err := gpt.Read(gptdev)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to read GPT table: %v", err)
		}

		if err = pt.DeletePartition(partitionNumber - 1); err != nil {
			return status.Errorf(codes.Internal, "failed to delete partition: %v", err)
		}

		if err = pt.Write(); err != nil {
			return status.Errorf(codes.Internal, "failed to write GPT table: %v", err)
		}

		log.Printf("deleted partition %d from block device %q", partitionNumber, parentName)
	}

	return nil
}
