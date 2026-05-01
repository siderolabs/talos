// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package aws contains the AWS implementation of the [platform.Platform].
package aws

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/netip"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	platformerrors "github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// awsInterfaceName is the name of the (single) network interface configured by the AWS platform.
const awsInterfaceName = "eth0"

// awsIPv6DNSServer is the link-local IPv6 address of the Amazon-provided DNS resolver inside a VPC.
//
// See https://docs.aws.amazon.com/vpc/latest/userguide/AmazonDNS-concepts.html
//
// AWS distributes IPv6 addresses via DHCPv6 (stateful) and the IPv6 default route via Router Advertisements
// (the kernel installs it automatically when accept_ra is enabled). DHCPv6 in EC2 does not advertise
// resolvers, so we configure the well-known DNS address explicitly to make IPv6-only instances usable.
const awsIPv6DNSServer = "fd00:ec2::253"

// AWS is the concrete type that implements the runtime.Platform interface.
type AWS struct {
	cfg aws.Config
}

// NewAWS initializes AWS platform.
//
// The IMDS client is built lazily on first use because the IMDS endpoint
// (IPv4 vs IPv6) can only be determined once the network is reachable —
// AWS supports IPv4-only, IPv6-only, and dual-stack instances and the SDK
// will not auto-fall back between the two endpoints.
func NewAWS() (*AWS, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("error initializing AWS default config: %w", err)
	}

	return &AWS{cfg: cfg}, nil
}

// buildIMDSClient picks an IMDS endpoint that responds (IPv4 or IPv6) and
// returns a client bound to it. It races a probe against both endpoints and
// returns the first to succeed; if both fail it retries with backoff because
// the network stack may not be fully up on the very first attempt.
func (a *AWS) buildIMDSClient(ctx context.Context) (*imds.Client, error) {
	var resolved *imds.Client

	err := retry.Constant(
		30*time.Second,
		retry.WithUnits(2*time.Second),
		retry.WithErrorLogging(true),
	).RetryWithContext(ctx, func(ctx context.Context) error {
		client, probeErr := a.probeIMDS(ctx)
		if probeErr != nil {
			return retry.ExpectedError(probeErr)
		}

		resolved = client

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to reach IMDS on IPv4 or IPv6: %w", err)
	}

	return resolved, nil
}

// probeIMDS races a metadata request against the IPv4 and IPv6 IMDS endpoints
// and returns a client bound to whichever one responds first.
func (a *AWS) probeIMDS(ctx context.Context) (*imds.Client, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	type result struct {
		client *imds.Client
		mode   string
		err    error
	}

	candidates := []struct {
		mode imds.EndpointModeState
		name string
	}{
		{imds.EndpointModeStateIPv4, "IPv4"},
		{imds.EndpointModeStateIPv6, "IPv6"},
	}

	ch := make(chan result, len(candidates))

	for _, c := range candidates {
		client := imds.NewFromConfig(a.cfg, func(o *imds.Options) {
			o.EndpointMode = c.mode
		})

		go func(client *imds.Client, name string) {
			_, err := client.GetMetadata(probeCtx, &imds.GetMetadataInput{Path: "instance-id"})
			ch <- result{client: client, mode: name, err: err}
		}(client, c.name)
	}

	var lastErr error

	for range candidates {
		select {
		case r := <-ch:
			if r.err == nil {
				log.Printf("AWS IMDS reachable via %s endpoint", r.mode)

				return r.client, nil
			}

			lastErr = r.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, lastErr
}

// ParseMetadata converts AWS platform metadata into platform network config.
//
//nolint:gocyclo
func (a *AWS) ParseMetadata(metadata *MetadataConfig) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{
		TimeServers: []network.TimeServerSpecSpec{
			{
				NTPServers: []string{
					// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configure-ec2-ntp.html
					//
					// Include both IPv4 & IPv6 addresses for the NTP servers, Talos would lock to one of them (whichever works),
					// but it would be compatible with v4-only and v6-only deployments.
					"169.254.169.123",
					"fd00:ec2::123",
				},
				ConfigLayer: network.ConfigPlatform,
			},
		},
	}

	if metadata.Hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(metadata.Hostname); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	// Configure the primary interface based on which address families are present.
	//
	// AWS supports IPv4-only, IPv6-only, and dual-stack instances. We detect IPv6-only
	// by the absence of any IPv4 address (neither private nor public) and skip DHCPv4
	// in that case so the network comes up cleanly without a doomed DHCPv4 client.
	//
	// The IPv6 default gateway is delivered via Router Advertisements (the kernel adds
	// it when accept_ra is on) — DHCPv6 only hands out addresses — so we don't add any
	// static route for IPv6 here.
	if iface := metadata.PrimaryInterface; iface != nil {
		networkConfig.Links = append(networkConfig.Links, network.LinkSpecSpec{
			Name:        awsInterfaceName,
			Up:          true,
			ConfigLayer: network.ConfigPlatform,
		})

		hasIPv4 := len(iface.LocalIPv4s) > 0 || metadata.PublicIPv4 != ""
		hasIPv6 := len(iface.IPv6s) > 0

		// Default to IPv4 if the metadata is ambiguous (e.g. neither list populated).
		if !hasIPv4 && !hasIPv6 {
			hasIPv4 = true
		}

		if hasIPv4 {
			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP4,
				LinkName:  awsInterfaceName,
				RequireUp: true,
				DHCP4: network.DHCP4OperatorSpec{
					RouteMetric: network.DefaultRouteMetric,
				},
				ConfigLayer: network.ConfigPlatform,
			})
		}

		if hasIPv6 {
			networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP6,
				LinkName:  awsInterfaceName,
				RequireUp: true,
				DHCP6: network.DHCP6OperatorSpec{
					RouteMetric: network.DefaultRouteMetric,
				},
				ConfigLayer: network.ConfigPlatform,
			})

			dns, _ := netip.ParseAddr(awsIPv6DNSServer) //nolint:errcheck

			networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
				DNSServers:  []netip.Addr{dns},
				ConfigLayer: network.ConfigPlatform,
			})
		}
	}

	if metadata.PublicIPv4 != "" {
		if ip, err := netip.ParseAddr(metadata.PublicIPv4); err == nil {
			networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
		}
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     a.Name(),
		Hostname:     metadata.Hostname,
		Region:       metadata.Region,
		Zone:         metadata.Zone,
		InstanceType: metadata.InstanceType,
		InstanceID:   metadata.InstanceID,
		ProviderID:   fmt.Sprintf("aws:///%s/%s", metadata.Zone, metadata.InstanceID),
		Spot:         metadata.InstanceLifeCycle == "spot",
		InternalDNS:  metadata.InternalDNS,
		ExternalDNS:  metadata.ExternalDNS,
		Tags:         metadata.Tags,
	}

	return networkConfig, nil
}

