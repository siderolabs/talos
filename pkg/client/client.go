// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	grpctls "github.com/talos-systems/talos/pkg/grpc/tls"

	"github.com/talos-systems/talos/api/common"
	machineapi "github.com/talos-systems/talos/api/machine"
	networkapi "github.com/talos-systems/talos/api/network"
	osapi "github.com/talos-systems/talos/api/os"
	timeapi "github.com/talos-systems/talos/api/time"
	"github.com/talos-systems/talos/pkg/client/config"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/net"
)

// Credentials represents the set of values required to initialize a valid
// Client.
type Credentials struct {
	CA  []byte
	Crt tls.Certificate
}

// Client implements the proto.OSClient interface. It serves as the
// concrete type with the required methods.
type Client struct {
	endpointsOverride []string
	config            *config.Context
	tlsConfig         *tls.Config
	grpcDialOptions   []grpc.DialOption
	conn              *grpc.ClientConn
	client            osapi.OSServiceClient
	MachineClient     machineapi.MachineServiceClient
	TimeClient        timeapi.TimeServiceClient
	NetworkClient     networkapi.NetworkServiceClient
}

// Option sets an option for the creation of the Client
type Option func(*Client) error

// WithConfigContext configures the Client with the configuration context provided.
func WithConfigContext(cfg *config.Context) Option {
	return func(c *Client) error {
		c.config = cfg
		return nil
	}
}

// WithGRPCDialOptions adds the given grpc.DialOptions to a Client.
func WithGRPCDialOptions(opts ...grpc.DialOption) Option {
	return func(c *Client) error {
		c.grpcDialOptions = append(c.grpcDialOptions, opts...)
		return nil
	}
}

// WithTLSConfig overrides the default TLS configuration with the one provided.
func WithTLSConfig(tlsConfig *tls.Config) Option {
	return func(c *Client) error {
		c.tlsConfig = tlsConfig
		return nil
	}
}

// WithEndpoints overrides the default endpoints with the provided list.
func WithEndpoints(endpoints ...string) Option {
	return func(c *Client) error {
		c.endpointsOverride = endpoints
		return nil
	}
}

func withDefaultConfig() Option {
	return func(c *Client) (err error) {
		defaultConfigPath, err := config.GetDefaultPath()
		if err != nil {
			return fmt.Errorf("no client configuration provided and no default path found: %w", err)
		}

		return WithConfigContextFromFile(defaultConfigPath, "")(c)
	}
}

// WithConfigContextFromFile creates a Client with its configuration extracted from the given file in the given context.
// Supplying an empty context will return the default context contained within the configuration file.
func WithConfigContextFromFile(fn, context string) Option {
	return func(c *Client) (err error) {
		fullConfig, err := config.Open(fn)
		if err != nil {
			return fmt.Errorf("failed to read config from %q: %w", fn, err)
		}

		cfgContext := fullConfig.GetContext(context)
		if cfgContext == nil {
			return fmt.Errorf("context %q not found in config file %q", context, fn)
		}

		return WithConfigContext(cfgContext)(c)
	}
}

func (c *Client) getEndpoints() []string {
	if len(c.endpointsOverride) > 0 {
		return c.endpointsOverride
	}

	if c.config != nil {
		return c.config.Endpoints
	}

	return nil
}

// New returns a new Client.
func New(ctx context.Context, opts ...Option) (c *Client, err error) {
	c = new(Client)

	for _, opt := range opts {
		if err = opt(c); err != nil {
			return nil, err
		}
	}

	if len(c.getEndpoints()) < 1 {
		if err = withDefaultConfig()(c); err != nil {
			return nil, fmt.Errorf("failed to load default config: %w", err)
		}
	}

	c.conn, err = c.getConn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create client connection: %w", err)
	}

	c.client = osapi.NewOSServiceClient(c.conn)
	c.MachineClient = machineapi.NewMachineServiceClient(c.conn)
	c.TimeClient = timeapi.NewTimeServiceClient(c.conn)
	c.NetworkClient = networkapi.NewNetworkServiceClient(c.conn)

	return c, nil
}

