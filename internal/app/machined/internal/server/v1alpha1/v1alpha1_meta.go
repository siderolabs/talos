// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"io/fs"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// MetaWrite implements the machine.MachineServer interface.
func (s *Server) MetaWrite(ctx context.Context, req *machine.MetaWriteRequest) (*machine.MetaWriteResponse, error) {
	if err := s.checkSupported(runtime.MetaKV); err != nil {
		return nil, err
	}

	if uint32(uint8(req.Key)) != req.Key {
		return nil, status.Errorf(codes.InvalidArgument, "key must be a uint8")
	}

	ok, err := s.Controller.Runtime().State().Machine().Meta().SetTagBytes(ctx, uint8(req.Key), req.Value)
	if err != nil {
		return nil, err
	}

	if !ok {
		// META overflowed
		return nil, status.Errorf(codes.ResourceExhausted, "meta write failed")
	}

	err = s.Controller.Runtime().State().Machine().Meta().Flush()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		// ignore not exist error, as it's possible that the meta partition is not created yet
		return nil, err
	}

	return &machine.MetaWriteResponse{
		Messages: []*machine.MetaWrite{
			{},
		},
	}, nil
}

// MetaDelete implements the machine.MachineServer interface.
func (s *Server) MetaDelete(ctx context.Context, req *machine.MetaDeleteRequest) (*machine.MetaDeleteResponse, error) {
	if err := s.checkSupported(runtime.MetaKV); err != nil {
		return nil, err
	}

	if uint32(uint8(req.Key)) != req.Key {
		return nil, status.Errorf(codes.InvalidArgument, "key must be a uint8")
	}

	ok, err := s.Controller.Runtime().State().Machine().Meta().DeleteTag(ctx, uint8(req.Key))
	if err != nil {
		return nil, err
	}

	if !ok {
		// META key not found
		return nil, status.Errorf(codes.NotFound, "meta key not found")
	}

	err = s.Controller.Runtime().State().Machine().Meta().Flush()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		// ignore not exist error, as it's possible that the meta partition is not created yet
		return nil, err
	}

	return &machine.MetaDeleteResponse{
		Messages: []*machine.MetaDelete{
			{},
		},
	}, nil
}
