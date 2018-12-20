package reg

import (
	"context"

	"github.com/autonomy/talos/internal/app/blockd/proto"
	"github.com/autonomy/talos/internal/pkg/userdata"
	"github.com/golang/protobuf/ptypes/empty"
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
