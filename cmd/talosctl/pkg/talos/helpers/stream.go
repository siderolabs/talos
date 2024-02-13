// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"errors"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// Stream implements the contract for the grpc stream of a specific type.
type Stream[T proto.Message] interface {
	Recv() (T, error)
	grpc.ClientStream
}

// Message defines the contract for the grpc message.
type Message interface {
	GetMetadata() *common.Metadata
	proto.Message
}

// ReadGRPCStream consumes all messages from the gRPC stream, handles errors, calls the passed handler for each message.
func ReadGRPCStream[S Stream[T], T Message](stream S, handler func(T, string, bool) error) error {
	var streamErrs error

	defaultNode := client.RemotePeer(stream.Context())

	multipleNodes := false

	for {
		info, err := stream.Recv()
		if err != nil {
			if err == io.EOF || client.StatusCode(err) == codes.Canceled {
				return streamErrs
			}

			return fmt.Errorf("error streaming results: %s", err)
		}

		node := defaultNode

		if info.GetMetadata() != nil {
			if info.GetMetadata().Hostname != "" {
				multipleNodes = true
				node = info.GetMetadata().Hostname
			}

			if info.GetMetadata().Error != "" {
				streamErrs = AppendErrors(streamErrs, errors.New(info.GetMetadata().Error))

				continue
			}
		}

		if err = handler(info, node, multipleNodes); err != nil {
			var errNonFatal *ErrNonFatalError
			if errors.As(err, &errNonFatal) {
				streamErrs = AppendErrors(streamErrs, err)

				continue
			}

			return err
		}
	}
}

// ErrNonFatalError represents the error that can be returned from the handler in the gRPC stream reader
// which doesn't mean that we should stop iterating over the messages in the stream, but log this error
// and continue the process.
type ErrNonFatalError struct {
	err error
}

// Error implements error interface.
func (e *ErrNonFatalError) Error() string {
	return e.err.Error()
}

// NonFatalError wraps another error into a ErrNonFatal.
func NonFatalError(err error) error {
	return &ErrNonFatalError{
		err: err,
	}
}
