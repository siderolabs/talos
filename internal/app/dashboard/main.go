// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dashboard implements dashboard functionality.
package dashboard

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/siderolabs/go-procfs/procfs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	metalurl "github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal/url"
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

	ctx, cancel := sigtermAwareContext(context.Background())
	defer cancel()

	ctx = metadata.NewOutgoingContext(ctx, md)

	c, err := client.New(ctx,
		client.WithUnixSocket(constants.MachineSocketPath),
		client.WithGRPCDialOptions(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		),
	)
	if err != nil {
		return fmt.Errorf("error connecting to the machine service: %w", err)
	}

	screens := []dashboard.Screen{dashboard.ScreenSummary, dashboard.ScreenMonitor}

	// activate the network config screen only on metal platform
	currentPlatform, _ := platform.CurrentPlatform() //nolint:errcheck
	if currentPlatform != nil && currentPlatform.Name() == constants.PlatformMetal {
		screens = append(screens, dashboard.ScreenNetworkConfig)

		if showConfigURLTab() {
			screens = append(screens, dashboard.ScreenConfigURL)
		}
	}

	return dashboard.Run(ctx, c, dashboard.WithAllowExitKeys(false), dashboard.WithScreens(screens...))
}

func showConfigURLTab() bool {
	option := procfs.ProcCmdline().Get(constants.KernelParamConfig).First()
	if option == nil {
		return false
	}

	parsedURL, err := url.Parse(*option)
	if err != nil {
		return false
	}

	codeVar := metalurl.AllVariables()[constants.CodeKey]
	if codeVar == nil {
		return false
	}

	return codeVar.Matches(parsedURL.Query())
}

func sigtermAwareContext(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM)

	go func() {
		select {
		case <-signalCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}
