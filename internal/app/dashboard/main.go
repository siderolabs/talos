// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dashboard implements dashboard functionality.
package dashboard

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/siderolabs/talos/internal/pkg/dashboard"
	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"
	"github.com/siderolabs/talos/pkg/startup"
)

// Main is the entrypoint into dashboard.
func Main() {
	if err := dashboardMain(); err != nil {
		log.Fatal(err)
	}
}

func dashboardMain() error {
	startup.LimitMaxProcs(constants.DashboardMaxProcs)

	md := metadata.Pairs()
	authz.SetMetadata(md, role.MakeSet(role.Admin))
	adminCtx := metadata.NewOutgoingContext(context.Background(), md)

	c, err := client.New(adminCtx,
		client.WithUnixSocket(constants.MachineSocketPath),
		client.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		return fmt.Errorf("error connecting to the machine service: %w", err)
	}

	screens := []dashboard.Screen{dashboard.ScreenSummary, dashboard.ScreenMonitor}

	// activate the network config screen only on metal platform
	currentPlatform, _ := platform.CurrentPlatform() //nolint:errcheck
	if currentPlatform != nil && currentPlatform.Name() == constants.PlatformMetal {
		screens = append(screens, dashboard.ScreenNetworkConfig)
	}

	return dashboard.Run(adminCtx, c, dashboard.WithAllowExitKeys(false), dashboard.WithScreens(screens...))
}
