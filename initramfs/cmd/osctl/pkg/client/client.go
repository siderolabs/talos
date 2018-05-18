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

	"github.com/autonomy/dianemo/initramfs/cmd/osd/proto"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Credentials represents the set of values required to initialize a vaild
// Client.
type Credentials struct {
	ca  string
	crt string
	key string
}

// Client implements the proto.OSDClient interface. It serves as the
// concrete type with the required methods.
type Client struct {
	conn   *grpc.ClientConn
	client proto.OSDClient
}

// NewDefaultClientCredentials initializes ClientCredentials using default paths
// to the required CA, certificate, and key.
func NewDefaultClientCredentials() (creds *Credentials, err error) {
	u, err := user.Current()
	if err != nil {
		return
	}

	creds = &Credentials{
		ca:  path.Join(u.HomeDir, ".dianemo/ca.pem"),
		crt: path.Join(u.HomeDir, ".dianemo/crt.pem"),
		key: path.Join(u.HomeDir, ".dianemo/key.pem"),
	}

	return creds, nil
}

// NewClient initializes a Client.
func NewClient(address string, port int, clientcreds *Credentials) (c *Client, err error) {
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
		// Set the root certificate authorities to use the self-signed
		// certificate.
		RootCAs: certPool,
	})

	grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(creds))
	c.conn, err = grpc.Dial(fmt.Sprintf("%s:%d", address, port), grpcOpts...)
	if err != nil {
		return
	}

	c.client = proto.NewOSDClient(c.conn)

	return c, nil
}

// Kubeconfig implements the proto.OSDClient interface.
func (c *Client) Kubeconfig() (err error) {
	ctx := context.Background()
	r, err := c.client.Kubeconfig(ctx, &empty.Empty{})
	if err != nil {
		return
	}
	fmt.Print(string(r.Bytes))

	return nil
}

// Dmesg implements the proto.OSDClient interface.
// nolint: dupl
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

// Logs implements the proto.OSDClient interface.
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

// Version implements the proto.OSDClient interface.
// nolint: dupl
func (c *Client) Version() (err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	data, err := c.client.Version(ctx, &empty.Empty{})
	if err != nil {
		return
	}
	fmt.Print(string(data.Bytes))

	return nil
}
