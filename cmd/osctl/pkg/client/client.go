// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	machineapi "github.com/talos-systems/talos/api/machine"
	networkapi "github.com/talos-systems/talos/api/network"
	osapi "github.com/talos-systems/talos/api/os"
	timeapi "github.com/talos-systems/talos/api/time"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client/config"
	"github.com/talos-systems/talos/pkg/net"
)

// Credentials represents the set of values required to initialize a vaild
// Client.
type Credentials struct {
	ca  []byte
	crt []byte
	key []byte
}

// Client implements the proto.OSClient interface. It serves as the
// concrete type with the required methods.
type Client struct {
	conn          *grpc.ClientConn
	client        osapi.OSClient
	MachineClient machineapi.MachineClient
	TimeClient    timeapi.TimeClient
	NetworkClient networkapi.NetworkClient
}

// NewClientTargetAndCredentialsFromConfig initializes ClientCredentials using default paths
// to the required CA, certificate, and key.
func NewClientTargetAndCredentialsFromConfig(p string) (target string, creds *Credentials, err error) {
	c, err := config.Open(p)
	if err != nil {
		return
	}

	if c.Context == "" {
		return "", nil, fmt.Errorf("'context' key is not set in the config")
	}

	context := c.Contexts[c.Context]
	if context == nil {
		return "", nil, fmt.Errorf("context %q is not defined in 'contexts' key in config", c.Context)
	}

	caBytes, err := base64.StdEncoding.DecodeString(context.CA)
	if err != nil {
		return "", nil, fmt.Errorf("error decoding CA: %w", err)
	}

	crtBytes, err := base64.StdEncoding.DecodeString(context.Crt)
	if err != nil {
		return "", nil, fmt.Errorf("error decoding certificate: %w", err)
	}

	keyBytes, err := base64.StdEncoding.DecodeString(context.Key)
	if err != nil {
		return "", nil, fmt.Errorf("error decoding key: %w", err)
	}

	creds = &Credentials{
		ca:  caBytes,
		crt: crtBytes,
		key: keyBytes,
	}

	return context.Target, creds, nil
}

// NewClientCredentials initializes ClientCredentials using default paths
// to the required CA, certificate, and key.
func NewClientCredentials(ca, crt, key []byte) (creds *Credentials) {
	creds = &Credentials{
		ca:  ca,
		crt: crt,
		key: key,
	}

	return creds
}

// NewClient initializes a Client.
func NewClient(creds *Credentials, target string, port int) (c *Client, err error) {
	grpcOpts := []grpc.DialOption{}

	c = &Client{}

	crt, err := tls.X509KeyPair(creds.crt, creds.key)
	if err != nil {
		return nil, fmt.Errorf("could not load client key pair: %s", err)
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(creds.ca); !ok {
		return nil, fmt.Errorf("failed to append client certs")
	}

	// TODO(andrewrynhard): Do not parse the address. Pass the IP and port in as separate
	// parameters.
	transportCreds := credentials.NewTLS(&tls.Config{
		ServerName:   target,
		Certificates: []tls.Certificate{crt},
		// Set the root certificate authorities to use the self-signed
		// certificate.
		RootCAs: certPool,
	})

	grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(transportCreds))

	c.conn, err = grpc.Dial(fmt.Sprintf("%s:%d", net.FormatAddress(target), port), grpcOpts...)
	if err != nil {
		return
	}

	c.client = osapi.NewOSClient(c.conn)
	c.MachineClient = machineapi.NewMachineClient(c.conn)
	c.TimeClient = timeapi.NewTimeClient(c.conn)
	c.NetworkClient = networkapi.NewNetworkClient(c.conn)

	return c, nil
}

// Close shuts down client protocol
func (c *Client) Close() error {
	return c.conn.Close()
}

// Kubeconfig implements the proto.OSClient interface.
func (c *Client) Kubeconfig(ctx context.Context) (*osapi.DataReply, error) {
	return c.client.Kubeconfig(ctx, &empty.Empty{})
}

// Stats implements the proto.OSClient interface.
func (c *Client) Stats(ctx context.Context, namespace string, driver osapi.ContainerDriver) (reply *osapi.StatsReply, err error) {
	reply, err = c.client.Stats(ctx, &osapi.StatsRequest{
		Namespace: namespace,
		Driver:    driver,
	})

	return
}

// Containers implements the proto.OSClient interface.
func (c *Client) Containers(ctx context.Context, namespace string, driver osapi.ContainerDriver) (reply *osapi.ContainersReply, err error) {
	reply, err = c.client.Containers(ctx, &osapi.ContainersRequest{
		Namespace: namespace,
		Driver:    driver,
	})

	return
}

// Restart implements the proto.OSClient interface.
func (c *Client) Restart(ctx context.Context, namespace string, driver osapi.ContainerDriver, id string) (err error) {
	_, err = c.client.Restart(ctx, &osapi.RestartRequest{
		Id:        id,
		Namespace: namespace,
		Driver:    driver,
	})

	return
}