func (c *Client) getConn(ctx context.Context, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	endpoints := c.getEndpoints()

	target := fmt.Sprintf("dns:///%s:%d", net.FormatAddress(endpoints[0]), constants.ApidPort)

	// If we are given more than one endpoint, we need to use our custom list resolver
	if len(endpoints) > 1 {
		target = fmt.Sprintf("%s:///%s", talosListResolverScheme, strings.Join(c.config.Endpoints, ","))
	}

	// Add TLS credentials to gRPC DialOptions
	tlsConfig := c.tlsConfig
	if tlsConfig == nil {
		creds, err := CredentialsFromConfigContext(c.config)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire credentials: %w", err)
		}

		tlsConfig, err = grpctls.New(
			grpctls.WithKeypair(creds.Crt),
			grpctls.WithClientAuthType(grpctls.Mutual),
			grpctls.WithCACertPEM(creds.CA),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to construct TLS credentials: %w", err)
		}
	}

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	}

	dialOpts = append(dialOpts, c.grpcDialOptions...)

	dialOpts = append(dialOpts, opts...)

	return grpc.DialContext(ctx, target, dialOpts...)
}

// CredentialsFromConfigContext constructs the client Credentials from the given configuration Context.
func CredentialsFromConfigContext(context *config.Context) (*Credentials, error) {
	caBytes, err := base64.StdEncoding.DecodeString(context.CA)
	if err != nil {
		return nil, fmt.Errorf("error decoding CA: %w", err)
	}

	crtBytes, err := base64.StdEncoding.DecodeString(context.Crt)
	if err != nil {
		return nil, fmt.Errorf("error decoding certificate: %w", err)
	}

	keyBytes, err := base64.StdEncoding.DecodeString(context.Key)
	if err != nil {
		return nil, fmt.Errorf("error decoding key: %w", err)
	}

	crt, err := tls.X509KeyPair(crtBytes, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("could not load client key pair: %s", err)
	}

	return &Credentials{
		CA:  caBytes,
		Crt: crt,
	}, nil
}

// NewClientContextAndCredentialsFromConfig initializes Credentials from config file.
//
// Deprecated: use Option-based methods for client creation.
func NewClientContextAndCredentialsFromConfig(p string, ctx string) (context *config.Context, creds *Credentials, err error) {
	c, err := config.Open(p)
	if err != nil {
		return
	}

	context, creds, err = NewClientContextAndCredentialsFromParsedConfig(c, ctx)

	return
}

// NewClientContextAndCredentialsFromParsedConfig initializes Credentials from parsed configuration.
//
// Deprecated: use Option-based methods for client creation.
func NewClientContextAndCredentialsFromParsedConfig(c *config.Config, ctx string) (context *config.Context, creds *Credentials, err error) {
	if ctx != "" {
		c.Context = ctx
	}

	if c.Context == "" {
		return nil, nil, fmt.Errorf("'context' key is not set in the config")
	}

	context = c.Contexts[c.Context]
	if context == nil {
		return nil, nil, fmt.Errorf("context %q is not defined in 'contexts' key in config", c.Context)
	}

	creds, err = CredentialsFromConfigContext(context)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to extract credentials from context: %w", err)
	}

	return context, creds, nil
}

// NewClient initializes a Client.
//
// Deprecated: use client.NewFromConfigContext() instead
func NewClient(cfg *tls.Config, endpoints []string, port int, opts ...grpc.DialOption) (c *Client, err error) {
	opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(cfg)))

	cfg.ServerName = endpoints[0]

	c = &Client{}

	// TODO(smira): endpoints[0] should be replaced with proper load-balancing
	c.conn, err = grpc.DialContext(context.Background(), fmt.Sprintf("%s:%d", net.FormatAddress(endpoints[0]), port), opts...)
	if err != nil {
		return
	}

	c.client = osapi.NewOSServiceClient(c.conn)
	c.MachineClient = machineapi.NewMachineServiceClient(c.conn)
	c.TimeClient = timeapi.NewTimeServiceClient(c.conn)
	c.NetworkClient = networkapi.NewNetworkServiceClient(c.conn)

	return c, nil
}

// Close shuts down client protocol
func (c *Client) Close() error {
	return c.conn.Close()
}

