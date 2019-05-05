/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package bully

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/bully/proto"
	"google.golang.org/grpc"
)

// Address represents the address of a bully.
type Address = string

// Pid represents a process pid.
type Pid = uint32

// Processes represents the set of processes that form the cluster.
type Processes struct {
	*sync.RWMutex

	internal map[Pid]*Process
}

// Process represents a bully process.
type Process struct {
	*proto.Process

	conn   *grpc.ClientConn
	client proto.BullyClient
}

// Bully represents the bully algorithm.
type Bully struct {
	Processes *Processes

	broadcast   chan *proto.MessageWithPid
	addrs       []Address
	self        *Process
	coordinator *Process
}

// NewBullyServer initializes and returns Bully.
func NewBullyServer(pid Pid, addr Address, addrs ...Address) *Bully {
	procs := &Processes{
		RWMutex:  &sync.RWMutex{},
		internal: map[Pid]*Process{},
	}

	self := &Process{
		Process: &proto.Process{
			Address: addr,
			Pid:     &proto.PidValue{Value: pid},
		},
	}

	bully := &Bully{
		Processes: procs,
		broadcast: make(chan *proto.MessageWithPid, len(addrs)*3),
		addrs:     addrs,
		self:      self,
	}

	return bully
}

// Start registers and starts the gRPC server.
func (b *Bully) Start() error {
	l, err := net.Listen("unix", b.self.Process.Address)
	if err != nil {
		return err
	}

	s := grpc.NewServer()
	proto.RegisterBullyServer(s, b)

	return s.Serve(l)
}

// Join initializes a bully's set of processes.
func (b *Bully) Join() error {
	for _, addr := range b.addrs {
		sock := "unix://" + addr
		conn, err := grpc.Dial(sock, grpc.WithInsecure())
		if err != nil {
			return err
		}

		c := proto.NewBullyClient(conn)
		reply, err := c.Pid(context.Background(), &empty.Empty{})
		if err != nil {
			return err
		}

		proc := &Process{
			Process: &proto.Process{
				Address: addr,
				Pid:     reply,
			},
			conn:   conn,
			client: c,
		}

		if err = b.Processes.Add(proc); err != nil {
			return errors.Wrap(err, "failed to add process to peers")
		}
	}

	return nil
}

// Broadcast sends a message to the appropriate set of bullies.
func (b *Bully) Broadcast(ctx context.Context, msg *proto.MessageWithPid) {
	procs := []*Process{}

	switch msg.Message {
	case proto.Message_COORDINATOR:
		// We need to broadcast the coordinator to all known processes.
		for _, process := range b.Processes.internal {
			procs = append(procs, process)
		}
	default:
		// The bully algorithm states we need only to send messages to pids
		// greater than the current process pid.
		for _, process := range b.Processes.internal {
			if process.Pid.Value > b.self.Pid.Value {
				procs = append(procs, process)
			}
		}
	}

	for _, process := range procs {
		log.Printf("[%d] BROADCASTING %s to [%d]", b.self.Pid.Value, msg.Message.String(), process.Pid.Value)
		reply, err := process.client.Send(ctx, msg)
		if err != nil {
			log.Printf("failed to send: %v", err)
		}
		b.broadcast <- reply
	}
}

// Elect implements the gRPC Bully server interface. It sends an ELECTION
// message to all processes known to the bully and waits for a COORDINATOR message.
func (b *Bully) Elect(ctx context.Context, in *empty.Empty) (*empty.Empty, error) {
	go b.Broadcast(ctx, &proto.MessageWithPid{Pid: b.self.Pid, Message: proto.Message_ELECTION})

	// Set a timer for electing the current bully as the coordinator.
	timer := time.NewTimer(500 * time.Millisecond)

	for {
		select {
		case msg := <-b.broadcast:
			log.Printf("[%d] RECEIVED %s from [%d]", b.self.Pid.Value, msg.Message.String(), msg.Pid.Value)
			switch msg.Message {
			case proto.Message_OK:
				// There is nothing to do, a process with a higher PID is alive.
				// Stop the timer, and wait for a coordinator to be broadcasted.
				timer.Stop()
			case proto.Message_COORDINATOR:
				// Set the current bully's coordinator to the broadcasted
				// process.
				b.coordinator = b.Processes.Get(msg.Pid.Value)
				return &empty.Empty{}, nil
			}
		case <-timer.C:
			// No other proccesses responded, so we assume that the current
			// bully is the coordinator.
			log.Printf("[%d] ELECTED SELF", b.self.Pid.Value)
			b.coordinator = b.self
			b.Broadcast(context.Background(), &proto.MessageWithPid{Pid: b.self.Pid, Message: proto.Message_COORDINATOR})
			return &empty.Empty{}, nil
		}
	}
}

// Pid implements the gRPC Bully server interface. It returns the current
// bully's pid.
func (b *Bully) Pid(ctx context.Context, in *empty.Empty) (*proto.PidValue, error) {
	return b.self.Pid, nil
}

// Send implements the gRPC Bully server interface. It sends a message to
// another process.
func (b *Bully) Send(ctx context.Context, in *proto.MessageWithPid) (*proto.MessageWithPid, error) {
	var msg proto.Message
	switch in.Message {
	case proto.Message_ELECTION:
		msg = proto.Message_OK
	case proto.Message_COORDINATOR:
		msg = proto.Message_OK
		// NB: Announce the coordinator to the current bully. If we don't, then
		// the current bully will hang waiting for a COORDINATOR message.
		b.broadcast <- &proto.MessageWithPid{
			Pid:     in.Pid,
			Message: proto.Message_COORDINATOR,
		}
	}

	reply := &proto.MessageWithPid{
		Pid:     b.self.Pid,
		Message: msg,
	}

	log.Printf("[%d] SENDING %s to [%d]", b.self.Pid.Value, reply.Message.String(), in.Pid.Value)

	return reply, nil
}

// Add adds a Process to the internal bully storage.
func (p *Processes) Add(proc *Process) error {
	p.Lock()
	defer p.Unlock()
	p.internal[proc.Pid.Value] = proc
	return nil
}

// Remove removes a Process identified by a Pid from the internal bully storage.
func (p *Processes) Remove(pid Pid) error {
	p.Lock()
	defer p.Unlock()
	delete(p.internal, pid)
	return nil
}

// Get returns a point to a Process identified by a Pid from the internal bully
// storage.
func (p *Processes) Get(pid Pid) *Process {
	p.RLock()
	defer p.RUnlock()
	return p.internal[pid]
}