// Reset implements the proto.OSClient interface.
func (c *Client) Reset(ctx context.Context) (err error) {
	_, err = c.MachineClient.Reset(ctx, &empty.Empty{})
	return
}

// Reboot implements the proto.OSClient interface.
func (c *Client) Reboot(ctx context.Context) (err error) {
	_, err = c.MachineClient.Reboot(ctx, &empty.Empty{})
	return
}

// Shutdown implements the proto.OSClient interface.
func (c *Client) Shutdown(ctx context.Context) (err error) {
	_, err = c.MachineClient.Shutdown(ctx, &empty.Empty{})
	return
}

// Dmesg implements the proto.OSClient interface.
func (c *Client) Dmesg(ctx context.Context) (*osapi.DataReply, error) {
	return c.client.Dmesg(ctx, &empty.Empty{})
}

// Logs implements the proto.OSClient interface.
func (c *Client) Logs(ctx context.Context, namespace string, driver osapi.ContainerDriver, id string) (stream osapi.OS_LogsClient, err error) {
	stream, err = c.client.Logs(ctx, &osapi.LogsRequest{
		Namespace: namespace,
		Driver:    driver,
		Id:        id,
	})

	return
}

// Version implements the proto.OSClient interface.
func (c *Client) Version(ctx context.Context) (*machineapi.VersionReply, error) {
	return c.MachineClient.Version(ctx, &empty.Empty{})
}

// Routes implements the networkdproto.NetworkClient interface.
func (c *Client) Routes(ctx context.Context) (*networkapi.RoutesReply, error) {
	return c.NetworkClient.Routes(ctx, &empty.Empty{})
}

// Interfaces implements the proto.OSClient interface.
func (c *Client) Interfaces(ctx context.Context) (*networkapi.InterfacesReply, error) {
	return c.NetworkClient.Interfaces(ctx, &empty.Empty{})
}

// Processes implements the proto.OSClient interface.
func (c *Client) Processes(ctx context.Context) (reply *osapi.ProcessesReply, err error) {
	return c.client.Processes(ctx, &empty.Empty{})
}

// Mounts implements the proto.OSClient interface.
func (c *Client) Mounts(ctx context.Context) (*machineapi.MountsReply, error) {
	return c.MachineClient.Mounts(ctx, &empty.Empty{})
}

// LS implements the proto.OSClient interface.
func (c *Client) LS(ctx context.Context, req machineapi.LSRequest) (stream machineapi.Machine_LSClient, err error) {
	return c.MachineClient.LS(ctx, &req)
}

// CopyOut implements the proto.OSClient interface
func (c *Client) CopyOut(ctx context.Context, rootPath string) (io.Reader, <-chan error, error) {
	stream, err := c.MachineClient.CopyOut(ctx, &machineapi.CopyOutRequest{
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

// Upgrade initiates a Talos upgrade ... and implements the proto.OSClient
// interface
func (c *Client) Upgrade(ctx context.Context, image string) (*machineapi.UpgradeReply, error) {
	return c.MachineClient.Upgrade(ctx, &machineapi.UpgradeRequest{Image: image})
}

// ServiceList returns list of services with their state
func (c *Client) ServiceList(ctx context.Context) (*machineapi.ServiceListReply, error) {
	return c.MachineClient.ServiceList(ctx, &empty.Empty{})
}

// ServiceInfo returns info about a single service
//
// This is implemented via service list API, as we don't have many services
// If service with given id is not registered, function returns nil
func (c *Client) ServiceInfo(ctx context.Context, id string) (*machineapi.ServiceListReply, error) {
	return c.MachineClient.ServiceList(ctx, &empty.Empty{})
}

// ServiceStart starts a service.
func (c *Client) ServiceStart(ctx context.Context, id string) (*machineapi.ServiceStartReply, error) {
	return c.MachineClient.ServiceStart(ctx, &machineapi.ServiceStartRequest{Id: id})
}

// ServiceStop stops a service.
func (c *Client) ServiceStop(ctx context.Context, id string) (*machineapi.ServiceStopReply, error) {
	return c.MachineClient.ServiceStop(ctx, &machineapi.ServiceStopRequest{Id: id})
}

// ServiceRestart restarts a service.
func (c *Client) ServiceRestart(ctx context.Context, id string) (*machineapi.ServiceRestartReply, error) {
	return c.MachineClient.ServiceRestart(ctx, &machineapi.ServiceRestartRequest{Id: id})
}

// Time returns the time
func (c *Client) Time(ctx context.Context) (*timeapi.TimeReply, error) {
	return c.TimeClient.Time(ctx, &empty.Empty{})
}

// TimeCheck returns the time compared to the specified ntp server
func (c *Client) TimeCheck(ctx context.Context, server string) (*timeapi.TimeReply, error) {
	return c.TimeClient.TimeCheck(ctx, &timeapi.TimeRequest{Server: server})
}
