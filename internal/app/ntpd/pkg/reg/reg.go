/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/talos-systems/talos/internal/app/ntpd/pkg/ntp"
	"github.com/talos-systems/talos/internal/app/ntpd/proto"
	"google.golang.org/grpc"
)

// Registrator is the concrete type that implements the factory.Registrator and
// proto.Init interfaces.
type Registrator struct {
	Ntpd *ntp.NTP
}

// NewRegistrator builds new Registrator instance
func NewRegistrator(n *ntp.NTP) *Registrator {
	return &Registrator{
		Ntpd: n,
	}
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	proto.RegisterNtpdServer(s, r)
}

// Time issues a query to the configured ntp server and displays the results
func (r *Registrator) Time(ctx context.Context, in *empty.Empty) (reply *proto.TimeReply, err error) {
	reply = &proto.TimeReply{}
	rt, err := r.Ntpd.Query()
	if err != nil {
		return reply, err
	}

	return genProtobufTimeReply(r.Ntpd.GetTime(), rt.Time, r.Ntpd.Server)
}

// TimeCheck issues a query to the specified ntp server and displays the results
func (r *Registrator) TimeCheck(ctx context.Context, in *proto.TimeRequest) (reply *proto.TimeReply, err error) {
	reply = &proto.TimeReply{}
	tc, err := ntp.NewNTPClient(ntp.WithServer(in.Server))
	if err != nil {
		return reply, err
	}

	rt, err := tc.Query()
	if err != nil {
		return reply, err
	}

	return genProtobufTimeReply(tc.GetTime(), rt.Time, in.Server)
}

func genProtobufTimeReply(local, remote time.Time, server string) (*proto.TimeReply, error) {
	reply := &proto.TimeReply{}

	localpbts, err := ptypes.TimestampProto(local)
	if err != nil {
		return reply, err
	}

	remotepbts, err := ptypes.TimestampProto(remote)
	if err != nil {
		return reply, err
	}

	reply = &proto.TimeReply{
		Server:     server,
		Localtime:  localpbts,
		Remotetime: remotepbts,
	}

	return reply, nil
}