// KubeconfigRaw returns K8s client config (kubeconfig).
func (c *Client) KubeconfigRaw(ctx context.Context) (io.ReadCloser, <-chan error, error) {
	stream, err := c.MachineClient.Kubeconfig(ctx, &empty.Empty{})
	if err != nil {
		return nil, nil, err
	}

	return ReadStream(stream)
}

func (c *Client) extractKubeconfig(r io.ReadCloser) ([]byte, error) {
	defer r.Close() //nolint: errcheck

	gzR, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	// returned .tar.gz should contain only single file (kubeconfig)
	var kubeconfigBuf bytes.Buffer

	tar := tar.NewReader(gzR)

	for {
		_, err = tar.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		_, err = io.Copy(&kubeconfigBuf, tar)
		if err != nil {
			return nil, err
		}
	}

	if err = gzR.Close(); err != nil {
		return nil, err
	}

	return kubeconfigBuf.Bytes(), nil
}

// Kubeconfig returns K8s client config (kubeconfig).
func (c *Client) Kubeconfig(ctx context.Context) ([]byte, error) {
	r, errCh, err := c.KubeconfigRaw(ctx)
	if err != nil {
		return nil, err
	}

	kubeconfig, err := c.extractKubeconfig(r)

	if err2 := <-errCh; err2 != nil {
		// prefer errCh (error from server) as if server failed,
		// extractKubeconfig failed as well, but server failure is more descriptive
		return nil, err2
	}

	return kubeconfig, err
}

// Stats implements the proto.OSClient interface.
func (c *Client) Stats(ctx context.Context, namespace string, driver common.ContainerDriver, callOptions ...grpc.CallOption) (resp *osapi.StatsResponse, err error) {
	resp, err = c.client.Stats(
		ctx, &osapi.StatsRequest{
			Namespace: namespace,
			Driver:    driver,
		},
		callOptions...,
	)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*osapi.StatsResponse) //nolint: errcheck

	return
}

// Containers implements the proto.OSClient interface.
func (c *Client) Containers(ctx context.Context, namespace string, driver common.ContainerDriver, callOptions ...grpc.CallOption) (resp *osapi.ContainersResponse, err error) {
	resp, err = c.client.Containers(
		ctx,
		&osapi.ContainersRequest{
			Namespace: namespace,
			Driver:    driver,
		},
		callOptions...,
	)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*osapi.ContainersResponse) //nolint: errcheck

	return
}

// Restart implements the proto.OSClient interface.
func (c *Client) Restart(ctx context.Context, namespace string, driver common.ContainerDriver, id string, callOptions ...grpc.CallOption) (err error) {
	_, err = c.client.Restart(ctx, &osapi.RestartRequest{
		Id:        id,
		Namespace: namespace,
		Driver:    driver,
	})

	return
}

// Reset implements the proto.OSClient interface.
func (c *Client) Reset(ctx context.Context, graceful, reboot bool) (err error) {
	_, err = c.MachineClient.Reset(ctx, &machineapi.ResetRequest{Graceful: graceful, Reboot: reboot})
	return
}

// Reboot implements the proto.OSClient interface.
func (c *Client) Reboot(ctx context.Context) (err error) {
	_, err = c.MachineClient.Reboot(ctx, &empty.Empty{})
	return
}

// Recover implements the proto.OSClient interface.
func (c *Client) Recover(ctx context.Context, source machineapi.RecoverRequest_Source) (err error) {
	_, err = c.MachineClient.Recover(ctx, &machineapi.RecoverRequest{Source: source})
	return
}

// Bootstrap implements the proto.OSClient interface.
func (c *Client) Bootstrap(ctx context.Context) (err error) {
	_, err = c.MachineClient.Bootstrap(ctx, &machineapi.BootstrapRequest{})
	return
}

// Shutdown implements the proto.OSClient interface.
func (c *Client) Shutdown(ctx context.Context) (err error) {
	_, err = c.MachineClient.Shutdown(ctx, &empty.Empty{})
	return
}

// Dmesg implements the proto.OSClient interface.
func (c *Client) Dmesg(ctx context.Context, follow, tail bool) (osapi.OSService_DmesgClient, error) {
	return c.client.Dmesg(ctx, &osapi.DmesgRequest{
		Follow: follow,
		Tail:   tail,
	})
}

