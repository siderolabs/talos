package main

import (
	"flag"
	"log"

	"github.com/autonomy/dianemo/initramfs/cmd/rotd/pkg/reg"
	"github.com/autonomy/dianemo/initramfs/pkg/grpc/factory"
	"github.com/autonomy/dianemo/initramfs/pkg/grpc/middleware/auth/basic"
	"github.com/autonomy/dianemo/initramfs/pkg/grpc/tls"
	"github.com/autonomy/dianemo/initramfs/pkg/userdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	dataPath *string
	port     *int
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
	dataPath = flag.String("userdata", "", "the path to the user data")
	port = flag.Int("port", 50001, "the port to listen on")
	flag.Parse()
}

func main() {
	var err error

	data, err := userdata.Open(*dataPath)
	if err != nil {
		log.Fatalf("credentials: %v", err)
	}

	config, err := tls.NewConfig(tls.ServerOnly, data.OS.Security)
	if err != nil {
		log.Fatalf("credentials: %v", err)
	}

	creds := basic.NewCredentials(
		data.OS.Security.CA.Crt,
		data.OS.Security.RootsOfTrust.Username,
		data.OS.Security.RootsOfTrust.Password,
	)

	err = factory.Listen(
		&reg.Registrator{Data: data.OS.Security},
		factory.Port(*port),
		factory.ServerOptions(
			grpc.Creds(
				credentials.NewTLS(config),
			),
			grpc.UnaryInterceptor(creds.UnaryInterceptor),
		),
	)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
}
