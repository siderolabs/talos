// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package poweroff

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// Main is the entrypoint into /sbin/poweroff.
func Main() {
	ctx := context.Background()

	md := metadata.Pairs()
	authz.SetMetadata(md, role.MakeSet(role.Admin))
	adminCtx := metadata.NewOutgoingContext(ctx, md)

	client, err := client.New(adminCtx, client.WithUnixSocket(constants.APISocketPath), client.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())))
	if err != nil {
		log.Fatalf(fmt.Errorf("error while creating machinery client: %w", err).Error())
	}

	err = client.Shutdown(adminCtx)
	if err != nil {
		log.Fatalf(fmt.Errorf("error while sending shutdown command: %w", err).Error())
	}
}
