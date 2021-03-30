// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package apid

import (
	"flag"
	"log"
	"regexp"
	"strings"

	"github.com/talos-systems/grpc-proxy/proxy"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	apidbackend "github.com/talos-systems/talos/internal/app/apid/pkg/backend"
	"github.com/talos-systems/talos/internal/app/apid/pkg/director"
	"github.com/talos-systems/talos/internal/app/apid/pkg/provider"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/grpc/proxy/backend"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/startup"
)

var (
	endpoints       *string
	useK8sEndpoints *bool
)

// Main is the entrypoint of apid.
func Main() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	endpoints = flag.String("endpoints", "", "the static list of IPs of the control plane nodes")
	useK8sEndpoints = flag.Bool("use-kubernetes-endpoints", false, "use Kubernetes master node endpoints as control plane endpoints")

	flag.Parse()

	if err := startup.RandSeed(); err != nil {
		log.Fatalf("failed to seed RNG: %v", err)
	}

	config, err := configloader.NewFromStdin()
	if err != nil {
		log.Fatalf("open config: %v", err)
	}

	var endpointsProvider provider.Endpoints

	if *useK8sEndpoints {
		endpointsProvider = &provider.KubernetesEndpoints{}
	} else {
		endpointsProvider = &provider.StaticEndpoints{
			Endpoints: strings.Split(*endpoints, ","),
		}
	}

	tlsConfig, err := provider.NewTLSConfig(config, endpointsProvider)
	if err != nil {
		log.Fatalf("failed to create remote certificate provider: %+v", err)
	}

	serverTLSConfig, err := tlsConfig.ServerConfig()
	if err != nil {
		log.Fatalf("failed to create OS-level TLS configuration: %v", err)
	}

	clientTLSConfig, err := tlsConfig.ClientConfig()
	if err != nil {
		log.Fatalf("failed to create client TLS config: %v", err)
	}

	backendFactory := apidbackend.NewAPIDFactory(clientTLSConfig)
	localBackend := backend.NewLocal("machined", constants.MachineSocketPath)

	router := director.NewRouter(backendFactory.Get, localBackend)

	// all existing streaming methods
	for _, methodName := range []string{
		"/machine.MachineService/Copy",
		"/machine.MachineService/DiskUsage",
		"/machine.MachineService/Dmesg",
		"/machine.MachineService/EtcdSnapshot",
		"/machine.MachineService/Events",
		"/machine.MachineService/Kubeconfig",
		"/machine.MachineService/List",
		"/machine.MachineService/Logs",
		"/machine.MachineService/Read",
		"/resource.ResourceService/List",
		"/resource.ResourceService/Watch",
		"/os.OSService/Dmesg",
		"/cluster.ClusterService/HealthCheck",
	} {
		router.RegisterStreamedRegex("^" + regexp.QuoteMeta(methodName) + "$")
	}

	// register future pattern: method should have suffix "Stream"
	router.RegisterStreamedRegex("Stream$")

	var errGroup errgroup.Group

	errGroup.Go(func() error {
		return factory.ListenAndServe(
			router,
			factory.Port(constants.ApidPort),
			factory.WithDefaultLog(),
			factory.ServerOptions(
				grpc.Creds(
					credentials.NewTLS(serverTLSConfig),
				),
				grpc.CustomCodec(proxy.Codec()), //nolint:staticcheck
				grpc.UnknownServiceHandler(
					proxy.TransparentHandler(
						router.Director,
						proxy.WithStreamedDetector(router.StreamedDetector),
					)),
			),
		)
	})

	errGroup.Go(func() error {
		return factory.ListenAndServe(
			router,
			factory.Network("unix"),
			factory.SocketPath(constants.APISocketPath),
			factory.WithDefaultLog(),
			factory.ServerOptions(
				grpc.CustomCodec(proxy.Codec()), //nolint:staticcheck
				grpc.UnknownServiceHandler(
					proxy.TransparentHandler(
						router.Director,
						proxy.WithStreamedDetector(router.StreamedDetector),
					)),
			),
		)
	})

	if err := errGroup.Wait(); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
