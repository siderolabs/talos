// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package storage implements machine.StorageService.
package storage

import (
	"context"

	"golang.org/x/sys/unix"

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
	var stat unix.Statfs_t

	if err := unix.Statfs(req.Path, &stat); err != nil {
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
