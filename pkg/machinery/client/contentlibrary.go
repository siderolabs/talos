// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"
	"errors"
	"fmt"
	"io"

	"google.golang.org/grpc"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// contentLibraryUploadChunkSize is the size of the chunks an upload is split into.
//
// Virtual machine images run to gigabytes, so the chunks are a lot bigger than the ones the older
// streaming APIs use, while staying well under the default gRPC message size limit.
const contentLibraryUploadChunkSize = 1 << 20

// ContentLibraryUpload uploads a file to a content library on the node.
func (c *Client) ContentLibraryUpload(
	ctx context.Context,
	libraryID, name string,
	overwrite bool,
	contents io.Reader,
	callOptions ...grpc.CallOption,
) (*machineapi.ContentLibraryServiceUploadResponse, error) {
	cli, err := c.ContentLibraryClient.Upload(ctx, callOptions...)
	if err != nil {
		return nil, err
	}

	if err = cli.Send(&machineapi.ContentLibraryServiceUploadRequest{
		Request: &machineapi.ContentLibraryServiceUploadRequest_Info{
			Info: &machineapi.ContentLibraryServiceUploadInfo{
				LibraryId: libraryID,
				Name:      name,
				Overwrite: overwrite,
			},
		},
	}); err != nil && !errors.Is(err, io.EOF) {
		// io.EOF means the server has already given up on the stream; CloseAndRecv below has the
		// error that says why.
		return nil, err
	}

	buf := make([]byte, contentLibraryUploadChunkSize)

	for {
		var n int

		n, err = contents.Read(buf)
		if n > 0 {
			if err := cli.Send(&machineapi.ContentLibraryServiceUploadRequest{
				Request: &machineapi.ContentLibraryServiceUploadRequest_Chunk{
					Chunk: &common.Data{
						Bytes: buf[:n],
					},
				},
			}); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return nil, err
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("error reading the file: %w", err)
		}
	}

	return cli.CloseAndRecv()
}
