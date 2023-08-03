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

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// AWS is the concrete type that implements the runtime.Platform interface.
type AWS struct {
	metadataClient *imds.Client
}

// NewAWS initializes AWS platform building the IMDS client.
func NewAWS() (*AWS, error) {
	a := &AWS{}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("error initializing AWS default config: %w", err)
	}

	a.metadataClient = imds.NewFromConfig(cfg)

	return a, nil
}

// ParseMetadata converts AWS platform metadata into platform network config.
func (a *AWS) ParseMetadata(metadata *MetadataConfig) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	if metadata.Hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(metadata.Hostname); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	publicIPs := []string{}

	if metadata.PublicIPv4 != "" {
		publicIPs = append(publicIPs, metadata.PublicIPv4)
	}

	if metadata.PublicIPv6 != "" {
		publicIPs = append(publicIPs, metadata.PublicIPv6)
	}

	for _, ipStr := range publicIPs {
		if ip, err := netip.ParseAddr(ipStr); err == nil {
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

	userdata, err := netutils.RetryFetch(ctx, a.fetchConfiguration)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(userdata) == "" {
		return nil, errors.ErrNoConfigSource
	}

	return []byte(userdata), nil
}

func (a *AWS) fetchConfiguration(ctx context.Context) (string, error) {
	resp, err := a.metadataClient.GetUserData(ctx, &imds.GetUserDataInput{})
	if err != nil {
		if isNotFoundError(err) {
			return "", errors.ErrNoConfigSource
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
func (a *AWS) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (a *AWS) NetworkConfiguration(ctx context.Context, _ state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching aws instance config")

	metadata, err := a.getMetadata(ctx)
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
