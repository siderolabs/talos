// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package storage implements machine.StorageService.
package storage

import (
	"context"
	"errors"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// Service implements machine.StorageService.
type Service struct {
	machine.UnimplementedStorageServiceServer
}

// NewService creates a new StorageService.
func NewService() *Service {
	return &Service{}
}

// Statfs returns information about a mounted file system.
func (svc *Service) Statfs(ctx context.Context, req *machine.StorageServiceStatfsRequest) (*machine.StorageServiceStatfsResponse, error) {
	if req.GetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "path is required")
	}

	var stat unix.Statfs_t

	if err := unix.Statfs(req.Path, &stat); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil, status.Error(codes.NotFound, "path not found")
		}

		return nil, err
	}

	bsize := uint64(stat.Bsize)

	return &machine.StorageServiceStatfsResponse{
		Size:       stat.Blocks * bsize,
		Used:       (stat.Blocks - stat.Bfree) * bsize,
		Available:  stat.Bavail * bsize,
		Inodes:     stat.Files,
		InodesUsed: stat.Files - stat.Ffree,
		InodesFree: stat.Ffree,
	}, nil
}
