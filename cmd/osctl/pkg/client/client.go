/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client/config"
	"github.com/talos-systems/talos/internal/app/osd/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Credentials represents the set of values required to initialize a vaild
// Client.
type Credentials struct {
	Target string
	ca     []byte
	crt    []byte
	key    []byte
}

// Client implements the proto.OSDClient interface. It serves as the
// concrete type with the required methods.
type Client struct {
	conn   *grpc.ClientConn
	client proto.OSDClient
}

// NewDefaultClientCredentials initializes ClientCredentials using default paths
// to the required CA, certificate, and key.
func NewDefaultClientCredentials(p string) (creds *Credentials, err error) {
	c, err := config.Open(p)
	if err != nil {
		return
	}

	caBytes, err := base64.StdEncoding.DecodeString(c.Contexts[c.Context].CA)
	if err != nil {
		return
	}
	crtBytes, err := base64.StdEncoding.DecodeString(c.Contexts[c.Context].Crt)
	if err != nil {
		return
	}
	keyBytes, err := base64.StdEncoding.DecodeString(c.Contexts[c.Context].Key)
	if err != nil {
		return
	}
	creds = &Credentials{
		Target: c.Contexts[c.Context].Target,
		ca:     caBytes,
		crt:    crtBytes,
		key:    keyBytes,
	}

	return creds, nil
}

// NewClient initializes a Client.
func NewClient(port int, clientcreds *Credentials) (c *Client, err error) {
	grpcOpts := []grpc.DialOption{}

	c = &Client{}
	crt, err := tls.X509KeyPair(clientcreds.crt, clientcreds.key)
	if err != nil {
		return nil, fmt.Errorf("could not load client key pair: %s", err)
	}
	certPool := x509.NewCertPool()
	if err != nil {
		return nil, fmt.Errorf("could not read ca certificate: %s", err)
	}
	if ok := certPool.AppendCertsFromPEM(clientcreds.ca); !ok {
		return nil, fmt.Errorf("failed to append client certs")
	}
	// TODO(andrewrynhard): Do not parse the address. Pass the IP and port in as separate
	// parameters.
	creds := credentials.NewTLS(&tls.Config{
		ServerName:   clientcreds.Target,
		Certificates: []tls.Certificate{crt},
		// Set the root certificate authorities to use the self-signed
		// certificate.
		RootCAs: certPool,
	})

	grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(creds))
	c.conn, err = grpc.Dial(fmt.Sprintf("%s:%d", clientcreds.Target, port), grpcOpts...)
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

// Stats implements the proto.OSDClient interface.
func (c *Client) Stats(namespace string) (err error) {
	ctx := context.Background()
	reply, err := c.client.Stats(ctx, &proto.StatsRequest{Namespace: namespace})
	if err != nil {
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAMESPACE\tID\tMEMORY(MB)\tCPU")
	for _, s := range reply.Stats {
		fmt.Fprintf(w, "%s\t%s\t%.2f\t%d\n", s.Namespace, s.Id, float64(s.MemoryUsage)*1e-6, s.CpuUsage)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	return nil
}

// Processes implements the proto.OSDClient interface.
func (c *Client) Processes(namespace string) (err error) {
	ctx := context.Background()
	reply, err := c.client.Processes(ctx, &proto.ProcessesRequest{Namespace: namespace})
	if err != nil {
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAMESPACE\tID\tIMAGE\tPID\tSTATUS")
	for _, p := range reply.Processes {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", p.Namespace, p.Id, p.Image, p.Pid, p.Status)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	return nil
}

// Restart implements the proto.OSDClient interface.
func (c *Client) Restart(r *proto.RestartRequest) (err error) {
	ctx := context.Background()
	_, err = c.client.Restart(ctx, r)
	if err != nil {
		return
	}

	return nil
}

// Reset implements the proto.OSDClient interface.
func (c *Client) Reset() (err error) {
	ctx := context.Background()
	_, err = c.client.Reset(ctx, &empty.Empty{})
	if err != nil {
		return
	}

	return nil
}

// Reboot implements the proto.OSDClient interface.
func (c *Client) Reboot() (err error) {
	ctx := context.Background()
	_, err = c.client.Reboot(ctx, &empty.Empty{})
	if err != nil {
		return
	}

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

// Routes implements the proto.OSDClient interface.
func (c *Client) Routes() (err error) {
	ctx := context.Background()
	reply, err := c.client.Routes(ctx, &empty.Empty{})
	if err != nil {
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "INTERFACE\tDESTINATION\tGATEWAY")
	for _, r := range reply.Routes {
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.Interface, r.Destination, r.Gateway)
	}
	if err := w.Flush(); err != nil {
		return err
	}

	return nil
}
