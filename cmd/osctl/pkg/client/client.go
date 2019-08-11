/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"io"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client/config"
	initproto "github.com/talos-systems/talos/internal/app/machined/proto"
	"github.com/talos-systems/talos/internal/app/osd/proto"
	"github.com/talos-systems/talos/pkg/net"
	"github.com/talos-systems/talos/pkg/proc"
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
	conn       *grpc.ClientConn
	client     proto.OSDClient
	initClient initproto.InitClient
}

// NewDefaultClientCredentials initializes ClientCredentials using default paths
// to the required CA, certificate, and key.
func NewDefaultClientCredentials(p string) (creds *Credentials, err error) {
	c, err := config.Open(p)
	if err != nil {
		return
	}

	if c.Context == "" {
		return nil, fmt.Errorf("'context' key is not set in the config")
	}

	context := c.Contexts[c.Context]
	if context == nil {
		return nil, fmt.Errorf("context %q is not defined in 'contexts' key in config", c.Context)
	}

	caBytes, err := base64.StdEncoding.DecodeString(context.CA)
	if err != nil {
		return
	}
	crtBytes, err := base64.StdEncoding.DecodeString(context.Crt)
	if err != nil {
		return
	}
	keyBytes, err := base64.StdEncoding.DecodeString(context.Key)
	if err != nil {
		return
	}
	creds = &Credentials{
		Target: context.Target,
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
	c.conn, err = grpc.Dial(fmt.Sprintf("%s:%d", net.FormatAddress(clientcreds.Target), port), grpcOpts...)
	if err != nil {
		return
	}

	c.client = proto.NewOSDClient(c.conn)
	c.initClient = initproto.NewInitClient(c.conn)

	return c, nil
}

// Close shuts down client protocol
func (c *Client) Close() error {
	return c.conn.Close()
}

// Kubeconfig implements the proto.OSDClient interface.
func (c *Client) Kubeconfig(ctx context.Context) ([]byte, error) {
	r, err := c.client.Kubeconfig(ctx, &empty.Empty{})
	if err != nil {
		return nil, err
	}
	return r.Bytes, nil
}

// Stats implements the proto.OSDClient interface.
func (c *Client) Stats(ctx context.Context, namespace string, driver proto.ContainerDriver) (reply *proto.StatsReply, err error) {
	reply, err = c.client.Stats(ctx, &proto.StatsRequest{
		Namespace: namespace,
		Driver:    driver,
	})
	return
}

// Processes implements the proto.OSDClient interface.
func (c *Client) Processes(ctx context.Context, namespace string, driver proto.ContainerDriver) (reply *proto.ProcessesReply, err error) {
	reply, err = c.client.Processes(ctx, &proto.ProcessesRequest{
		Namespace: namespace,
		Driver:    driver,
	})
	return
}

// Restart implements the proto.OSDClient interface.
func (c *Client) Restart(ctx context.Context, namespace string, driver proto.ContainerDriver, id string) (err error) {
	_, err = c.client.Restart(ctx, &proto.RestartRequest{
		Id:        id,
		Namespace: namespace,
		Driver:    driver,
	})
	return
}

// Reset implements the proto.OSDClient interface.
func (c *Client) Reset(ctx context.Context) (err error) {
	_, err = c.initClient.Reset(ctx, &empty.Empty{})
	return
}

// Reboot implements the proto.OSDClient interface.
func (c *Client) Reboot(ctx context.Context) (err error) {
	_, err = c.initClient.Reboot(ctx, &empty.Empty{})
	return
}

// Shutdown implements the proto.OSDClient interface.
func (c *Client) Shutdown(ctx context.Context) (err error) {
	_, err = c.initClient.Shutdown(ctx, &empty.Empty{})
	return
}

// Dmesg implements the proto.OSDClient interface.
func (c *Client) Dmesg(ctx context.Context) ([]byte, error) {
	data, err := c.client.Dmesg(ctx, &empty.Empty{})
	if err != nil {
		return nil, err
	}

	return data.Bytes, nil
}

// Logs implements the proto.OSDClient interface.
func (c *Client) Logs(ctx context.Context, namespace string, driver proto.ContainerDriver, id string) (stream proto.OSD_LogsClient, err error) {
	stream, err = c.client.Logs(ctx, &proto.LogsRequest{
		Namespace: namespace,
		Driver:    driver,
		Id:        id,
	})
	return
}

// Version implements the proto.OSDClient interface.
func (c *Client) Version(ctx context.Context) ([]byte, error) {
	data, err := c.client.Version(ctx, &empty.Empty{})
	if err != nil {
		return nil, err
	}
	return data.Bytes, nil
}

// Routes implements the proto.OSDClient interface.
func (c *Client) Routes(ctx context.Context) (reply *proto.RoutesReply, err error) {
	reply, err = c.client.Routes(ctx, &empty.Empty{})
	return
}

// Top implements the proto.OSDClient interface.
func (c *Client) Top(ctx context.Context) (pl []proc.ProcessList, err error) {
	var reply *proto.TopReply
	reply, err = c.client.Top(ctx, &empty.Empty{})
	if err != nil {
		return
	}

	buf := bytes.NewBuffer(reply.ProcessList.Bytes)
	dec := gob.NewDecoder(buf)
	err = dec.Decode(&pl)
	return
}

// DF implements the proto.OSDClient interface.
func (c *Client) DF(ctx context.Context) (*initproto.DFReply, error) {
	return c.initClient.DF(ctx, &empty.Empty{})
}

// LS implements the proto.OSDClient interface.
func (c *Client) LS(ctx context.Context, req initproto.LSRequest) (stream initproto.Init_LSClient, err error) {
	return c.initClient.LS(ctx, &req)
}

// CopyOut implements the proto.OSDClient interface
func (c *Client) CopyOut(ctx context.Context, rootPath string) (io.Reader, <-chan error, error) {
	stream, err := c.initClient.CopyOut(ctx, &initproto.CopyOutRequest{
		RootPath: rootPath,
	})
	if err != nil {
		return nil, nil, err
	}

	errCh := make(chan error)

	pr, pw := io.Pipe()

	go func() {
		//nolint: errcheck
		defer pw.Close()
		defer close(errCh)

		for {

			data, err := stream.Recv()
			if err != nil {
				if err == io.EOF || status.Code(err) == codes.Canceled {
					return
				}
				//nolint: errcheck
				pw.CloseWithError(err)
				return
			}

			if data.Bytes != nil {
				_, err = pw.Write(data.Bytes)
				if err != nil {
					return
				}
			}

			if data.Errors != "" {
				errCh <- errors.New(data.Errors)
			}
		}
	}()

	return pr, errCh, nil
}

// Upgrade initiates a Talos upgrade ... and implements the proto.OSDClient
// interface
func (c *Client) Upgrade(ctx context.Context, asseturl string) (string, error) {
	reply, err := c.initClient.Upgrade(ctx, &initproto.UpgradeRequest{Url: asseturl})
	if err != nil {
		return "", err
	}
	return reply.Ack, nil
}

// ServiceList returns list of services with their state
func (c *Client) ServiceList(ctx context.Context) (*initproto.ServiceListReply, error) {
	return c.initClient.ServiceList(ctx, &empty.Empty{})
}

// ServiceInfo returns info about a single service
//
// This is implemented via service list API, as we don't have many services
// If service with given id is not registered, function returns nil
func (c *Client) ServiceInfo(ctx context.Context, id string) (*initproto.ServiceInfo, error) {
	reply, err := c.initClient.ServiceList(ctx, &empty.Empty{})
	if err != nil {
		return nil, err
	}

	for _, svc := range reply.Services {
		if svc.Id == id {
			return svc, nil
		}
	}

	return nil, nil
}

// Start starts a service.
func (c *Client) Start(ctx context.Context, id string) (string, error) {
	r, err := c.initClient.Start(ctx, &initproto.StartRequest{Id: id})
	if err != nil {
		return "", err
	}

	return r.Resp, nil
}

// Stop stops a service.
func (c *Client) Stop(ctx context.Context, id string) (string, error) {
	r, err := c.initClient.Stop(ctx, &initproto.StopRequest{Id: id})
	if err != nil {
		return "", err
	}

	return r.Resp, nil
}
