// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package poweroff

import (
	"context"
	"log"
	"path/filepath"
	"slices"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// Action is the action to be performed by the poweroff command.
type Action string

const (
	// Shutdown is the action to shutdown the machine.
	Shutdown Action = "shutdown"
	// Reboot is the action to reboot the machine.
	Reboot Action = "reboot"
)

// Main is the entrypoint into /sbin/poweroff.
func Main(args []string) {
	ctx := context.Background()

	md := metadata.Pairs()
	authz.SetMetadata(md, role.MakeSet(role.Operator))
	adminCtx := metadata.NewOutgoingContext(ctx, md)

	client, err := client.New(
		adminCtx,
		client.WithUnixSocket(constants.MachineSocketPath),
		client.WithGRPCDialOptions(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		),
	)
	if err != nil {
		log.Fatalf("error while creating machinery client: %s", err)
	}

	action := ActionFromArgs(args)

	log.Printf("helper: action %q", action)

	switch action {
	case Shutdown:
		err = client.Shutdown(adminCtx)
		if err != nil {
			log.Fatalf("error while sending shutdown command: %s", err)
		}

		log.Printf("shutdown command sent")
	case Reboot:
		err = client.Reboot(adminCtx)
		if err != nil {
			log.Fatalf("error while sending reboot command: %s", err)
		}

		log.Printf("reboot command sent")
	}
}

// ActionFromArgs returns the action to be performed based on the arguments.
//
// The default action is derived from the basename the binary was invoked as
// (e.g. the kernel usermode helper calls `/sbin/reboot` for orderly_reboot and
// `/sbin/poweroff` for orderly_poweroff), and can be overridden by explicit flags.
//
//nolint:gocyclo
func ActionFromArgs(args []string) Action {
	action := Shutdown

	if len(args) > 0 && filepath.Base(args[0]) == "reboot" {
		action = Reboot
	}

	if len(args) > 1 {
		if slices.ContainsFunc(args[1:], func(s string) bool {
			return s == "--halt" || s == "-H" || s == "--poweroff" || s == "-P" || s == "-p"
		}) {
			return Shutdown
		}

		if slices.ContainsFunc(args[1:], func(s string) bool {
			return s == "--reboot" || s == "-r"
		}) {
			return Reboot
		}
	}

	return action
}