// Logs implements the proto.OSClient interface.
func (c *Client) Logs(ctx context.Context, namespace string, driver common.ContainerDriver, id string, follow bool, tailLines int32) (stream machineapi.MachineService_LogsClient, err error) {
	stream, err = c.MachineClient.Logs(ctx, &machineapi.LogsRequest{
		Namespace: namespace,
		Driver:    driver,
		Id:        id,
		Follow:    follow,
		TailLines: tailLines,
	})

	return
}

// Version implements the proto.OSClient interface.
func (c *Client) Version(ctx context.Context, callOptions ...grpc.CallOption) (resp *machineapi.VersionResponse, err error) {
	resp, err = c.MachineClient.Version(
		ctx,
		&empty.Empty{},
		callOptions...,
	)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*machineapi.VersionResponse) //nolint: errcheck

	return
}

// Routes implements the networkdproto.NetworkClient interface.
func (c *Client) Routes(ctx context.Context, callOptions ...grpc.CallOption) (resp *networkapi.RoutesResponse, err error) {
	resp, err = c.NetworkClient.Routes(
		ctx,
		&empty.Empty{},
		callOptions...,
	)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*networkapi.RoutesResponse) //nolint: errcheck

	return
}

// Interfaces implements the proto.OSClient interface.
func (c *Client) Interfaces(ctx context.Context, callOptions ...grpc.CallOption) (resp *networkapi.InterfacesResponse, err error) {
	resp, err = c.NetworkClient.Interfaces(
		ctx,
		&empty.Empty{},
		callOptions...,
	)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*networkapi.InterfacesResponse) //nolint: errcheck

	return
}

// Processes implements the proto.OSClient interface.
func (c *Client) Processes(ctx context.Context, callOptions ...grpc.CallOption) (resp *osapi.ProcessesResponse, err error) {
	resp, err = c.client.Processes(
		ctx,
		&empty.Empty{},
		callOptions...,
	)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*osapi.ProcessesResponse) //nolint: errcheck

	return
}

// Memory implements the proto.OSClient interface.
func (c *Client) Memory(ctx context.Context, callOptions ...grpc.CallOption) (resp *osapi.MemoryResponse, err error) {
	resp, err = c.client.Memory(
		ctx,
		&empty.Empty{},
		callOptions...,
	)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*osapi.MemoryResponse) //nolint: errcheck

	return
}

// Mounts implements the proto.OSClient interface.
func (c *Client) Mounts(ctx context.Context, callOptions ...grpc.CallOption) (resp *machineapi.MountsResponse, err error) {
	resp, err = c.MachineClient.Mounts(
		ctx,
		&empty.Empty{},
		callOptions...,
	)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*machineapi.MountsResponse) //nolint: errcheck

	return
}

// LS implements the proto.OSClient interface.
func (c *Client) LS(ctx context.Context, req machineapi.ListRequest) (stream machineapi.MachineService_ListClient, err error) {
	return c.MachineClient.List(ctx, &req)
}

// Copy implements the proto.OSClient interface
func (c *Client) Copy(ctx context.Context, rootPath string) (io.ReadCloser, <-chan error, error) {
	stream, err := c.MachineClient.Copy(ctx, &machineapi.CopyRequest{
		RootPath: rootPath,
	})
	if err != nil {
		return nil, nil, err
	}

	return ReadStream(stream)
}

// Upgrade initiates a Talos upgrade ... and implements the proto.OSClient
// interface
func (c *Client) Upgrade(ctx context.Context, image string, preserve bool, callOptions ...grpc.CallOption) (resp *machineapi.UpgradeResponse, err error) {
	resp, err = c.MachineClient.Upgrade(
		ctx,
		&machineapi.UpgradeRequest{
			Image:    image,
			Preserve: preserve,
		},
		callOptions...,
	)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*machineapi.UpgradeResponse) //nolint: errcheck

	return
}

// ServiceList returns list of services with their state
func (c *Client) ServiceList(ctx context.Context, callOptions ...grpc.CallOption) (resp *machineapi.ServiceListResponse, err error) {
	resp, err = c.MachineClient.ServiceList(
		ctx,
		&empty.Empty{},
		callOptions...,
	)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*machineapi.ServiceListResponse) //nolint: errcheck

	return
}

