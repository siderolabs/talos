// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package aws

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/talos-systems/go-procfs/procfs"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
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

// Name implements the runtime.Platform interface.
func (a *AWS) Name() string {
	return "aws"
}

// Configuration implements the runtime.Platform interface.
func (a *AWS) Configuration(ctx context.Context) ([]byte, error) {
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
//
//nolint:gocyclo
func (a *AWS) NetworkConfiguration(ctx context.Context, ch chan<- *runtime.PlatformNetworkConfig) error {
	getMetadataKey := func(key string) (string, error) {
		v, err := a.metadataClient.GetMetadataWithContext(ctx, key)
		if err != nil {
			if awsErr, ok := err.(awserr.RequestFailure); ok {
				if awsErr.StatusCode() == http.StatusNotFound {
					return "", nil
				}
			}

			return "", fmt.Errorf("failed to fetch %q from IMDS: %w", key, err)
		}

		return v, nil
	}

	networkConfig := &runtime.PlatformNetworkConfig{}

	hostname, err := getMetadataKey("hostname")
	if err != nil {
		return err
	}

	if hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err = hostnameSpec.ParseFQDN(hostname); err != nil {
			return err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	externalIP, err := getMetadataKey("public-ipv4")
	if err != nil {
		return err
	}

	if externalIP != "" {
		ip, err := netaddr.ParseIP(externalIP)
		if err != nil {
			return err
		}

		networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
	}

	select {
	case ch <- networkConfig:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
