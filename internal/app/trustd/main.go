package main

import (
	"flag"
	"log"

	"github.com/autonomy/talos/internal/app/trustd/internal/reg"
	"github.com/autonomy/talos/internal/pkg/constants"
	"github.com/autonomy/talos/internal/pkg/grpc/factory"
	"github.com/autonomy/talos/internal/pkg/grpc/middleware/auth/basic"
	"github.com/autonomy/talos/internal/pkg/grpc/tls"
	"github.com/autonomy/talos/internal/pkg/userdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	dataPath *string
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
	dataPath = flag.String("userdata", "", "the path to the user data")
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
		factory.Port(constants.TrustdPort),
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
