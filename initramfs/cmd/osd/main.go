package main

import (
	"flag"
	"log"

	"github.com/autonomy/dianemo/initramfs/cmd/osd/pkg/gen"
	"github.com/autonomy/dianemo/initramfs/cmd/osd/pkg/reg"
	"github.com/autonomy/dianemo/initramfs/pkg/grpc/factory"
	"github.com/autonomy/dianemo/initramfs/pkg/grpc/middleware/auth/basic"
	"github.com/autonomy/dianemo/initramfs/pkg/grpc/tls"
	"github.com/autonomy/dianemo/initramfs/pkg/userdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	dataPath *string
	generate *bool
	port     *int
	rotPort  *int
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
	dataPath = flag.String("userdata", "", "the path to the user data")
	port = flag.Int("port", 50000, "the port to listen on")
	rotPort = flag.Int("rot-port", 50001, "the port to listen on")
	generate = flag.Bool("generate", false, "generate the TLS certificate using one of the Root of Trusts")
	flag.Parse()
}

func main() {
	var err error

	data, err := userdata.Open(*dataPath)
	if err != nil {
		log.Fatalf("open user data: %v", err)
	}

	if *generate {
		if len(data.OS.Security.RootsOfTrust.Endpoints) == 0 {
			log.Fatalf("at least one root of trust endpoint is required")
		}

		creds := basic.NewCredentials(
			data.OS.Security.CA.Crt,
			data.OS.Security.RootsOfTrust.Username,
			data.OS.Security.RootsOfTrust.Password,
		)

		// TODO: In the case of failure, attempt to generate the identity from
		// another RoT.
		var conn *grpc.ClientConn
		conn, err = basic.NewConnection(data.OS.Security.RootsOfTrust.Endpoints[0], *rotPort, creds)
		if err != nil {
			return
		}
		generator := gen.NewGenerator(conn)
		if err = generator.Identity(data.OS.Security); err != nil {
			log.Fatalf("generate identity: %v", err)
		}
	}

	config, err := tls.NewConfig(tls.Mutual, data.OS.Security)
	if err != nil {
		log.Fatalf("credentials: %v", err)
	}

	err = factory.Listen(
		&reg.Registrator{},
		factory.Port(*port),
		factory.ServerOptions(
			grpc.Creds(
				credentials.NewTLS(config),
			),
		),
	)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
}
