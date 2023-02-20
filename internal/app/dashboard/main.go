// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/internal/pkg/dashboard"
	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"
	"github.com/siderolabs/talos/pkg/startup"
)

// Main is the entrypoint into trustd.
func Main() {
	if err := dashboardMain(); err != nil {
		log.Fatal(err)
	}
}

func dashboardMain() error {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	log.Printf("starting dashboard service")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	startup.LimitMaxProcs(constants.DashboardMaxProcs)

	var err error

	if err = startup.RandSeed(); err != nil {
		return fmt.Errorf("startup: %s", err)
	}

	md := metadata.Pairs()
	authz.SetMetadata(md, role.MakeSet(role.Admin))
	adminCtx := metadata.NewOutgoingContext(ctx, md)

	c, err := client.New(adminCtx,
		client.WithUnixSocket(constants.MachineSocketPath),
		client.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		return fmt.Errorf("error connecting to the machine service: %w", err)
	}

	return dashboard.Main(adminCtx, c, 5*time.Second)
}
