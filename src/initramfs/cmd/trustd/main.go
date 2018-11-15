package main

import (
	"flag"
	"log"

	"github.com/autonomy/talos/src/initramfs/cmd/trustd/pkg/reg"
	"github.com/autonomy/talos/src/initramfs/pkg/grpc/factory"
	"github.com/autonomy/talos/src/initramfs/pkg/grpc/middleware/auth/basic"
	"github.com/autonomy/talos/src/initramfs/pkg/grpc/tls"
	"github.com/autonomy/talos/src/initramfs/pkg/userdata"
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

	config, err := tls.NewConfig(tls.ServerOnly, data.Security.OS)
	if err != nil {
		log.Fatalf("credentials: %v", err)
	}

	creds := basic.NewCredentials(
		data.Security.OS.CA.Crt,
		data.Services.Trustd.Username,
		data.Services.Trustd.Password,
	)

	err = factory.Listen(
		&reg.Registrator{Data: data.Security.OS},
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
