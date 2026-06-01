// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/siderolabs/gen/ensure"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/siderolabs/talos/pkg/machinery/client"
)

func ExampleNew() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	connPprof := pprof.Lookup("machinery/client/grpc.grpcConn")
	if connPprof == nil {
		panic(errors.New("profile machinery/client/grpc.grpcConn not found"))
	}

	fmt.Println("before:", connPprof.Count())

	c := ensure.Value(
		client.New(
			ctx,
			client.WithUnixSocket("/path/to/socket"),
			client.WithGRPCDialOptions(
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			),
		),
	)

	fmt.Println("after client.New:", connPprof.Count())

	if err := c.Close(); err != nil {
		panic(err)
	}

	fmt.Println("after client.Close:", connPprof.Count())

	c2 := ensure.Value(
		client.New(
			ctx,
			client.WithUnixSocket("/path/to/socket"),
			client.WithGRPCDialOptions(
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			),
		),
	)

	fmt.Println("after client.New 2:", connPprof.Count())

	_ = c2

	runtime.GC()

	fmt.Println("after gc:", connPprof.Count())

	// Output:
	// before: 0
	// after client.New: 1
	// after client.Close: 0
	// after client.New 2: 1
	// after gc: 1
}
