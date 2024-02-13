// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"context"
	"errors"
	"log"
	"net"

	"github.com/siderolabs/kms-client/api/kms"
	"github.com/siderolabs/kms-client/pkg/server"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	grpclog "github.com/siderolabs/talos/pkg/grpc/middleware/log"
)

var kmsLaunchCmdFlags struct {
	addr string
	key  []byte
}

// kmsLaunchCmd represents the kms-launch command.
var kmsLaunchCmd = &cobra.Command{
	Use:    "kms-launch",
	Short:  "Internal command used by QEMU provisioner",
	Long:   ``,
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if kmsLaunchCmdFlags.key == nil {
			return errors.New("no key provided to the KMS server")
		}

		srv := server.NewServer(func(_ context.Context, nodeUUID string) ([]byte, error) {
			return kmsLaunchCmdFlags.key, nil
		})

		lis, err := net.Listen("tcp", kmsLaunchCmdFlags.addr)
		if err != nil {
			return err
		}

		log.Printf("starting KMS server on %s", kmsLaunchCmdFlags.addr)

		logMiddleware := grpclog.NewMiddleware(log.New(log.Writer(), "", log.Flags()))

		s := grpc.NewServer(
			grpc.UnaryInterceptor(logMiddleware.UnaryInterceptor()),
			grpc.StreamInterceptor(logMiddleware.StreamInterceptor()),
		)
		kms.RegisterKMSServiceServer(s, srv)

		eg, ctx := errgroup.WithContext(cmd.Context())

		eg.Go(func() error {
			err := s.Serve(lis)
			if errors.Is(err, context.Canceled) {
				return nil
			}

			return err
		})

		eg.Go(func() error {
			<-ctx.Done()

			s.Stop()

			return nil
		})

		return s.Serve(lis)
	},
}

func init() {
	kmsLaunchCmd.Flags().StringVar(&kmsLaunchCmdFlags.addr, "kms-addr", "localhost", "KMS listen address (IP or host)")
	kmsLaunchCmd.Flags().BytesBase64Var(&kmsLaunchCmdFlags.key, "kms-key", nil, "KMS key to use")
	addCommand(kmsLaunchCmd)
}
