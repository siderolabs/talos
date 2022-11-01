// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package aws contains the AWS implementation of the [platform.Platform].
package aws

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	runtimeres "github.com/talos-systems/talos/pkg/machinery/resources/runtime"
)

// AWS is the concrete type that implements the runtime.Platform interface.
type AWS struct {
	metadataClient *ec2metadata.EC2Metadata
}

// NewAWS initializes AWS platform building the IMDS client.
func NewAWS() (*AWS, error) {
	a := &AWS{}

	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		return nil, err
	}

	sess.Config.Credentials = ec2rolecreds.NewCredentials(sess)

	a.metadataClient = ec2metadata.New(sess)

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

	if metadata.PublicIPv4 != "" {
		ip, err := netip.ParseAddr(metadata.PublicIPv4)
		if err != nil {
			return nil, err
		}

		networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     a.Name(),
		Hostname:     metadata.Hostname,
		Region:       metadata.Region,
		Zone:         metadata.Zone,
		InstanceType: metadata.InstanceType,
		InstanceID:   metadata.InstanceID,
		ProviderID:   fmt.Sprintf("aws://%s/%s", metadata.Zone, metadata.InstanceID),
	}

	return networkConfig, nil
}

// Name implements the runtime.Platform interface.
func (a *AWS) Name() string {
	return "aws"
}

// Configuration implements the runtime.Platform interface.
func (a *AWS) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	log.Printf("fetching machine config from AWS")

	userdata, err := a.metadataClient.GetUserDataWithContext(ctx)
	if err != nil {
		if awsErr, ok := err.(awserr.RequestFailure); ok {
			if awsErr.StatusCode() == http.StatusNotFound {
				return nil, errors.ErrNoConfigSource
			}
		}

		return nil, fmt.Errorf("failed to fetch EC2 userdata: %w", err)
	}

	if strings.TrimSpace(userdata) == "" {
		return nil, errors.ErrNoConfigSource
	}

	return []byte(userdata), nil
}

// Mode implements the runtime.Platform interface.
func (a *AWS) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (a *AWS) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
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
