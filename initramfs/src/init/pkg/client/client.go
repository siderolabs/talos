package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"os/user"
	"path"

	"github.com/autonomy/dianemo/initramfs/src/init/proto"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type ClientCredentials struct {
	ca  string
	crt string
	key string
}

type Client struct {
	conn        *grpc.ClientConn
	client      proto.DianemoClient
	credentials *ClientCredentials
}

func NewDefaultClientCredentials() (creds *ClientCredentials, err error) {
	u, err := user.Current()
	if err != nil {
		return
	}

	creds = &ClientCredentials{
		ca:  path.Join(u.HomeDir, ".dianemo/ca.pem"),
		crt: path.Join(u.HomeDir, ".dianemo/crt.pem"),
		key: path.Join(u.HomeDir, ".dianemo/key.pem"),
	}

	return creds, nil
}

func NewClient(address string, port int, clientcreds *ClientCredentials) (c *Client, err error) {
	grpcOpts := []grpc.DialOption{}

	caBytes, err := ioutil.ReadFile(clientcreds.ca)
	if err != nil {
		return
	}
	c = &Client{}
	crt, err := tls.LoadX509KeyPair(clientcreds.crt, clientcreds.key)
	if err != nil {
		return nil, fmt.Errorf("could not load client key pair: %s", err)
	}
	certPool := x509.NewCertPool()
	if err != nil {
		return nil, fmt.Errorf("could not read ca certificate: %s", err)
	}
	if ok := certPool.AppendCertsFromPEM(caBytes); !ok {
		return nil, fmt.Errorf("failed to append client certs")
	}
	// TODO: Do not parse the address. Pass the IP and port in as separate
	// parameters.
	creds := credentials.NewTLS(&tls.Config{
		ServerName:   address,
		Certificates: []tls.Certificate{crt},
		RootCAs:      certPool,
	})

	grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(creds))
	c.conn, err = grpc.Dial(fmt.Sprintf("%s:%d", address, port), grpcOpts...)
	if err != nil {
		return
	}

	c.client = proto.NewDianemoClient(c.conn)

	return c, nil
}

func (c *Client) Kubeconfig() (err error) {
	ctx := context.Background()
	r, err := c.client.Kubeconfig(ctx, &empty.Empty{})
	if err != nil {
		return
	}
	fmt.Print(string(r.Bytes))

	return nil
}

func (c *Client) Dmesg() (err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	data, err := c.client.Dmesg(ctx, &empty.Empty{})
	if err != nil {
		return
	}
	fmt.Print(string(data.Bytes))

	return nil
}

func (c *Client) Logs(r *proto.LogsRequest) (err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := c.client.Logs(ctx, r)
	if err != nil {
		return
	}
	for {
		data, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return err
			}

			return err
		}
		fmt.Print(string(data.Bytes))
	}
}