// ServiceInfo provides info about a service and node metadata
type ServiceInfo struct {
	Metadata *common.Metadata
	Service  *machineapi.ServiceInfo
}

// ServiceInfo returns info about a single service
//
// This is implemented via service list API, as we don't have many services
// If service with given id is not registered, function returns nil
func (c *Client) ServiceInfo(ctx context.Context, id string, callOptions ...grpc.CallOption) (services []ServiceInfo, err error) {
	var resp *machineapi.ServiceListResponse

	resp, err = c.MachineClient.ServiceList(
		ctx,
		&empty.Empty{},
		callOptions...,
	)

	if err != nil {
		return
	}

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*machineapi.ServiceListResponse) //nolint: errcheck

	for _, resp := range resp.Messages {
		for _, svc := range resp.Services {
			if svc.Id == id {
				services = append(services, ServiceInfo{
					Metadata: resp.Metadata,
					Service:  svc,
				})
			}
		}
	}

	return
}

// ServiceStart starts a service.
func (c *Client) ServiceStart(ctx context.Context, id string, callOptions ...grpc.CallOption) (resp *machineapi.ServiceStartResponse, err error) {
	resp, err = c.MachineClient.ServiceStart(
		ctx,
		&machineapi.ServiceStartRequest{Id: id},
		callOptions...,
	)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*machineapi.ServiceStartResponse) //nolint: errcheck

	return
}

// ServiceStop stops a service.
func (c *Client) ServiceStop(ctx context.Context, id string, callOptions ...grpc.CallOption) (resp *machineapi.ServiceStopResponse, err error) {
	resp, err = c.MachineClient.ServiceStop(
		ctx,
		&machineapi.ServiceStopRequest{Id: id},
		callOptions...,
	)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*machineapi.ServiceStopResponse) //nolint: errcheck

	return
}

// ServiceRestart restarts a service.
func (c *Client) ServiceRestart(ctx context.Context, id string, callOptions ...grpc.CallOption) (resp *machineapi.ServiceRestartResponse, err error) {
	resp, err = c.MachineClient.ServiceRestart(
		ctx,
		&machineapi.ServiceRestartRequest{Id: id},
		callOptions...,
	)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*machineapi.ServiceRestartResponse) //nolint: errcheck

	return
}

// Time returns the time
func (c *Client) Time(ctx context.Context, callOptions ...grpc.CallOption) (resp *timeapi.TimeResponse, err error) {
	resp, err = c.TimeClient.Time(
		ctx,
		&empty.Empty{},
		callOptions...,
	)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*timeapi.TimeResponse) //nolint: errcheck

	return
}

// TimeCheck returns the time compared to the specified ntp server
func (c *Client) TimeCheck(ctx context.Context, server string, callOptions ...grpc.CallOption) (resp *timeapi.TimeResponse, err error) {
	resp, err = c.TimeClient.TimeCheck(
		ctx,
		&timeapi.TimeRequest{Server: server},
		callOptions...,
	)

	var filtered interface{}
	filtered, err = FilterMessages(resp, err)
	resp, _ = filtered.(*timeapi.TimeResponse) //nolint: errcheck

	return
}

// Read reads a file.
func (c *Client) Read(ctx context.Context, path string) (io.ReadCloser, <-chan error, error) {
	stream, err := c.MachineClient.Read(ctx, &machineapi.ReadRequest{Path: path})
	if err != nil {
		return nil, nil, err
	}

	return ReadStream(stream)
}

// MachineStream is a common interface for streams returned by streaming APIs.
type MachineStream interface {
	Recv() (*common.Data, error)
	grpc.ClientStream
}

// ReadStream converts grpc stream into io.Reader.
func ReadStream(stream MachineStream) (io.ReadCloser, <-chan error, error) {
	errCh := make(chan error, 1)
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

			if data.Metadata != nil && data.Metadata.Error != "" {
				errCh <- errors.New(data.Metadata.Error)
			}
		}
	}()

	return pr, errCh, stream.CloseSend()
}
