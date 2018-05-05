package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strconv"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/chunker"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/constants"
	servicelog "github.com/autonomy/dianemo/initramfs/src/init/pkg/service/log"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/userdata"
	proto "github.com/autonomy/dianemo/initramfs/src/init/proto"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/kubernetes-incubator/cri-o/client"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Server struct {
	server *grpc.Server
	port   int
	ca     string
	crt    string
	key    string
}

func NewServer(data *userdata.Security) (s *Server, err error) {
	s = &Server{
		port: 50000,
		ca:   data.CA.Crt,
		crt:  data.Identity.Crt,
		key:  data.Identity.Key,
	}
	return s, err
}

func (s *Server) Listen() (err error) {
	var (
		listener net.Listener
		grpcOpts = []grpc.ServerOption{}
	)
	caBytes, err := base64.StdEncoding.DecodeString(s.ca)
	if err != nil {
		return err
	}
	keyBytes, err := base64.StdEncoding.DecodeString(s.key)
	if err != nil {
		return err
	}
	crtBytes, err := base64.StdEncoding.DecodeString(s.crt)
	if err != nil {
		return err
	}
	listener, err = net.Listen("tcp", ":"+strconv.Itoa(s.port))
	if err != nil {
		return
	}

	crt, err := tls.X509KeyPair(crtBytes, keyBytes)
	if err != nil {
		return fmt.Errorf("could not load server key pair: %s", err)
	}
	certPool := x509.NewCertPool()
	if err != nil {
		return fmt.Errorf("could not read ca certificate: %s", err)
	}
	if ok := certPool.AppendCertsFromPEM(caBytes); !ok {
		return fmt.Errorf("failed to append client certs")
	}
	creds := credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{crt},
		// Validate certificates against the provided CA.
		ClientCAs: certPool,
		// Perfect Forward Secrecy.
		CurvePreferences: []tls.CurveID{tls.X25519},
		CipherSuites:     []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384},
		// Force the above cipher suites.
		PreferServerCipherSuites: true,
		// TLS 1.2
		MinVersion: tls.VersionTLS12,
	})
	grpcOpts = append(grpcOpts, grpc.Creds(creds))
	s.server = grpc.NewServer(grpcOpts...)
	proto.RegisterDianemoServer(s.server, s)

	log.Printf("Started gRPC on :%d", s.port)
	err = s.server.Serve(listener)
	if err != nil {
		return
	}

	return
}

func (s *Server) Kubeconfig(ctx context.Context, in *empty.Empty) (r *proto.Data, err error) {
	fileBytes, err := ioutil.ReadFile("/etc/kubernetes/admin.conf")
	if err != nil {
		return
	}
	r = &proto.Data{
		Bytes: fileBytes,
	}

	return
}

func (s *Server) Processes(ctx context.Context, in *proto.ProcessesRequest) (r *proto.ProcessesReply, err error) {
	return
}

func (s *Server) Dmesg(ctx context.Context, in *empty.Empty) (data *proto.Data, err error) {
	// Return the size of the kernel ring buffer
	size, err := unix.Klogctl(constants.SYSLOG_ACTION_SIZE_BUFFER, nil)
	if err != nil {
		return
	}
	// Read all messages from the log (non-destructively)
	buf := make([]byte, size)
	n, err := unix.Klogctl(constants.SYSLOG_ACTION_READ_ALL, buf)
	if err != nil {
		return
	}

	data = &proto.Data{Bytes: buf[:n]}

	return
}

func (s *Server) Logs(r *proto.LogsRequest, l proto.Dianemo_LogsServer) (err error) {
	var stream chunker.ChunkReader
	if r.Container {
		// TODO: Use the specified container runtime.
		cli, e := client.New("/var/run/crio/crio.sock")
		if e != nil {
			err = e
			return
		}
		info, e := cli.ContainerInfo(r.Process)
		if e != nil {
			err = e
			return
		}
		stream = chunker.NewDefaultChunker(info.LogPath)
	} else {
		stream = servicelog.Get(r.Process)
		if stream == nil {
			err = fmt.Errorf("no such process: %s", r.Process)
			return
		}
	}

	for data := range stream.Read(l.Context()) {
		l.Send(&proto.Data{Bytes: data})
	}

	return
}
