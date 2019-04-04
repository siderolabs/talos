/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/talos-systems/talos/internal/app/blockd/proto"
	"github.com/talos-systems/talos/pkg/userdata"
	"google.golang.org/grpc"
)

// Registrator is the concrete type that implements the factory.Registrator and
// proto.BlockdServer interfaces.
type Registrator struct {
	Data *userdata.OSSecurity
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	proto.RegisterBlockdServer(s, r)
}

// Resize implements the proto.BlockdServer interface.
func (r *Registrator) Resize(ctx context.Context, in *proto.ResizePartitionRequest) (reply *empty.Empty, err error) {
	return nil, nil
}