// Name implements the runtime.Platform interface.
func (a *AWS) Name() string {
	return "aws"
}

// Configuration implements the runtime.Platform interface.
func (a *AWS) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	if err := netutils.Wait(ctx, r); err != nil {
		return nil, err
	}

	log.Printf("fetching machine config from AWS")

	client, err := a.buildIMDSClient(ctx)
	if err != nil {
		return nil, err
	}

	userdata, err := netutils.RetryFetch(ctx, func(ctx context.Context) (string, error) {
		return fetchConfiguration(ctx, client)
	})
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(userdata) == "" {
		return nil, platformerrors.ErrNoConfigSource
	}

	return []byte(userdata), nil
}

func fetchConfiguration(ctx context.Context, client *imds.Client) (string, error) {
	resp, err := client.GetUserData(ctx, &imds.GetUserDataInput{})
	if err != nil {
		if isNotFoundError(err) {
			return "", platformerrors.ErrNoConfigSource
		}

		return "", retry.ExpectedErrorf("failed to fetch EC2 userdata: %w", err)
	}

	defer resp.Content.Close() //nolint:errcheck

	userdata, err := io.ReadAll(resp.Content)

	return string(userdata), err
}

// Mode implements the runtime.Platform interface.
func (a *AWS) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (a *AWS) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (a *AWS) NetworkConfiguration(ctx context.Context, _ state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	// Emit a bootstrap config before fetching IMDS. In IPv6-only deployments
	// (subnets with IPv4 disabled) the IMDS endpoint at [fd00:ec2::254] is only
	// reachable from a non-link-local IPv6 address — which we get from DHCPv6 —
	// but the DHCPv6 operator only starts once the platform publishes a config
	// asking for it. Without this step the platform deadlocks: IMDS is
	// unreachable, NetworkConfiguration never returns, and DHCPv6 never runs.
	// The bootstrap brings up eth0 and starts both DHCPv4 and DHCPv6 so either
	// family can come up; the post-IMDS config below replaces it with the
	// family that actually applies.
	select {
	case ch <- bootstrapNetworkConfig():
	case <-ctx.Done():
		return ctx.Err()
	}

	log.Printf("fetching aws instance config")

	client, err := a.buildIMDSClient(ctx)
	if err != nil {
		return err
	}

	metadata, err := a.getMetadata(ctx, client)
	if err != nil {
		return err
	}

	networkConfig, err := a.ParseMetadata(metadata)
	if err != nil {
		return err
	}

	select {
	case ch <- networkConfig:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// bootstrapNetworkConfig returns the platform network config emitted before
// IMDS metadata is available. It brings up the primary interface and enables
// both DHCPv4 and DHCPv6 so the instance can reach IMDS regardless of which
// address family the VPC exposes.
func bootstrapNetworkConfig() *runtime.PlatformNetworkConfig {
	return &runtime.PlatformNetworkConfig{
		Links: []network.LinkSpecSpec{
			{
				Name:        awsInterfaceName,
				Up:          true,
				ConfigLayer: network.ConfigPlatform,
			},
		},
		Operators: []network.OperatorSpecSpec{
			{
				Operator:  network.OperatorDHCP4,
				LinkName:  awsInterfaceName,
				RequireUp: true,
				DHCP4: network.DHCP4OperatorSpec{
					RouteMetric: network.DefaultRouteMetric,
				},
				ConfigLayer: network.ConfigPlatform,
			},
			{
				Operator:  network.OperatorDHCP6,
				LinkName:  awsInterfaceName,
				RequireUp: true,
				DHCP6: network.DHCP6OperatorSpec{
					RouteMetric: network.DefaultRouteMetric,
				},
				ConfigLayer: network.ConfigPlatform,
			},
		},
	}
}
