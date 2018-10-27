package main

import (
	"flag"
	"log"

	"github.com/autonomy/dianemo/src/initramfs/cmd/osd/pkg/reg"
	"github.com/autonomy/dianemo/src/initramfs/pkg/grpc/factory"
	"github.com/autonomy/dianemo/src/initramfs/pkg/grpc/gen"
	"github.com/autonomy/dianemo/src/initramfs/pkg/grpc/tls"
	"github.com/autonomy/dianemo/src/initramfs/pkg/userdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	dataPath   *string
	generate   *bool
	port       *int
	trustdPort *int
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
	dataPath = flag.String("userdata", "", "the path to the user data")
	port = flag.Int("port", 50000, "the port to listen on")
	trustdPort = flag.Int("trustd-port", 50001, "the trustd port")
	generate = flag.Bool("generate", false, "generate the TLS certificate using one of the Root of Trusts")
	flag.Parse()
}

func main() {
	data, err := userdata.Open(*dataPath)
	if err != nil {
		log.Fatalf("open user data: %v", err)
	}

	if *generate {
		var generator *gen.Generator
		generator, err = gen.NewGenerator(data, *trustdPort)
		if err != nil {
			log.Fatal(err)
		}
		if err = generator.Identity(data.Security); err != nil {
			log.Fatalf("generate identity: %v", err)
		}
	}

	config, err := tls.NewConfig(tls.Mutual, data.Security.OS)
	if err != nil {
		log.Fatalf("credentials: %v", err)
	}

	log.Println("Starting osd")
	err = factory.Listen(
		&reg.Registrator{Data: data},
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
