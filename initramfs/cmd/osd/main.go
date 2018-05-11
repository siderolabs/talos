package main

import (
	"flag"
	"io/ioutil"
	"log"

	"github.com/autonomy/dianemo/initramfs/cmd/init/pkg/userdata"
	"github.com/autonomy/dianemo/initramfs/cmd/osd/pkg/server"
	yaml "gopkg.in/yaml.v2"
)

var (
	dataPath *string
	port     *int
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
	dataPath = flag.String("userdata", "", "the path to the user data")
	port = flag.Int("port", 50000, "the port to listen on")
	flag.Parse()
}

func main() {
	fileBytes, err := ioutil.ReadFile(*dataPath)
	if err != nil {
		log.Fatalf("read user data: %v", err)
	}
	data := &userdata.UserData{}
	if err := yaml.Unmarshal(fileBytes, data); err != nil {
		log.Fatalf("unmarshal user data: %v", err)
	}
	if err := server.NewServer(*port, data.OS.Security).Listen(); err != nil {
		log.Fatalf("start gRPC server: %v", err)
	}
}
