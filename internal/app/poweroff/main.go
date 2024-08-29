// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package poweroff

import (
	"context"
	"log"
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
	authz.SetMetadata(md, role.MakeSet(role.Admin))
	adminCtx := metadata.NewOutgoingContext(ctx, md)

	client, err := client.New(adminCtx, client.WithUnixSocket(constants.MachineSocketPath), client.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())))
	if err != nil {
		log.Fatalf("error while creating machinery client: %s", err)
	}

	switch ActionFromArgs(args) {
	case Shutdown:
		err = client.Shutdown(adminCtx)
		if err != nil {
			log.Fatalf("error while sending shutdown command: %s", err)
		}
	case Reboot:
		err = client.Reboot(adminCtx)
		if err != nil {
			log.Fatalf("error while sending reboot command: %s", err)
		}
	}
}

// ActionFromArgs returns the action to be performed based on the arguments.
func ActionFromArgs(args []string) Action {
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

	return Shutdown
}
